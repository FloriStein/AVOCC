# Sprint 3 — Frontend Core

Ziel: Frontend kommuniziert live mit Backend. SYSTEM STATE sichtbar. SAFE MODE blockiert Steuerung. Emergency Stop und Dead-man Switch funktionieren.

Datum: 2026-06-03
Vorgänger: Sprint 2 ✅ (Safety & Failure Model — 19/19 Tests grün, alle CRITICAL Trigger verifiziert)

---

## Ausgangslage (aus Sprint 2)

| Was existiert | Stand |
|---------------|-------|
| `frontend/src/App.tsx` | UI-Shell vollständig: Header, Video Panel, Safety Panel, Connection Panel, Control Bar. Alles hardcoded/disabled. Kein Live-Zustand. |
| `frontend/package.json` | React 18 + TypeScript + Vite + Tailwind. Kein `@bufbuild/protobuf`, kein ULID. |
| `infrastructure/docker/nginx.conf` | Nur `/ws` proxied. Kein `/api/`-Proxy. |
| `gen/ts/` | Leer — Proto-Code-Gen für TypeScript nie ausgeführt. |
| `cmd/control-server/main.go` | Alle Sprint-2-Endpoints vorhanden (`/session/start`, `/handover/*`, `/state`). Kein `/emergency-stop`-Proxy. |
| WS-Handler | Akzeptiert alle Bytes, antwortet `{"ack":true}`. Kein Protobuf-Parsing serverseitig (bis BE-04). |
| Dead-man Watchdog | Resettet auf **jede** eingehende WS-Nachricht (Sprint-2-Vereinfachung, bis BE-04). |

**Sprint-3-Einschränkungen (explizit):**
- Frontend sendet Protobuf-codierte `ControlCommand`-Nachrichten, Server parst sie noch nicht (kommt mit BE-04 in Sprint 4)
- Server-ACK ist `{"ack":true}` (kein Protobuf `ControlAck` bis BE-04) → Latenzanzeige misst JSON-Roundtrip
- State-Sync via Polling `GET /api/state` (500ms Intervall) — WebSocket-State-Push kommt mit BE-04

---

## Tasks

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| FE-09 | Frontend Protobuf Adapter + Build-Pipeline | M | 🔲 Todo | ADR-012b/013/016; `@bufbuild/protobuf` + `@bufbuild/protoc-gen-es`; Dockerfile anpassen; nginx API-Proxy; gen in `frontend/src/gen/` (gitignored) |
| FE-02 | WebSocket Client + State-Polling | M | 🔲 Todo | ADR-008/010/011/012b; `src/lib/ws-client.ts`; JWT-Auth; Reconnect mit Exponential Backoff; Polling `GET /api/state`; `useSystemState` Hook |
| FE-08 | SAFE MODE Overlay + Operator Ack Flow | M | 🔲 Todo | ADR-009/011/015; Overlay blockiert alle Inputs; Resume-Flow (reconnect + `/api/session/start`); DEGRADED-Banner |
| FE-04 | Safety Controls — Emergency Stop + Dead-man Switch | M | 🔲 Todo | ADR-009/015; E-Stop → `POST /api/emergency-stop`; Dead-man via Spacebar/Mousedown (hält WS-Reset aktiv); Operator-Ack vor erster Aktivierung |
| FE-03 | Connection Status — Live-Anzeige | S | 🔲 Todo | ADR-016; Latenz (JSON ACK-Roundtrip), SYSTEM STATE Badge, Session-ID, Operator-Rolle |

---

## Abhängigkeitspfad

```
FE-09 (Proto + Build-Pipeline + nginx) — sofort startbar
  │
  ▼
FE-02 (WS Client + State-Polling) ──────────────────────────────┐
  │                                                             │
  ├──▶ FE-08 (SAFE MODE Overlay + Ack Flow)                    │
  ├──▶ FE-04 (Emergency Stop + Dead-man)                       │
  └──▶ FE-03 (Connection Status — S-Task, schnell)             │
                                                                ▼
                                                   Sprint 3 DoD erfüllt ✓
```

---

## Implementierungsdetails je Task

### FE-09 — Protobuf Adapter + Build-Pipeline

**`frontend/package.json` — neue Dependencies:**
```json
"dependencies": {
  "@bufbuild/protobuf": "^2.x"
},
"devDependencies": {
  "@bufbuild/protoc-gen-es": "^2.x"
},
"scripts": {
  "proto-gen": "protoc --proto_path=../proto --es_out=src/gen --es_opt=target=ts ../proto/*.proto",
  "prebuild": "npm run proto-gen",
  "build": "tsc -b && vite build"
}
```

**`infrastructure/docker/frontend.Dockerfile`** — protoc-Installation vor Build:
```dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache protobuf
RUN npm install -g @bufbuild/protoc-gen-es@latest
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
COPY proto/ ../proto/
RUN npm run proto-gen
RUN npm run build
```

**`frontend/.gitignore`** — `src/gen/` hinzufügen

