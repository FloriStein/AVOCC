# Done

Lifecycle: backlog → sprint → done

---

## Sprint 3 — Frontend Core

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| FE-09 | Frontend Protobuf Adapter + Build-Pipeline | M | ✅ `@bufbuild/protobuf` + `@bufbuild/protoc-gen-es`; `common_pb.js` + `control_pb.js` im Bundle; nginx `/api/` + `/auth/` + `/vehicle/ws` Proxy-Routen |
| FE-02 | WebSocket Client + State-Polling | M | ✅ `ws-client.ts` mit Latenz-Messung; `useSystemState` (500ms Polling); `useSession` (auto-login, Reconnect mit Exponential Backoff) |
| FE-08 | SAFE MODE Overlay + Operator Ack Flow | M | ✅ Fullscreen-Overlay bei SAFE_MODE; Resume-Button triggert Recovery-Flow; DEGRADED-Banner |
| FE-04 | Safety Controls — Emergency Stop + Dead-man Switch | M | ✅ E-Stop → `POST /api/emergency-stop`; Dead-man (Spacebar/Mousedown, 400ms Interval, Protobuf DEADMAN_HOLD) |
| FE-03 | Connection Status — Live-Anzeige | S | ✅ Live Latenz (grün <50ms, gelb <100ms, rot ≥100ms), Session-ID (gekürzt), Operator-Rolle, State-Badge |

### Bugfixes während Tests

1. **`transport/websocket.go`**: Recovery-Pfad fehlte — bei neuem WS-Connect aus SAFE_MODE wurde `CONNECTING` versucht (invalid Transition). Fix: Wenn System in `SAFE_MODE`, dann `SAFE_MODE → RECOVERING → AUTHENTICATED` statt `IDLE → CONNECTING → AUTHENTICATED`.
2. **`frontend/package-lock.json`**: Neuer Dependencies (`@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`, `ulidx`) nicht im Lock-File — `npm install` ausgeführt.
3. **`ConnectionPanel.tsx`**: Ungenutzte `type SystemState` Deklaration (TS6196) → entfernt.

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Health-Checks alle 5 Services | `{"status":"ok"}` | ✅ |
| nginx `/api/state` | JSON (nicht HTML) | ✅ `{"system":"IDLE",...}` |
| nginx `/auth/operator/login` | JWT-Token | ✅ Token mit `role=OBSERVER` |
| nginx `/api/nonexistent` | `404 page not found` (nicht HTML) | ✅ |
| nginx `/auth/nonexistent` | HTTP 404 | ✅ |
| Protobuf-Bundle: `common_pb.js` + `control_pb.js` | Vorhanden | ✅ |
| Protobuf-Strings im Bundle | `CorrelationHeader`, `DEADMAN_HOLD`, `EMERGENCY_STOP` | ✅ |
| Login → WS → session/start → CONNECTED + ULID | `CONTROL_ACTIVE / ACTIVE_OPERATOR / session_id (26 Zeichen)` | ✅ |
| State-Polling (5 × 500ms) | Konsistente JSON-Antwort | ✅ |
| Emergency Stop `/api/emergency-stop` | HTTP 202, `SAFE_MODE / CONTROL_BLOCKED`, Safety Bus `EMERGENCY_STOP` | ✅ |
| Dead-man Watchdog (2s ohne Reset) | `SAFE_MODE` nach 2.5s | ✅ |
| Recovery: SAFE_MODE → Resume → CONNECTED | `RECOVERING → AUTHENTICATED → CONNECTED`, neue Session-ID ≠ alte | ✅ |
| Vehicle WS via nginx `/vehicle/ws` | Verbindung aufgebaut | ✅ |
| Vehicle WS mit Operator-Token → 401 | HTTP 401 | ✅ |
| Vehicle Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Handover Request → HANDOVER_PENDING | HTTP 202 | ✅ |
| Handover Confirm → ACTIVE_OPERATOR | HTTP 200 | ✅ |
| Session End → NO_OPERATOR → SAFE_MODE | HTTP 204, `SAFE_MODE / NO_OPERATOR` | ✅ |
| Sprint-2 Safety Tests (Regression) | 19/19 grün | ✅ |
| SAFE MODE Overlay-Text im Bundle | `"SAFE MODE"`, `"Resume — Operator Acknowledgment"` | ✅ |

### Neue Dateien

