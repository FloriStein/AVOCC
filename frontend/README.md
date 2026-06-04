# AVOC Frontend

React 18 + TypeScript + Vite — Teleoperation Control Center UI.

Teil des AVOC-Systems. Vollständige Projektdokumentation: [../README.md](../README.md)

---

## Starten

**Im Docker-Stack (empfohlen):**
```bash
# Aus dem Repo-Root:
make up
# → http://localhost:3000
```

**Lokal mit Hot-Reload:**
```bash
# 1. Proto-Dateien generieren (einmalig, oder nach proto/-Änderungen)
make proto-gen-ts   # aus Repo-Root — generiert frontend/src/gen/*.ts

# 2. Dependencies installieren
npm install

# 3. Dev-Server starten
npm run dev
# → http://localhost:5173
# Voraussetzung: docker-compose up läuft für die Backend-Services
```

Vite proxied automatisch:

| Pfad | Ziel |
|------|------|
| `/ws` | ws://localhost:8080 (Control Server WebSocket) |
| `/api/` | http://localhost:8080 (Control Server REST) |
| `/auth/` | http://localhost:8081 (Auth Service) |
| `/sfu/` | http://localhost:8084 (WebRTC SFU) |
| `/telemetry/` | http://localhost:8083 (Telemetry Service) |

> **Wichtig:** `src/gen/*.ts` ist gitignored. Nach jedem `git clone` muss `make proto-gen-ts` einmal ausgeführt werden, bevor `npm run dev` funktioniert.

---

## Proto-Code generieren

TypeScript-Klassen werden build-time aus `../proto/*.proto` generiert:

```bash
# Via Docker (kein lokales protoc nötig):
make proto-gen-ts    # aus Repo-Root

# Oder lokal (protoc + protoc-gen-es muss installiert sein):
npm run proto-gen
```

Generierte Dateien landen in `src/gen/` (gitignored). Beide Schemas sind im Bundle:
- `control_pb.js` — `ControlCommandSchema`, `ControlAckSchema`, `CommandType`
- `common_pb.js` — `CorrelationHeaderSchema`

---

## Build

```bash
npm run build   # TypeScript-Kompilierung + Vite Build → dist/
npm run lint    # ESLint
```

---

## Struktur

```
src/
├── components/
│   ├── ConnectionPanel.tsx   # SYSTEM STATE, Latenz, Session-ID, Operator-Rolle, Speed/Battery
│   ├── ControlPanel.tsx      # Virtual Joystick SVG, Speed Slider, Steer/Throttle Bars, Modus-Anzeige
│   ├── SafeModeOverlay.tsx   # Fullscreen-Block bei SAFE_MODE, Resume-Button
│   ├── SafetyPanel.tsx       # Emergency Stop + Dead-man Switch
│   └── VideoPanel.tsx        # WebRTC Video Element, MEDIA STATE Badge, Overlays, Retry
├── hooks/
│   ├── useControls.ts        # 20 Hz Keyboard/Joystick/Gamepad → Protobuf STEER/THROTTLE/BRAKE/SPEED
│   ├── useDeadmanSwitch.ts   # Spacebar/Mousedown → DEADMAN_HOLD Commands (1500ms Interval)
│   ├── useSession.ts         # Login, WS-Connect, Session-Start, Reconnect (Exponential Backoff)
│   ├── useSystemState.ts     # Polling GET /api/state (500ms), 4-Layer State
│   ├── useTelemetry.ts       # Polling GET /telemetry/latest/{vehicleId} (1000ms)
│   └── useWebRTC.ts          # RTCPeerConnection, SDP Signaling via /sfu/subscribe/, MEDIA STATE
├── lib/
│   ├── api-client.ts         # HTTP-Client (login, startSession, emergencyStop, reportMediaState)
│   └── ws-client.ts          # WebSocket-Client, Protobuf ControlAck parsen, Latenz-Messung
└── gen/                      # Protobuf-generiert — gitignored
    ├── common_pb.js
    ├── control_pb.js
    └── ...
```

---

## Tech-Stack (ADR-013)

| Bereich | Technologie |
|---------|------------|
| Framework | React 18 |
| Sprache | TypeScript |
| Build Tool | Vite |
| Styling | Tailwind CSS v4 |
| Protobuf | @bufbuild/protobuf + @bufbuild/protoc-gen-es |
| ULID | ulidx |

---

## Implementierter Funktionsumfang

### Control Panel (Sprint 5 — FE-05)
- **Keyboard:** WASD + Pfeiltasten → STEER/THROTTLE Commands
- **Virtual Joystick:** SVG mit Pointer-Events; X-Achse = STEER, Y-Achse = THROTTLE
- **Gamepad API:** Left Stick → STEER/THROTTLE, Left Trigger (Button 6) → BRAKE
- **Speed Slider:** Multiplikator 10–100 % skaliert alle Achswerte
- **20 Hz Command Loop:** Protobuf `ControlCommand` via WebSocket, 50ms Interval
- **Priorität:** Gamepad > Joystick > Keyboard
- **CONTROL_BLOCKED:** ControlPanel deaktiviert im SAFE_MODE

### Video Panel (Sprint 5 — FE-06)
- **RTCPeerConnection:** SDP Offer/Answer via `POST /sfu/subscribe/{sessionId}/{operatorId}`
- **MEDIA STATE Tracking:** INIT → NEGOTIATING → CONNECTED / FAILED → DEGRADED (Invariante 1)
- **Reporting:** `POST /api/media/event` bei jeder MEDIA STATE-Änderung → Control Server
- **Overlays:** MEDIA_NEGOTIATING (Spinner-Text), MEDIA_FAILED (Warnung + Retry), MEDIA_CONNECTED (Video)
- **Auto-Connect:** startet wenn Session aktiv; trennt bei Disconnect/SAFE_MODE

### Dashboard Integration (Sprint 5 — FE-07)
- **Telemetrie:** Speed (km/h), Battery (%), Status via MQTT; 1 Hz Polling
- **Operator-Rolle:** Header zeigt ACTIVE_OPERATOR / HANDOVER_PENDING etc.
- **Protobuf ControlAck:** `fromBinary()` parst `success` und `error_msg` aus Server-Response
- **Fallback-Handling:** Server sendet Protobuf binary (nie JSON) bei fehlender Session

### Verhalten bei Systemzuständen

| Zustand | Verhalten |
|---------|-----------|
| CONNECTED | Control Panel aktiv, Video verbindet |
| DEGRADED | Control möglich, Video-Warnung sichtbar |
| SAFE_MODE | ControlPanel deaktiviert, SafeModeOverlay, kein Video |
| RECOVERING | WS reconnect läuft, Operator-Ack erforderlich |
