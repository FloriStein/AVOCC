# Sprint 11 — Vehicle Connectivity & Feedback

Stand: 2026-06-11
Status: ✅ Abgeschlossen — Go Build + 26 Unit Tests grün + 41 Frontend Tests grün

---

## Ziel

Steuerbefehle kommen tatsächlich beim Fahrzeug an — und der Operator sieht, was am Fahrzeug ankommt, nicht was er eingibt.

Architekturentscheidung: [ADR-021](../adr/021-vehicle-connectivity-feedback.md)

---

## Root Cause: Critical Gap

Vor diesem Sprint wurden Steuerbefehle **nie** zum Fahrzeug weitergeleitet. Die `readLoop` in `internal/vehicleconnection/handler.go` hat alle eingehenden Nachrichten still verworfen. Die VehicleForwarder-Abstraktion fehlte komplett.

---

## Architektur-Entscheidungen (ADR-021)

| Entscheidung | Option A | Option B gewählt | Begründung |
|---|---|---|---|
| Fahrzeug-Identität | Separater Identity Service | JWT mit `sub = vehicleID` vom Auth Service | Kein Mehrwert bei 1 Fahrzeug, bestehender Auth Service ausreichend |
| Protokoll Operator→Fahrzeug | Neues Binary-Format | Protobuf end-to-end | ControlCommand bereits fertig; keine Serialisierungsschicht |
| Feedback-Mechanismus | Polling REST | WebSocket ACK + MQTT Telemetrie | Zwei-Stufen: Transport-Bestätigung (WS) + Aktuator-Status (MQTT) |

---

## Implementierung (12 Tasks)

| ID | Task | Status |
|----|------|--------|
| VEH-01 | ADR-021 schreiben | ✅ |
| VEH-02 | `proto/vehicle.proto` — `VehicleCommandAck` | ✅ |
| VEH-03 | `proto/telemetry.proto` — Actuation Fields 7–11 | ✅ |
| VEH-04 | `internal/vehicleconnection/registry.go` — Registry + ForwardCommand | ✅ |
| VEH-05 | `internal/vehicleconnection/ackstore.go` — AckStore | ✅ |
| VEH-06 | `internal/controlserver/command/engine.go` — VehicleForwarder Interface | ✅ |
| VEH-07 | `cmd/control-server/main.go` — Verdrahtung + `GET /vehicle/ack/latest/` | ✅ |
| VEH-08 | `cmd/vehicle-mock/main.go` — Go Service (JWT, WS, Protobuf, MQTT, Lerp) | ✅ |
| VEH-09 | Compose + Dockerfile + nginx `/vehicle/` Proxy | ✅ |
| VEH-10 | `useVehicleAck.ts` + `useTelemetry.ts` erweitert | ✅ |
| VEH-11 | `InputIndicatorPanel.tsx` — Lenkrad-SVG + ActuationBar + AckBadge | ✅ |
| VEH-12 | Tests: `vehicleconnection_test.go` (7 Go) + `InputIndicatorPanel.test.tsx` (7 TS) | ✅ |

---

## Neue Dateien

### Backend (Go)

| Datei | Inhalt |
|-------|--------|
| `proto/vehicle.proto` | `VehicleCommandAck` — Protobuf |
| `internal/vehicleconnection/registry.go` | Vehicle-WS-Connections, thread-safe ForwardCommand |
| `internal/vehicleconnection/ackstore.go` | Latest-ACK je vehicleID |
| `internal/vehicleconnection/handler.go` | Rewritten: register/unregister, readLoop dekodiert VehicleCommandAck |
| `cmd/vehicle-mock/main.go` | Docker-Service: JWT self-gen, WS-Connect, Protobuf decode, ACK send, MQTT lerp |
| `infrastructure/docker/vehicle-mock.Dockerfile` | Multi-stage Go build |

### Frontend (TypeScript/React)

| Datei | Inhalt |
|-------|--------|
| `frontend/src/hooks/useVehicleAck.ts` | Pollt `/vehicle/ack/latest/{vehicleId}` alle 500ms |
| `frontend/src/components/InputIndicatorPanel.tsx` | Lenkrad-SVG (steerActual × 120°), ActuationBar, AckBadge |

### Modifizierte Dateien

| Datei | Änderung |
|-------|----------|
| `proto/telemetry.proto` | Fields 7–11: steer/throttle/brake commanded + actual |
| `internal/vehicleconnection/handler.go` | Registry + AckStore injiziert |
| `internal/controlserver/command/engine.go` | VehicleForwarder Interface + WithVehicleForwarder() |
| `cmd/control-server/main.go` | Registry/AckStore verdrahtet, `/vehicle/ack/latest/` Endpoint |
| `cmd/telemetry-service/main.go` | Actuation Fields in JSON-Response |
| `frontend/src/hooks/useTelemetry.ts` | TelemetryData Interface erweitert |
| `frontend/src/App.tsx` | useVehicleAck + InputIndicatorPanel integriert |
| `infrastructure/compose/docker-compose.yml` | vehicle-mock Service |
| `infrastructure/compose/docker-compose.prod.yml` | vehicle-mock mit Image-Referenz |
| `infrastructure/docker/nginx.conf` | `/vehicle/` Proxy-Location |

---

## Zwei-Stufen-Feedback-Architektur

```
Operator → [ControlCommand Protobuf] → Control Server
                                           │
                                    VehicleForwarder.ForwardCommand()
                                           │
                                    vehicleconnection.Registry
                                           │ WebSocket Binary
                                           ▼
                                     vehicle-mock
                                    (oder Raspberry Pi)
                                           │
                              ┌────────────┴──────────────┐
                              │                           │
                    VehicleCommandAck               TelemetryEvent
                    (WS zurück)                  (MQTT, 2s Interval)
                              │                           │
                    AckStore.Store()            steer/throttle actual
                              │                 (15% Aktuator-Lag, Lerp)
                    GET /vehicle/ack/latest/               │
                              │                   useTelemetry()
                    useVehicleAck()                        │
                              │                           │
                              └────────────┬──────────────┘
                                           │
                                  InputIndicatorPanel
                                  (Lenkrad + Bars + Badge)
```

---

## Test-Ergebnisse

```
Go Build:         ✅ EXIT 0
Go Unit Tests:    ✅ 26/26 (19 Safety + 7 vehicleconnection)
Frontend Tests:   ✅ 41/41 (34 bestehend + 7 InputIndicatorPanel)
```

---

## DoD

- ✅ ControlCommand wird per WebSocket an das verbundene Fahrzeug weitergeleitet
- ✅ vehicle-mock: JWT self-generated, WS connect, Protobuf decode, ACK senden
- ✅ VehicleCommandAck gespeichert und per REST abrufbar
- ✅ TelemetryEvent enthält Aktuator-Ist-Werte (mit simuliertem 15% Lag)
- ✅ InputIndicatorPanel zeigt Lenkrad (Ist), ActuationBars (Soll/Ist) und AckBadge
- ✅ Alle Tests grün (Go Build + 26 Unit Tests + 41 Frontend Tests)
- ✅ ADR-021 dokumentiert
