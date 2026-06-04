# AVOC — Autonomous Vehicle Operational Control Center

Sicheres, modulares Echtzeit-Teleoperation-System zur Fernsteuerung von Fahrzeugen über das offene Internet (Vehicle ↔ Internet ↔ OCC).

→ Vollständige Projektdokumentation: [docs/](docs/) | ADRs: [docs/adr/](docs/adr/) | Vision: [docs/vision.md](docs/vision.md)

---

## Schnellstart

**Voraussetzungen:** Docker, Docker Compose

```bash
# Umgebungsvariablen einrichten
cp .env.example .env
# JWT_SECRET in .env auf einen sicheren Wert setzen

# Alle Services starten (Build inklusive)
make up
# oder direkt:
docker compose -f infrastructure/compose/docker-compose.yml --env-file .env up --build
```

**Services nach Start:**

| URL | Service |
|-----|---------|
| http://localhost:3000 | Frontend (React Dashboard) |
| http://localhost:8080 | Control Server |
| http://localhost:8081 | Auth Service |
| http://localhost:8082 | Safety Service |
| http://localhost:8083 | Telemetry Service |
| http://localhost:8084 | WebRTC SFU |

---

## Architektur

Zwei orthogonale Hubs, vier Kommunikationskanäle:

```
CONTROL HUB (Rang 1 — Safety Truth)     VIDEO HUB (Rang 2 — Awareness only)
Control Server (Go)                      WebRTC SFU (Pion/Go)
  · 4-Layer State Machine                  · Media Relay
  · Safety Decision Engine                 · Server-seitiges Recording
  · Session Manager (GSA)                  · Multi-Operator Forwarding
  · Failure Detection
  · Operator Handover
```

**4-Layer State Machine:** SYSTEM STATE (Master) · CONTROL STATE · MEDIA STATE · OPERATOR STATE

**Kanäle:** WebSocket (Control) · MQTT (Telemetry) · Safety Event Bus (Go In-Memory) · WebRTC SFU (Video)

→ Details: [docs/architecture.md](docs/architecture.md)

---

## Entwicklung

```bash
# Proto-Code generieren (Go + TypeScript)
make proto-gen          # Go (via Docker)
make proto-gen-ts       # TypeScript (via Docker)

# Alle Go-Services bauen
make build

# Tests
make test               # alle Tests
make test-safety        # Safety Test Suite (Safety Gate)

# Stack stoppen
make down
```

**Lokales Frontend mit Hot-Reload:**
```bash
cd frontend && npm install && npm run dev
# → http://localhost:5173 (Vite dev server mit API-Proxy auf :8080/:8081)
```

---

## Projektstruktur

```
├── cmd/                    # Go Service Entry Points
├── internal/               # Go Service-interne Pakete
│   ├── authservice/
│   ├── controlserver/
│   │   ├── safety/         # Safety Decision Module (DeadmanWatchdog, ACKTimeout)
│   │   ├── session/        # Session Manager (GSA), Handover
│   │   ├── statemachine/   # 4-Layer State Machine
│   │   └── transport/      # WebSocket Transport Layer
│   ├── safetyservice/      # Safety Event Bus (In-Memory)
│   └── vehicleconnection/  # Vehicle WebSocket Handler
├── pkg/ulid/               # ULID-Wrapper (ADR-016)
├── proto/                  # .proto Source — Single Source of Truth
├── gen/                    # Generated Code — gitignored
├── frontend/               # React 18 + TypeScript + Vite + Tailwind
├── infrastructure/
│   ├── compose/            # docker-compose.yml
│   ├── docker/             # Dockerfiles, nginx.conf
│   ├── coturn/             # STUN/TURN Konfiguration
│   └── mosquitto/          # MQTT Broker Konfiguration
└── tests/unit/             # Safety Test Suite (19 Szenarien, Sprint 2)
```

---

## Implementierungsstand

**Phasen 1–5 abgeschlossen — 27/31 Tasks ✅**

| Feature | Implementiert |
|---------|--------------|
| Keyboard WASD/Pfeiltasten → Protobuf STEER/THROTTLE | Sprint 5 |
| Virtual Joystick SVG (Drag, 20 Hz Command Loop) | Sprint 5 |
| Gamepad API → STEER/THROTTLE/BRAKE | Sprint 5 |
| Speed Slider (skaliert alle Commands) | Sprint 5 |
| WebRTC RTCPeerConnection + SDP Signaling via SFU | Sprint 5 |
| MEDIA STATE Badge + DEGRADED-Overlay im Browser | Sprint 5 |
| Telemetrie-Anzeige Speed/Battery via MQTT | Sprint 5 |
| Operator-Rolle im Dashboard-Header | Sprint 5 |
| Protobuf ControlAck parsen (Latenz + error_msg) | Sprint 5 |
| nginx Docker DNS Resolver (kein 502 nach Rebuild) | Sprint 5 |

**Offen (Sprint 6):** Vitest+RTL Tests, Playwright E2E, k6 Latenz-CI (<100ms Build-Fail), README Troubleshooting

---

## Sprint-Stand

| Sprint | Inhalt | Status |
|--------|--------|--------|
| Sprint 1 | Foundation Layer — Proto, Auth, WebSocket, Docker Compose | ✅ |
| Sprint 2 | Safety & Failure Model — State Machine, Watchdogs, Handover, Tests | ✅ |
| Sprint 3 | Frontend Core — WebSocket Client, State-Polling, SAFE MODE, E-Stop | ✅ |
| Sprint 4 | Core Backend Services — Command Engine, MQTT, Recording, WebRTC SFU | ✅ |
| Sprint 5 | Feature Completion Frontend — Control Panel, Video Panel, Dashboard, Telemetrie | ✅ |
| Sprint 6 | Testing & Quality Gates — TEST-03, TEST-04, TEST-05, DC-04 | 🔲 |

→ Aktueller Sprint: [tasks/current-sprint.md](tasks/current-sprint.md) | Backlog: [tasks/backlog.md](tasks/backlog.md)

---

## ADR-Übersicht (18 Entscheidungen)

Alle Architekturentscheidungen sind dokumentiert und unveränderlich. Neue Erkenntnisse führen zu einem neuen ADR.

Zuletzt hinzugefügt: [ADR-017](docs/adr/017-logging-strategy.md) (Hybrid Logging) · [ADR-018](docs/adr/018-audit-trail-strategy.md) (SQLite Audit Trail)

→ [docs/adr/README.md](docs/adr/README.md) | Live-Übersicht: [DECISIONS.MD](DECISIONS.MD)
