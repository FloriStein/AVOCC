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
npm install
npm run dev
# → http://localhost:5173
# Vite proxied /api/ → :8080, /auth/ → :8081, /ws → :8080
# Voraussetzung: docker-compose up läuft für die Backend-Services
```

---

## Proto-Code generieren

TypeScript-Klassen werden build-time aus `../proto/*.proto` generiert:

```bash
# Via Docker (kein lokales protoc nötig):
make proto-gen-ts    # aus Repo-Root

# Oder lokal (protoc + protoc-gen-es muss installiert sein):
npm run proto-gen
```

Generierte Dateien landen in `src/gen/` (gitignored).

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
│   ├── ConnectionPanel.tsx   # Live-Latenz, State-Badge, Session-ID, Operator-Rolle
│   ├── SafeModeOverlay.tsx   # Fullscreen-Block bei SAFE_MODE, Resume-Button
│   └── SafetyPanel.tsx       # Emergency Stop + Dead-man Switch
├── hooks/
│   ├── useDeadmanSwitch.ts   # Spacebar/Mousedown → DEADMAN_HOLD Commands
│   ├── useSession.ts         # Login, WS-Connect, Session-Start, Reconnect
│   └── useSystemState.ts     # Polling GET /api/state (500ms)
├── lib/
│   ├── api-client.ts         # HTTP-Client (login, startSession, emergencyStop)
│   └── ws-client.ts          # WebSocket-Client mit Latenz-Messung
└── gen/                      # Protobuf-generiert — gitignored
    ├── common_pb.ts
    ├── control_pb.ts
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

**Sprint-3-Einschränkungen (werden in Sprint 4 behoben):**
- State-Sync via Polling (500ms) — WebSocket-State-Push kommt mit BE-04
- Server antwortet auf ControlCommand mit `{"ack":true}` (kein Protobuf ControlAck bis BE-04)
- Control Panel (Joystick/Keyboard/Gamepad) disabled bis BE-04 fertig
