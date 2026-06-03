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
└── tests/unit/             # Safety Test Suite (19 Szenarien)
```

---

## Sprint-Stand

| Sprint | Inhalt | Status |
|--------|--------|--------|
| Sprint 1 | Foundation Layer — Proto, Auth, WebSocket, Docker Compose | ✅ |
| Sprint 2 | Safety & Failure Model — State Machine, Watchdogs, Handover, Tests | ✅ |
| Sprint 3 | Frontend Core — WebSocket Client, State-Polling, SAFE MODE, E-Stop | ✅ |
| Sprint 4 | Core Backend Services — BE-04, BE-05, BE-07, BE-08 | 🔲 |
| Sprint 5 | Feature Completion Frontend — FE-05, FE-06, FE-07 | 🔲 |
| Sprint 6 | Testing & Quality Gates — TEST-03, TEST-04, TEST-05 | 🔲 |

→ Aktueller Sprint: [tasks/current-sprint.md](tasks/current-sprint.md) | Backlog: [tasks/backlog.md](tasks/backlog.md)

---

## ADR-Übersicht (16 Entscheidungen)

Alle Architekturentscheidungen sind dokumentiert und unveränderlich. Neue Erkenntnisse führen zu einem neuen ADR.

→ [docs/adr/README.md](docs/adr/README.md) | Live-Übersicht: [DECISIONS.MD](DECISIONS.MD)
