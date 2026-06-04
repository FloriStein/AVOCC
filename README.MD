# Autonomous Vehicle Operational Control Center (AVOC)

Sicheres, modulares Echtzeit-Teleoperation-System zur Fernsteuerung und Überwachung von Fahrzeugen über das offene Internet.

---

## Überblick

Das AVOC-System ermöglicht einem Operator, ein Fahrzeug über eine Netzwerkverbindung zuverlässig zu steuern, zu überwachen und in kritischen Situationen sofort einzugreifen — auch über unzuverlässige 5G/Internet-Verbindungen mit NAT.

**Kernprinzipien:**
- Safety First — SAFE MODE ist nicht überbrückbar
- Control-plane-driven — Control Server = Brain, SFU = Reflex
- Correlation-First — alle Events systemweit nachverfolgbar (ULID Session-ID)
- Explicit Failure Model — jeder Fehlerfall deterministisch behandelt

---

## Schnellstart

```bash
# Proto-Code generieren (einmalig nach git clone oder proto/-Änderungen)
make proto-gen-ts    # TypeScript via Docker (kein lokales protoc nötig)

# Vollständiges System starten
make up

# System stoppen
make down

# Tests
make test               # alle Go-Tests
make test-safety        # Safety Gate — muss 19/19 grün bleiben
make test-integration   # Integration Tests (Stack startet/stoppt automatisch)
make test-latency       # ACK-Roundtrip Benchmark <100ms (CI Build-Fail)
make test-k6            # k6 Lasttest 10 VU / 30s
```

**Voraussetzungen:** Docker, Docker Compose, Go 1.23

---

## Systemarchitektur

```
CONTROL HUB (Safety Truth)          VIDEO HUB (Awareness only)
┌──────────────────────┐            ┌─────────────────────┐
│    Control Server    │            │     WebRTC SFU      │
│  · State Machine     │──Events───▶│  · Media Relay      │
│  · Session Manager   │            │  · Recording        │
│  · Safety Engine     │            │  · Multi-Operator   │
└──────────────────────┘            └─────────────────────┘
         │
    ┌────┴────┬────────────┐
    ▼         ▼            ▼
Auth      Safety Bus   MQTT/Mosquitto
Service   (Go)         (Telemetry)
```

**4-Layer State Machine:** SYSTEM → CONTROL → MEDIA → OPERATOR  
**Netzwerk:** Vehicle ↔ Internet ↔ OCC (NAT Traversal via coturn)

---

## Technologie-Stack

| Bereich | Technologie |
|---------|-------------|
| Backend | Go 1.23 |
| Frontend | React 18 + TypeScript + Vite + Tailwind + Shadcn/ui |
| Video | WebRTC SFU (Pion/Go) + coturn |
| Messaging | Protobuf (Control/Safety/Telemetry) |
| MQTT | Eclipse Mosquitto |
| Auth | JWT (Operator + Vehicle, Separate Service) |
| Session-ID | ULID (zeitlich sortierbar) |
| Deployment | Docker Compose |

---

## Projektstruktur

```
proto/          .proto Schemas (Single Source of Truth)
gen/            Generierter Code (gitignored — make proto-gen)
cmd/            Go Service Entry Points
internal/       Service-interne Pakete
pkg/            Shared Go-Pakete (ulid, logger, audit)
frontend/       React App
infrastructure/ Docker + Docker Compose
tests/          Unit / Integration / E2E
docs/           Architektur, ADRs, Requirements
tasks/          Sprint-Planung (backlog, current-sprint, done)
```

---

## Dokumentation

| Dokument | Inhalt |
|----------|--------|
| [docs/vision.md](docs/vision.md) | Projektvision, Ziele, Erfolgskriterien |
| [docs/requirements.md](docs/requirements.md) | Funktionale & nichtfunktionale Anforderungen |
| [docs/architecture.md](docs/architecture.md) | Systemdesign, Komponenten, Verzeichnisstruktur |
| [DOKU.MD](DOKU.MD) | Vollständige Projektdokumentation für neue Entwickler (inkl. Codebeispiele) |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Phasen, Tasks, ADR-Index (18 ADRs) |
| [docs/adr/](docs/adr/) | Alle Architecture Decision Records |
| [DECISIONS.MD](DECISIONS.MD) | Live-Übersicht aller Entscheidungen |
| [CONTEXT.MD](CONTEXT.MD) | Glossar, Domänenmodell, Architekturprinzipien |
| [tasks/backlog.md](tasks/backlog.md) | Vollständiger Task-Backlog |
| [tasks/done.md](tasks/done.md) | Abgeschlossene Sprints (1–7) |

---

## Safety-Modell (Kurzfassung)

| Kanal | Failure-Reaktion |
|-------|-----------------|
| Control (WebSocket) | CRITICAL → SAFE MODE |
| Safety Bus | CRITICAL → SAFE MODE |
| Video (WebRTC) | DEGRADED — kein SAFE MODE |
| Telemetry (MQTT) | DEGRADED |
| Auth (laufende Session) | OBSERVATION |

Nach CRITICAL Failure: kein automatisches Resume — immer Operator-Ack erforderlich.
