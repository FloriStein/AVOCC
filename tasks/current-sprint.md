# Sprint 2 — Safety & Failure Model

Ziel: 4-Layer State Machine vollständig. SAFE MODE deterministisch. Safety Test Suite grün.

Datum: 2026-06-03
Vorgänger: Sprint 1 ✅ (Foundation Layer — alle Services laufen, WS+JWT+Safety Bus verifiziert)

---

## Ausgangslage (aus Sprint 1)

Was bereits existiert und Sprint 2 als Basis dient:

| Datei | Stand |
|-------|-------|
| `internal/controlserver/statemachine/state.go` | 4 State-Typen + Machine-Struct + Basis-Transitionen (TransitionSystem/Media/Operator). Keine Checkpoint-Logik, kein Session-Manager. |
| `internal/controlserver/transport/websocket.go` | WS + JWT-Auth + Heartbeat + SAFE_MODE bei WS-Disconnect. Kein ACK-Timeout, kein Dead-man-Switch. TODO: BE-04 (Command Engine). |
| `internal/safetyservice/bus.go` | Vollständig: alle CRITICAL EventTypes, Pub/Sub, EmergencyStop, GetSafetyState, Subscribe, Reset. |
| `internal/controlserver/session/` | Leer — Ziel von BE-09 |
| `internal/controlserver/safety/` | Leer — Ziel von BE-10 |
| `pkg/ulid/` | Leer — ULID-Wrapper für BE-09 |

---

## Tasks

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| TEST-01 | Go Test Infrastructure — testify + Mock Pattern | S | 🔲 Todo | ADR-006; testify in go.mod; Mock-Interfaces für Safety Bus + SFU Event Stream; Basis für TEST-02 |
| BE-06 | Vehicle Connection Service — Session Management | M | 🔲 Todo | ADR-015; Disconnect-Erkennung; SAFE_MODE-Trigger bei Vehicle-Disconnect; kein Session-State-Ownership (GSA = Control Server) |
| BE-09 | Control Server — Session Manager (GSA) + State Machine Erweiterung | M | 🔲 Todo | ADR-011/015/016; `internal/controlserver/session/` + `pkg/ulid/`; ULID Session-ID bei CONNECTING→CONNECTED; Recovery Checkpoint bei SAFE_MODE; SFU Event-Push (SESSION_*); State Machine: volle Transition-Validierung |
| BE-10 | Control Server — Failure Detection & Recovery | M | 🔲 Todo | ADR-009/010/011/015; Dead-man-Switch Timeout; Command ACK Timeout → SAFE_MODE; Exponential Backoff Reconnect; RECOVERING → SAFE_MODE-Fallback; Checkpoint laden bei Recovery |
| BE-12 | Operator Handover Logic — Active/Observer/Standby | M | 🔲 Todo | ADR-011/015; OPERATOR STATE vervollständigen; HANDOVER_PENDING Transition; exklusiver ACTIVE_OPERATOR; Handover-Token via Auth Service; SFU Event: OPERATOR_HANDOVER |
| TEST-02 | Safety Test Suite — `safety_test.go` | M | 🔲 Todo | ADR-006/009/011; alle 7 CRITICAL Trigger; MEDIA_FAILED→DEGRADED (nicht SAFE_MODE); Recovery Checkpoint validieren |

---

## Abhängigkeitspfad

```
TEST-01 (sofort startbar) ──────────────────────────────────────┐
BE-06   (BE-01✅ BE-02✅, sofort startbar) ──────────────────── │
BE-09   (BE-02✅ BE-03✅, sofort startbar) → BE-10 ──────────── │→ TEST-02 ✓
                                           → BE-12 ─────────────┘
```

Parallelstart möglich: **TEST-01 + BE-06 + BE-09** haben keine gegenseitigen Abhängigkeiten.

---

## Implementierungsdetails je Task

### TEST-01 — Go Test Infrastructure
- `testify/assert` + `testify/mock` in `go.mod` aufnehmen
- Mock-Interface für Safety Bus: `MockSafetyBus` (implementiert dieselbe Interface-Signatur wie `safetyservice.Bus`)
- Mock-Interface für SFU Event Stream: `MockSFUPublisher`
- Basis-Testdatei `internal/controlserver/session/session_test.go` anlegen

### BE-06 — Vehicle Connection Service
- `internal/vehicleconnection/` Paket anlegen
- Disconnect-Detection: Heartbeat-Ausfall vom Vehicle → `EventWSDisconnect` auf Safety Bus publishen
- Kein Session-State-Ownership: nur Event-Publisher, kein State-Keeper
- HTTP/WS-Handler für Vehicle-Verbindung (`/vehicle/ws`)

### BE-09 — Session Manager (GSA) + State Machine Erweiterung
**`pkg/ulid/ulid.go`:**
- Thin Wrapper um `oklog/ulid/v2`
- `Generate() string` — threadsafe ULID