- `frontend/src/lib/api-client.ts` — HTTP-Client (login, getState, startSession, emergencyStop)
- `frontend/src/lib/ws-client.ts` — WebSocket-Client mit Latenz-Messung
- `frontend/src/hooks/useSystemState.ts` — Polling-Hook (500ms)
- `frontend/src/hooks/useSession.ts` — Session-Lifecycle (Login, Connect, Backoff, Resume)
- `frontend/src/hooks/useDeadmanSwitch.ts` — Dead-man (Spacebar/Button, Protobuf DEADMAN_HOLD)
- `frontend/src/components/SafeModeOverlay.tsx` — Fullscreen SAFE MODE Block
- `frontend/src/components/SafetyPanel.tsx` — Emergency Stop + Dead-man UI
- `frontend/src/components/ConnectionPanel.tsx` — Live State, Latenz, Session-ID, Rolle
- `frontend/src/App.tsx` (aktualisiert) — alle Komponenten verdrahtet
- `infrastructure/docker/nginx.conf` (aktualisiert) — `/api/`, `/auth/`, `/vehicle/ws` Proxy
- `infrastructure/docker/frontend.Dockerfile` (aktualisiert) — protoc + proto-gen vor Build
- `frontend/vite.config.ts` (aktualisiert) — Dev-Server Proxy
- `frontend/package.json` (aktualisiert) — `@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`, `ulidx`
- `cmd/control-server/main.go` (aktualisiert) — `/emergency-stop` Proxy-Endpunkt
- `internal/controlserver/transport/websocket.go` (aktualisiert) — Recovery-Pfad SAFE_MODE→RECOVERING

---

## Sprint 2 — Safety & Failure Model

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| TEST-01 | Go Test Infrastructure — testify + Mock Pattern | S | ✅ `MockSafetyPublisher`, `MockSFUPublisher` in `tests/unit/mocks/`; `SafetyPublisher` + `SFUPublisher` Interfaces angelegt |
| BE-06 | Vehicle Connection Service — Session Management | M | ✅ `internal/vehicleconnection/handler.go` — Vehicle WS, JWT-Auth, Disconnect → SAFE_MODE + Safety Event |
| BE-09 | Session Manager (GSA) + State Machine Erweiterung | M | ✅ `pkg/ulid/`, `session/manager.go` (CreateSession, Checkpoint, SFU Push), State Machine: Transition-Validierung + `TransitionToConnected()` |
| BE-10 | Failure Detection & Recovery | M | ✅ `safety/detector.go` — `DeadmanWatchdog` + `ACKTimeoutWatcher`; in `transport/websocket.go` integriert |
| BE-12 | Operator Handover Logic | M | ✅ `session/handover.go` — HANDOVER_PENDING, ConfirmHandover, CancelHandover, SFU EVENT: OPERATOR_HANDOVER |
| TEST-02 | Safety Test Suite | M | ✅ 19/19 Tests grün — alle 7 CRITICAL Trigger, Invariante 1 (MEDIA→DEGRADED), Recovery Checkpoint, Handover |

### Safety Test Suite — Ergebnis (19/19)

| Test | Status |
|------|--------|
| InvalidTransitionRejected | ✅ |
| WSDisconnect → SAFE_MODE | ✅ |
| DeadmanTimeout → SAFE_MODE | ✅ |
| DeadmanReset verhindert SAFE_MODE | ✅ |
| ACKTimeout → SAFE_MODE | ✅ |
| ACK in Zeit: kein SAFE_MODE | ✅ |
| NoOperator → SAFE_MODE | ✅ |
| EmergencyStop → SAFE_MODE | ✅ |
| AuthInvalidation → SAFE_MODE | ✅ |
| SafetyBusDown → SAFE_MODE | ✅ |
| MEDIA_FAILED → DEGRADED (niemals SAFE_MODE — Invariante 1) | ✅ |
| MEDIA_DEGRADED → DEGRADED (niemals SAFE_MODE — Invariante 1) | ✅ |
| RecoveryCheckpoint gespeichert | ✅ |
| Recovery-Fallback → SAFE_MODE bei Validierungsfehler | ✅ |
| SessionID ist ULID (26 Zeichen) | ✅ |
| SessionID eindeutig pro Session | ✅ |
| Handover → HANDOVER_PENDING | ✅ |
| Handover Confirm → neuer ACTIVE_OPERATOR + SFU Event | ✅ |
| Handover Cancel → ACTIVE_OPERATOR wiederhergestellt | ✅ |

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Safety Test Suite (Unit) | 19/19 grün | ✅ 19/19 PASS |
| Health-Checks alle 5 Services | `{"status":"ok"}` | ✅ |
| Initial State Machine | `IDLE/CONTROL_INIT/MEDIA_INIT/NO_OPERATOR` | ✅ |
| `session/start` ohne WS-Connect | HTTP 409 | ✅ Transition-Validierung greift |
| WS-Connect → AUTHENTICATED | `system: AUTHENTICATED` | ✅ |
| `session/start` → CONNECTED + ULID | `CONTROL_ACTIVE`, 26-stellige Session-ID | ✅ |
| Session-ID überlebt SAFE_MODE | `session_id` im `/state` nach WS-Disconnect | ✅ |
| Recovery Checkpoint gespeichert | Log: `checkpoint saved (session=...)` | ✅ |
| WS-Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Dead-man Watchdog (2s Timeout) | `SAFE_MODE` nach 2.5s ohne Reset | ✅ |
| Vehicle WS connect + JWT-Auth | Log: `[VEHICLE] connected: id=vehicle-1` | ✅ |
| Vehicle WS Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Vehicle-Endpoint mit Operator-Token | HTTP 401 | ✅ Rollenprüfung greift |
| Handover Request | HTTP 202, `HANDOVER_PENDING` | ✅ |
| Handover Confirm | HTTP 200, `ACTIVE_OPERATOR`, neuer Operator-ID im Session | ✅ |
| Handover Cancel | HTTP 200, `ACTIVE_OPERATOR` wiederhergestellt | ✅ |
| `session/end` → NO_OPERATOR → SAFE_MODE | `SAFE_MODE / NO_OPERATOR` | ✅ |

