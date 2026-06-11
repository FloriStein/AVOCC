# ADR-021: Vehicle Connectivity & Feedback Architecture

Status: Accepted

## Kontext

Nach Sprint 10 sendet der Control Server Steuerbefehle korrekt als Protobuf über
WebSocket vom Operator entgegen und gibt ein Server-seitiges `ControlAck` zurück.
Der `vehicleconnection`-Handler existiert und akzeptiert Fahrzeug-WebSocket-Verbindungen,
leitet jedoch **keine Commands an das Fahrzeug weiter** — der Forwarding-Pfad fehlt
vollständig. Es gibt außerdem keinen Feedback-Kanal, der dem Operator zeigt, ob und
was beim Fahrzeug tatsächlich ankam.

Für einen echten Teleoperations-Closed-Loop wird benötigt:

1. Commands müssen das Fahrzeug erreichen (Protobuf, Vehicle WebSocket)
2. Das Fahrzeug muss den Empfang auf Transportebene bestätigen (ACK)
3. Das Fahrzeug muss seinen tatsächlichen Aktierungszustand zurückmelden (MQTT)
4. Der Operator muss in der UI sehen: *gesendet → empfangen → ausgeführt*

## Entscheidungen

### 1. Was ist das Fahrzeug?

**Entscheidung: Externer Client auf `/vehicle/ws` — Ziel Raspberry Pi, Entwicklung Docker-Mock**

Das Fahrzeug verbindet sich aktiv mit dem Control Server. Die Schnittstelle ist von
Anfang an so ausgelegt, dass später ein reales Gerät (Raspberry Pi, Roboter) dieselbe
Implementierung verwenden kann. Ein Docker-Mock-Service implementiert die identische
Schnittstelle und ermöglicht Entwicklung und Tests ohne Hardware.

**Abgelehnt:** Browser-Tab (Option C) — keine echten Netzwerkbedingungen, keine
Reconnect-Probleme, keine realistischen Einschränkungen eines Embedded-Systems.

### 2. Welches Protokoll erhält das Fahrzeug?

**Entscheidung: Protobuf end-to-end — ControlCommand unverändert**

Der `ControlCommand` (inkl. vollständigem `CorrelationHeader`) wird vom Command Engine
unverändert als binäre Protobuf-Bytes über die Vehicle-WebSocket-Verbindung weitergeleitet.

```
Operator → [ControlCommand.proto] → Control Server → [ControlCommand.proto] → Fahrzeug
```

**Begründung:** Ein einheitliches Serialisierungsformat auf allen Kanälen eliminiert
doppelte Schemadefinitionen, doppelte Validierung und spätere Migrationsprobleme.
Protobuf ist auf Go, Python, Rust, C++ und sogar embedded (nanopb) verfügbar.

**Abgelehnt:** JSON über WebSocket — zweites Format, zweite Validierung, höhere
Wartungskosten ohne Vorteil.

### 3. Wie berichtet das Fahrzeug zurück?

**Entscheidung: Zweistufiges Feedback**

**Stufe 1 — WebSocket ACK (Transportebene):**
Das Fahrzeug sendet nach dem Empfang eines Commands ein `VehicleCommandAck` zurück
(neues Protobuf in `vehicle.proto`). Dieses enthält die `event_id` des empfangenen
Commands als Echo. Der Control Server speichert den letzten ACK pro Fahrzeug und
stellt ihn über `GET /vehicle/ack/latest/{vehicleId}` bereit.

→ Operator sieht: *„Command #4711 vom Fahrzeug empfangen (vor 18ms)"*

**Stufe 2 — MQTT Telemetrie (Fachlichkeit):**
Das Fahrzeug veröffentlicht über den bestehenden MQTT-Telemetriekanal erweiterte
`TelemetryEvent`-Nachrichten mit Aktierungswerten:
- `steer_commanded` / `throttle_commanded` / `brake_commanded` — was das Fahrzeug
  als Sollwert gesetzt hat
- `steer_actual` / `throttle_actual` — tatsächliche Aktorposition (soweit messbar)

→ Operator sieht: *„Fahrzeug: Lenkung 35% rechts (Soll: 35%, Ist: 33%)"*

**Abgelehnt:** Server-Echo (Option B) — zeigt nur was gesendet wurde, nicht was ankam.
Gefährliche Fehlwahrnehmung bei Packet Loss oder Fahrzeug-Disconnect.

## Datenfluss (vollständig)

```
Operator
  │ ControlCommand (Protobuf, WebSocket)
  ▼
Control Server (Command Engine)
  ├─→ ControlAck → Operator            (bestehend: Server-ACK, <100ms)
  │
  │ ControlCommand (Protobuf, Vehicle WebSocket)  ← NEU
  ▼
Fahrzeug / vehicle-mock
  ├─→ VehicleCommandAck → Control Server → GET /vehicle/ack/latest/{id}  ← NEU
  │
  │ TelemetryEvent + steer/throttle_commanded/actual (MQTT)  ← NEU
  ▼
MQTT Broker (Mosquitto) → Telemetry Service → GET /telemetry/latest/{id}
  ▼
Frontend (InputIndicatorPanel)
  - ACK-Status: gesendet / empfangen / timeout
  - Commanded vs. Actual Vergleich
```

## Neue Komponenten

| Komponente | Beschreibung |
|------------|-------------|
| `vehicle.proto` | `VehicleCommandAck` — Transport-ACK vom Fahrzeug |
| `telemetry.proto` Felder 7–11 | Aktierungsfelder: steer/throttle/brake commanded + actual |
| `vehicleconnection.Registry` | Speichert aktive Fahrzeug-WebSocket-Verbindungen (vehicleID → conn) |
| `vehicleconnection.AckStore` | Speichert letzten VehicleCommandAck pro vehicleID |
| `command.VehicleForwarder` | Interface: `ForwardCommand(vehicleID string, data []byte) error` |
| `cmd/vehicle-mock/` | Docker-Service: WS connect, Protobuf decode, ACK, MQTT publish |
| `InputIndicatorPanel.tsx` | Frontend: ACK-Status + tatsächliche Fahrzeugwerte |

## Safety-Invarianten (unverändert)

- Vehicle-Disconnect → SAFE_MODE (bestehend, ADR-009/010)
- Kein VehicleCommandAck ist kein CRITICAL-Trigger — nur DEGRADED/Warnung im UI
- MQTT-Telemetrieausfall ist DEGRADED, nicht SAFE_MODE (ADR-009)
- Der Forwarding-Pfad darf SAFE_MODE nie verzögern: Fire-and-Forget, kein Blocking

## Konsequenzen

**Positiv:**
- Echter Closed-Loop: Operator sieht was das Fahrzeug empfängt und ausführt
- Docker-Mock ermöglicht vollständige lokale Entwicklung ohne Hardware
- Protokoll-Konsistenz: Protobuf auf allen Kanälen
- Raspberry Pi kann dieselbe Schnittstelle implementieren — keine Migration

**Negativ/Trade-offs:**
- Fahrzeug muss Protobuf dekodieren können (akzeptabel: Libraries für alle Targets)
- MQTT-Aktierungsdaten sind asynchron (~1s Poll) — nicht für Safety, nur für UX
- VehicleCommandAck zeigt Empfang, nicht erfolgreiche Ausführung (explizite Einschränkung)

## Referenzen

- ADR-008: Protobuf als Application Bus
- ADR-009: Failure Model (CRITICAL/DEGRADED)
- ADR-010: Control Loop & Safety Override
- ADR-014: Video Streaming (coturn, separate vom Control-Kanal)
- ADR-019: Deployment-Strategie (Docker-Mock in Compose)