**`infrastructure/docker/nginx.conf`** — API-Proxy ergänzen:
```nginx
location /api/ {
    proxy_pass http://control-server:8080/;
}
location /auth/ {
    proxy_pass http://auth-service:8081/auth/;
}
location /vehicle/ws {
    proxy_pass http://control-server:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

**`cmd/control-server/main.go`** — `/api/emergency-stop` Endpunkt hinzufügen (Proxy zu safety-service), damit Frontend keine Cross-Origin-Requests braucht.

**Generierte Dateien:** `frontend/src/gen/common_pb.ts`, `control_pb.ts`, `safety_pb.ts`, `session_pb.ts`, `telemetry_pb.ts`

---

### FE-02 — WebSocket Client + State-Polling

**`frontend/src/lib/ws-client.ts`:**
```typescript
class WSClient {
  connect(wsUrl: string, token: string): void
  sendCommand(cmd: ControlCommand): Promise<number> // returns latency ms
  disconnect(): void
  onClose: (() => void) | null
}
```
- Reconnect: Exponential Backoff (1s, 2s, 4s, 8s, max 30s) nach Channel Close
- Latenz-Messung: `Date.now()` vor Send — `Date.now()` nach ACK-Empfang
- CorrelationHeader: `session_id` aus Context + `event_id` via ULID (`ulidx` npm package)

**`frontend/src/lib/api-client.ts`:**
```typescript
login(operatorId: string): Promise<string>          // → JWT token
startSession(vehicleId: string, operatorId: string): Promise<string>  // → session_id
getState(): Promise<SystemStateResponse>
emergencyStop(sessionId: string, vehicleId: string): Promise<void>
```

**`frontend/src/hooks/useSystemState.ts`:**
- Pollt `GET /api/state` alle 500ms
- Gibt `{ system, control, media, operator, sessionId, latency }` zurück
- Stoppt Polling bei `SAFE_MODE` (kein unnötiger Traffic)

**`frontend/src/hooks/useSession.ts`:**
- Verwaltet Login-Flow → JWT → WS-Connect → Session-Start
- Exponential Backoff Reconnect nach SAFE_MODE
- Stellt `sessionId`, `operatorRole`, `connect()`, `disconnect()` bereit

---

### FE-08 — SAFE MODE Overlay + Operator Ack Flow

**`frontend/src/components/SafeModeOverlay.tsx`:**
- Fullscreen-Overlay über allem (z-index hoch)
- Zeigt Safety Reason aus letztem State
- "Resume — Operator Acknowledgment" Button
- Klick → `connect()` (Reconnect) → `startSession()` → CONNECTED
- DEGRADED: kein Overlay, aber gelber Banner über Video Panel

**App.tsx-Integration:**
- `{systemState === 'SAFE_MODE' && <SafeModeOverlay />}` — overlay erscheint
- Alle Steuerungselemente werden bei `systemState === 'SAFE_MODE'` disabled

---

### FE-04 — Safety Controls

**`frontend/src/components/SafetyPanel.tsx`** (extrahiert aus App.tsx):

**Emergency Stop:**
- `POST /api/emergency-stop` mit `{session_id, vehicle_id, reason: "operator"}`
- Button disabled wenn `systemState === 'SAFE_MODE'` (bereits in SAFE_MODE)
- Button rot, Hover-State klar

**Dead-man Switch:**
- `mousedown` / Spacebar gedrückt → `active = true`
- `mouseup` / Spacebar losgelassen → `active = false` → Server-Watchdog läuft ab → SAFE_MODE
- Während aktiv: alle 500ms eine WS-Nachricht senden (reset deadman watchdog)
- Visual: grüner Halo wenn aktiv, grau wenn inaktiv

**`frontend/src/hooks/useDeadmanSwitch.ts`:**
```typescript
function useDeadmanSwitch(wsClient: WSClient | null, sessionId: string): {
  isActive: boolean
  bind: { onMouseDown, onMouseUp, onKeyDown, onKeyUp }
}
```
- Interval-Timer (400ms) sendet Protobuf `COMMAND_TYPE_DEADMAN_HOLD` solange aktiv
- Cleanup bei Unmount / SAFE_MODE-Eintritt

---

### FE-03 — Connection Status

**`frontend/src/components/ConnectionPanel.tsx`** (extrahiert aus App.tsx):
- SYSTEM STATE Badge (live, farbig)
- Latenz in ms (aus WSClient ACK-Roundtrip, aktualisiert bei jedem Command)
- Session-ID (gekürzt: erste 8 Zeichen + `…`)
- Operator-Rolle (ACTIVE_OPERATOR / OBSERVER / STANDBY)
- Verbindungsstatus-Dot (grün/gelb/rot)

---

## Infrastruktur-Änderungen

| Datei | Änderung |
|-------|---------|
| `infrastructure/docker/nginx.conf` | `/api/` und `/auth/` Proxy-Routen ergänzen |
| `infrastructure/docker/frontend.Dockerfile` | protoc + protoc-gen-es installieren + `npm run proto-gen` vor Build |
| `frontend/package.json` | `@bufbuild/protobuf` + `@bufbuild/protoc-gen-es` + `ulidx` |
| `frontend/.gitignore` | `src/gen/` ergänzen |
| `cmd/control-server/main.go` | `/api/emergency-stop` Proxy-Endpunkt |

---

## Sprint-Ziel / Definition of Done

- [ ] Protobuf-Klassen werden build-time generiert (`ControlCommand`, `ControlAck`, `CorrelationHeader`)
- [ ] Frontend loggt sich ein (JWT), verbindet WebSocket, startet Session
- [ ] SYSTEM STATE aus `/api/state` wird live im UI angezeigt (Polling 500ms)
- [ ] CONNECTED → State-Badge grün, Control-Elemente aktiv
- [ ] Dead-man Switch (Spacebar/Button halten) resettet Watchdog — Loslassen → SAFE_MODE nach 2s
- [ ] Emergency Stop → `POST /api/emergency-stop` → SAFE_MODE sofort
- [ ] SAFE MODE Overlay erscheint, blockiert alle Steuerungselemente vollständig
- [ ] "Resume / Operator Ack" reconnectet und startet neue Session
- [ ] DEGRADED State: gelber Banner sichtbar, Steuerung bleibt möglich
- [ ] Connection Panel zeigt Latenz, Session-ID, Operator-Rolle live
- [ ] `docker-compose up --build` → alle Features im Browser nutzbar