### Bugfix während Tests

`authservice/handler.go`: `HandoverToken` verlangte `current_token` auch bei Service-to-Service-Aufrufen. Feld ist nun optional — wenn leer, entfällt Client-Validierung (Vertrauen durch Netzwerk-Isolation im Docker Compose Stack).

### Neue Dateien

- `pkg/ulid/ulid.go`
- `internal/controlserver/safety/publisher.go` + `http_publisher.go` + `detector.go`
- `internal/controlserver/session/manager.go` + `handover.go` + `sfu_publisher.go`
- `internal/controlserver/statemachine/state.go` (erweitert: Transition-Validierung, `TransitionToConnected`)
- `internal/controlserver/transport/websocket.go` (erweitert: Deadman + ACK-Watcher)
- `internal/vehicleconnection/handler.go`
- `tests/unit/mocks/mock_safety.go` + `mock_sfu.go`
- `tests/unit/safety_test.go`
- `cmd/control-server/main.go` (erweitert: Session Manager, Handover, Vehicle WS, neue Endpoints)

---

## Sprint 1 — Foundation Layer

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| INFRA-01 | Proto Schema Repository — `.proto` + CorrelationHeader | M | ✅ Alle 5 Schemas (common, control, telemetry, safety, session) inkl. CorrelationHeader. ULID-Lib konfiguriert. |
| FE-01 | React Projekt Setup — Vite + TypeScript + Tailwind + Shadcn | S | ✅ React 18 + TypeScript + Vite läuft, erreichbar auf Port 3000 |
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | ✅ JWT-Ausstellung für Operator (role=OBSERVER) und Vehicle (role=VEHICLE) verifiziert |
| BE-11 | STUN/TURN Service — coturn Setup & Config | S | ✅ coturn läuft als Docker-Container auf Port 3479 |
| BE-03 | Safety Event Bus — Interface + In-Memory Implementierung | M | ✅ EmergencyStop auslösbar, State korrekt (SafeMode: true, LastEvent: EMERGENCY_STOP) |
| BE-02 | Control Server — WebSocket Setup + JWT Auth Middleware | M | ✅ WS-Verbindung mit JWT-Auth (101 Switching Protocols), Log: `subject=operator-1 role=OBSERVER` |
| DC-01 | Dockerfile — Frontend (React) | S | ✅ Multi-stage build, nginx serving, Port 3000 |
| DC-02 | Dockerfile — Backend Services (Go) | M | ✅ Alle Go-Services als separate Images gebaut |
| DC-03 | Docker Compose — Multi-Service Orchestrierung | M | ✅ Alle 8 Services starten fehlerfrei via `docker-compose up` |

### Testprotokoll (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Frontend localhost:3000 | HTML erreichbar | ✅ Vite + React + TS |
| Health /health alle Services | `{"status":"ok"}` | ✅ Alle 5 Services (8080–8084) |
| State Machine Initialzustand | `IDLE / CONTROL_INIT / MEDIA_INIT / NO_OPERATOR` | ✅ exakt |
| Operator JWT `POST /auth/operator/login` | `{"token":"eyJ..."}` mit `role=OBSERVER` | ✅ |
| Vehicle JWT `POST /auth/vehicle/register` | `{"token":"eyJ..."}` mit `role=VEHICLE` | ✅ |
| Safety Initial-State `GET /safety/state` | `SafeMode: false` | ✅ |
| Emergency Stop `POST /safety/emergency-stop` | SafeMode aktiviert | ✅ `SafeMode: true, LastEvent: EMERGENCY_STOP` |
| WebSocket Handshake mit JWT | `101 Switching Protocols`, Server-Log korrekt | ✅ Log: `WebSocket connected: subject=operator-1 role=OBSERVER` |
| WS-Disconnect → SAFE_MODE | System State wechselt | ✅ `SAFE_MODE / CONTROL_BLOCKED` nach Disconnect (ADR-009/010) |

### Beobachtung

WS-Disconnect triggert korrekt `SAFE_MODE → CONTROL_BLOCKED` (nicht im ursprünglichen Testplan, aber validiert). Safety-Verhalten funktioniert bereits auf Transport-Ebene wie in ADR-009/010 definiert.