**`internal/controlserver/session/manager.go`:**
- `Session`-Struct: `SessionID (ULID)`, `VehicleID`, `OperatorID`, `OperatorRole`, `CreatedAt`
- `RecoveryCheckpoint`-Struct: SessionID, VehicleID, OperatorID, LastSystemState, LastControlState, SafetyReason, CheckpointTS
- `Manager.CreateSession(vehicleID, operatorID string) Session` — generiert ULID, Zeitpunkt CONNECTING→CONNECTED
- `Manager.SaveCheckpoint(sm *statemachine.Machine, reason string)` — bei SAFE_MODE-Eintritt
- `Manager.LoadCheckpoint() (*RecoveryCheckpoint, bool)`
- `Manager.PublishSFUEvent(eventType string, session Session)` — async push an SFU

**State Machine Erweiterung in `statemachine/state.go`:**
- Transition-Validierung: ungültige Übergänge abweisen + loggen (z.B. IDLE→CONNECTED ohne AUTHENTICATED)
- `TransitionToConnected(sessionID string)` — atomare Transition AUTHENTICATED→CONNECTED + Session-ID setzen

### BE-10 — Failure Detection & Recovery
**`internal/controlserver/safety/detector.go`:**
- `DeadmanWatchdog`: Timer-basiert; Reset() bei jedem Input; Ablauf → `EventDeadmanTimeout` auf Safety Bus
- `ACKTimeoutWatcher`: pro Command ein Timer; kein ACK in Fenster → `EventACKTimeout` auf Safety Bus
- `RecoveryManager`: Exponential Backoff (1s, 2s, 4s, 8s, max 30s) für Reconnect-Versuche
- RECOVERING→SAFE_MODE-Fallback: wenn Validierung fehlschlägt

**Integration in `transport/websocket.go`:**
- `DeadmanWatchdog` starten bei CONTROL_ACTIVE
- `DeadmanWatchdog.Reset()` bei jedem eingehenden Command
- ACK-Timeout: nach Command-Empfang Timer starten, bei ACK canceln

### BE-12 — Operator Handover
**`internal/controlserver/session/handover.go`:**
- `HandoverManager.RequestHandover(fromOperatorID, toOperatorID string)`
- OPERATOR STATE: ACTIVE_OPERATOR → HANDOVER_PENDING → ACTIVE_OPERATOR (neuer) oder Rollback
- Exklusivitätsprüfung: max. 1 ACTIVE_OPERATOR
- Handover-Token: `POST /auth/handover/token` via Auth Service aufrufen
- Bei Abschluss: `Manager.PublishSFUEvent("OPERATOR_HANDOVER", session)`

### TEST-02 — Safety Test Suite
Testfälle in `tests/unit/safety_test.go`:

| Szenario | Erwartung |
|----------|-----------|
| Dead-man Timeout | SAFE_MODE + CONTROL_BLOCKED |
| Emergency Stop | SAFE_MODE + CONTROL_BLOCKED |
| WS Disconnect | SAFE_MODE + CONTROL_BLOCKED |
| Command ACK Timeout | SAFE_MODE + CONTROL_BLOCKED |
| No Active Operator | SAFE_MODE + CONTROL_BLOCKED |
| Auth Invalidation | SAFE_MODE + CONTROL_BLOCKED |
| Safety Bus Down | SAFE_MODE + CONTROL_BLOCKED |
| MEDIA_FAILED | DEGRADED — **niemals SAFE_MODE** (Invariante 1) |
| Recovery Checkpoint | SessionID + SafetyReason korrekt gespeichert |
| Recovery → SAFE_MODE-Fallback | Wenn Validierung fehlschlägt → SAFE_MODE, nicht CONNECTED |

---

## Sprint-Ziel / Definition of Done

- [ ] SYSTEM STATE Machine mit allen 7 Zuständen korrekt implementiert + Transition-Validierung
- [ ] CONTROL, MEDIA, OPERATOR STATE implementiert und entkoppelt (keine Cross-Contamination)
- [ ] Session Manager (GSA): Session-ID (ULID) bei CONNECTING→CONNECTED generiert
- [ ] Recovery Checkpoint bei SAFE_MODE gespeichert, bei Recovery geladen
- [ ] SFU Event-Push funktioniert (SESSION_CREATED, SESSION_SAFE_MODE, OPERATOR_HANDOVER)
- [ ] Dead-man Switch Timeout → SAFE_MODE (automatisch, kein Operator-Input nötig)
- [ ] Command ACK Timeout → SAFE_MODE
- [ ] Exponential Backoff Reconnect implementiert
- [ ] MEDIA_FAILED → DEGRADED, niemals SAFE_MODE (Invariante 1 verifiziert)
- [ ] Safety Test Suite grün: alle 7 CRITICAL Trigger + Recovery Checkpoint + MEDIA Invariante
- [ ] Operator-Ack Flow + Handover (HANDOVER_PENDING) funktionieren
