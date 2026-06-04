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
| http://localhost:3001 | Grafana (Log-Dashboard) |
| http://localhost:3100 | Loki (Log-Aggregation API) |

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
make proto-gen-ts       # TypeScript (via Docker) — einmalig vor npm run dev erforderlich

# Alle Go-Services bauen
make build

# Tests
make test               # alle Go-Tests
make test-safety        # Safety Test Suite (CI Safety Gate — muss 19/19 bleiben)
make test-integration   # Integration Tests (startet/stoppt Test-Stack automatisch)
make test-latency       # Go Benchmark ACK-Roundtrip <100ms (ADR-010 Build-Fail)
make test-k6            # k6 Load Test 10 VU / 30s (benötigt Docker)

# Frontend Tests
cd frontend && npm test           # Vitest Component-Tests (31 Tests)
cd frontend && npm run test:e2e   # Playwright E2E (benötigt laufenden Stack)

# Stack stoppen
make down
```

**Lokales Frontend mit Hot-Reload:**
```bash
# 1. Proto-Dateien generieren (einmalig nach git clone oder proto/-Änderungen)
make proto-gen-ts

# 2. Dependencies installieren
cd frontend && npm install

# 3. Dev-Server starten (benötigt laufenden Backend-Stack)
cd frontend && npm run dev
# → http://localhost:5173
```

---

## Troubleshooting

### 502 Bad Gateway nach Container-Rebuild
nginx cached Docker-IPs beim Start. Fix:
```bash
docker exec avoc-frontend-1 nginx -s reload
```
Ursache: `set $upstream` + `rewrite...break` — `set` muss vor `rewrite` stehen (nginx Rewrite-Modul).

### `npm run dev` schlägt fehl: `Cannot find @/gen/control_pb.js`
Proto-Dateien fehlen lokal. Fix (einmalig, aus Repo-Root):
```bash
make proto-gen-ts
```
Hintergrund: `frontend/src/gen/` ist gitignored — wird nur im Docker-Build und via `make proto-gen-ts` generiert.

### `npm run dev` schlägt fehl: `Cannot find @rollup/rollup-linux-x64-gnu`
`node_modules` wurde in Docker (Alpine/musl) als root installiert. Fix:
```bash
# Root-owned node_modules via Docker löschen
docker run --rm \
  -v $(PWD)/frontend:/app -w /app node:22-alpine \
  sh -c 'rm -rf node_modules package-lock.json'
# Neu installieren auf Host-Platform
cd frontend && npm install
```

### Port-Konflikte beim Stack-Start
```bash
lsof -i :3000   # Frontend (nginx)
lsof -i :8080   # Control Server
lsof -i :8081   # Auth Service
lsof -i :8084   # WebRTC SFU
```
Test-Stack läuft auf Ports 18080–18082 (kein Konflikt mit Dev-Stack).

### WSL2: Services nicht erreichbar über `localhost`
WSL2 hat eine eigene IP-Adresse. `.env` und `frontend/vite.config.ts` ggf. anpassen:
```bash
# WSL2-IP ermitteln:
hostname -I | awk '{print $1}'
```

### `make test-integration` schlägt fehl: Services nicht erreichbar
Test-Stack braucht ggf. mehr Zeit. Timeout erhöhen oder manuell starten:
```bash
docker compose -f tests/docker-compose.test.yml up --build -d
sleep 10
go test ./tests/integration/... -v -timeout 120s
docker compose -f tests/docker-compose.test.yml down
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

**Phasen 1–7 abgeschlossen — 42/42 Tasks ✅**

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

**Sprint 6 (Testing & Quality Gates) ✅:** 31 Vitest Component-Tests · 9 Go Integration Tests · Go Benchmark + k6 Latenz-CI (<100ms Build-Fail) · README Troubleshooting · Playwright E2E Baseline

