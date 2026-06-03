# ADR-009: Failure Model & System Behavior under Faults

Status: Accepted (erweitert durch ADR-014)

## Kontext

Ein Teleoperation-System muss bei Netzwerk- und Service-Fehlern deterministisch reagieren. Mit der Einführung von WebRTC (ADR-014) und dem 4-Layer State Machine Modell (ADR-011) wird das Failure Model auf alle vier Kanäle präzisiert: Control, Safety, Video (Media) und Auth.

## Failure Classification (vollständig)

### CRITICAL — Sofortiger Auto-Stop → SYSTEM SAFE_MODE

| Fehlerfall | Trigger | Verhalten |
|---|---|---|
| WebSocket Control Disconnect | CONTROL_LOST | Channel Close → SAFE_MODE |
| Safety Event Bus Failure | Safety Bus unreachable | Auto-Stop → SAFE_MODE |
| Dead-man Switch Timeout | Kein Heartbeat vom Operator | Auto-Stop → SAFE_MODE |
| Command ACK Timeout | Control Loop ohne Bestätigung | Auto-Stop → SAFE_MODE |
| Auth Invalidation | JWT revoked, laufende Session | Auto-Stop → SAFE_MODE |
| No Active Operator | OPERATOR_STATE = NO_OPERATOR | Auto-Stop → SAFE_MODE |

### DEGRADED — Warnung, Control bleibt möglich

| Fehlerfall | Trigger | Verhalten |
|---|---|---|
| Video Stream Lost | MEDIA_FAILED | SYSTEM → DEGRADED, Warnung im UI |
| Video Qualitätsverlust | MEDIA_DEGRADED | SYSTEM → DEGRADED, Warnung |
| Secondary Camera Failure | Partial MEDIA_FAILED | SYSTEM → DEGRADED |
| Partial Telemetry Loss | MQTT teilweise verloren | SYSTEM → DEGRADED |

### OBSERVATION — Kein Stop, laufende Session bleibt aktiv

| Fehlerfall | Verhalten |
|---|---|
| Auth Service nicht erreichbar (laufende Session) | Neue Sessions blockiert, JWT lokale Validierung weiterhin möglich |

---

## Valider SAFE_MODE Zustand

Ein valider SAFE_MODE ist erreicht, wenn:

- Fahrzeugbewegung sofort auf 0 gesetzt
- Control WebSocket geschlossen (Channel Close — ADR-010)
- CONTROL STATE = CONTROL_BLOCKED
- Keine Commands werden akzeptiert oder weitergeleitet
- SYSTEM STATE = SAFE_MODE
- MEDIA STATE läuft weiter (optional — Video bleibt für Monitoring aktiv)
- Operator UI = read-only

## Recovery Sequence

Nach einem CRITICAL Failure:

```
1. Auto-Stop → SAFE_MODE
2. Channel Close (WebSocket geschlossen)
3. Reconnect aller kritischen Services (automatisch, Exponential Backoff)
   - Control WebSocket
   - Safety Event Bus
4. Validierung des aktuellen Systemzustands
5. Operator erhält Resume-Aufforderung (HANDOVER_PENDING oder direktes Ack)
6. Operator bestätigt explizit (Operator ACK)
7. System → RECOVERING → AUTHENTICATED → CONNECTED
8. CONTROL STATE = CONTROL_ACTIVE
```

**Kein automatisches Resume nach Reconnect — immer Operator-Ack.**

---

## Formale System-Invarianten

```
INVARIANT 1:
  Media Layer SHALL NOT influence SAFE_MODE transitions
  except via DEGRADED annotation evaluated by the Control Hub.

INVARIANT 2:
  SAFE_MODE transitions are exclusively triggered by:
  - Control Channel failures
  - Safety Bus failures
  - Operator-level failures (Dead-man, ACK Timeout, No Operator)
  Never by: Video, Telemetry, or Media Layer events.

INVARIANT 3:
  Control Hub is the Single Source of Truth for Session State.
  Video Hub (SFU) derives session context from Control Hub.
  Conflicting states resolve in favor of Control Hub.
```

## Safety Rules

**Video darf niemals SAFE_MODE triggern.** MEDIA_FAILED → DEGRADED, nicht SAFE_MODE.

**Control ist der einzige sicherheitskritische Kanal.** Control + Safety Bus = System Safety.

---

## Konsequenzen

### Positiv:
- Vollständige Failure-Klassifizierung über alle 4 Kanäle
- Video-Failure explizit als nicht-kritisch definiert
- Safety Test Suite kann jeden Trigger isoliert testen
- Klare Recovery-Sequenz implementierbar

### Negativ:
- Command ACK Timeout als CRITICAL erfordert präzises Timeout-Management im Control Server
- SAFE_MODE lässt Media weiter laufen — erfordert bewusste Implementierungsentscheidung