**Sprint 7 (Logging & Audit Trail) ✅:** pkg/logger (slog, JSON) · pkg/audit (SQLiteAuditWriter, WAL+fsync) · Loki + Grafana + Promtail · Frontend logger.ts · POST /log · GET /audit/events · AuditWriter.WriteSync() vor jeder SAFE_MODE-Transition

---

## Sprint-Stand

| Sprint | Inhalt | Status |
|--------|--------|--------|
| Sprint 1 | Foundation Layer — Proto, Auth, WebSocket, Docker Compose | ✅ |
| Sprint 2 | Safety & Failure Model — State Machine, Watchdogs, Handover, Tests | ✅ |
| Sprint 3 | Frontend Core — WebSocket Client, State-Polling, SAFE MODE, E-Stop | ✅ |
| Sprint 4 | Core Backend Services — Command Engine, MQTT, Recording, WebRTC SFU | ✅ |
| Sprint 5 | Feature Completion Frontend — Control Panel, Video Panel, Dashboard, Telemetrie | ✅ |
| Sprint 6 | Testing & Quality Gates — Vitest 31 Tests, Integration Tests, k6 Latenz-CI, README | ✅ |
| Sprint 7 | Logging & Audit Trail — slog, SQLite AuditWriter, Loki + Grafana, Frontend logger.ts | ✅ |

→ Abgeschlossen: alle 7 Sprints | Backlog: [tasks/backlog.md](tasks/backlog.md)

---

## ADR-Übersicht (18 Entscheidungen)

Alle Architekturentscheidungen sind dokumentiert und unveränderlich. Neue Erkenntnisse führen zu einem neuen ADR.

Zuletzt hinzugefügt: [ADR-017](docs/adr/017-logging-strategy.md) (Hybrid Logging) · [ADR-018](docs/adr/018-audit-trail-strategy.md) (SQLite Audit Trail)

→ [docs/adr/README.md](docs/adr/README.md) | Live-Übersicht: [DECISIONS.MD](DECISIONS.MD)

---

## Contributor Guide

### Neues ADR erstellen
1. Kopiere [docs/adr/000-template.md](docs/adr/000-template.md) → `docs/adr/0XX-titel.md`
2. Fülle alle Pflichtfelder aus (Kontext, Optionen, Entscheidung, Konsequenzen)
3. Trage ADR in [DECISIONS.MD](DECISIONS.MD) und [docs/adr/README.md](docs/adr/README.md) ein
4. Aktualisiere [docs/implementation-plan.md](docs/implementation-plan.md) ADR-Index und Zähler

### Neuen Go-Service hinzufügen
1. Erstelle `cmd/<service-name>/main.go` mit `/health` Endpoint
2. Nutze `infrastructure/docker/go-service.Dockerfile` (wiederverwendbar via `SERVICE_NAME` ARG)
3. Ergänze Service in `infrastructure/compose/docker-compose.yml` + `tests/docker-compose.test.yml`
4. Füge `pkg/logger.New("<service-name>")` für strukturiertes Logging ein (Phase 7)

### Proto-Schema ändern
Field-based Versioning (ADR-012): **keine Field-IDs ändern**, keine Felder entfernen.
```bash
# 1. proto/*.proto ändern
# 2. Code generieren:
make proto-gen       # Go → gen/go/
make proto-gen-ts    # TypeScript → frontend/src/gen/
# gen/ ist gitignored — nie committen
```

### Neuen Frontend-Component erstellen
1. `frontend/src/components/<Name>.tsx`
2. Hooks in `frontend/src/hooks/use<Name>.ts`
3. Test: `frontend/src/components/<Name>.test.tsx` — Vitest + RTL
4. Mock externe Dependencies: `vi.mock('@/hooks/use...')`

### Safety-kritischen Code ändern
- Safety Tests (19/19) müssen grün bleiben: `make test-safety`
- Änderungen an `detector.go`, `statemachine.go`, `websocket.go` erfordern Test-Update
- SAFE_MODE-Transitionen: erst `AuditWriter.WriteSync()` (Phase 7 — ADR-018), dann Transition
