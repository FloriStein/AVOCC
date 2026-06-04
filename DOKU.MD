# AVOC — Projektdokumentation für neue Entwickler

**AVOC** (Autonomous Vehicle Operational Control Center) ist ein sicheres, modulares Echtzeit-Teleoperationssystem zur Fernsteuerung von Fahrzeugen über das offene Internet.

---

## Inhaltsverzeichnis

1. [Was ist AVOC?](#1-was-ist-avoc)
2. [Voraussetzungen](#2-voraussetzungen)
3. [Schnellstart](#3-schnellstart)
4. [Architekturübersicht](#4-architekturübersicht)
5. [Services im Überblick](#5-services-im-überblick)
6. [Projektstruktur](#6-projektstruktur)
7. [Die 4-Layer State Machine](#7-die-4-layer-state-machine)
8. [Sicherheitskonzepte](#8-sicherheitskonzepte)
9. [Schlüsselkonzepte & Glossar](#9-schlüsselkonzepte--glossar)
10. [Tech Stack](#10-tech-stack)
11. [Protobuf & Code-Generierung](#11-protobuf--code-generierung)
12. [Entwicklungs-Workflow](#12-entwicklungs-workflow)
13. [Tests ausführen](#13-tests-ausführen)
14. [Beitragen zum Projekt](#14-beitragen-zum-projekt)
15. [ADRs — Architekturentscheidungen](#15-adrs--architekturentscheidungen)
16. [Troubleshooting](#16-troubleshooting)
17. [Weiterführende Dokumentation](#17-weiterführende-dokumentation)
18. [Wichtige Codebeispiele](#18-wichtige-codebeispiele)

---

## 1. Was ist AVOC?

### Problem

Autonome oder halbautonome Fahrzeuge benötigen in vielen Szenarien eine menschliche Fernüberwachung oder manuelle Intervention. Die zentralen Herausforderungen dabei:

- hohe und schwankende Netzwerklatenz (mobiles Internet, 5G)
- unzuverlässige Verbindungen
- Sicherheitsrisiken bei Steuerungsverlust
- gleichzeitige Übertragung von Video, Steuerung und Telemetrie in Echtzeit

### Lösung

AVOC adressiert diese Probleme durch eine strukturierte, mehrschichtige Systemarchitektur mit klar getrennten Kommunikationskanälen und einem expliziten Sicherheitsmodell (SAFE MODE).

### Kernfunktionen

- Echtzeit-Steuerung eines Fahrzeugs über Keyboard, virtuellen Joystick oder Gamepad
- Live-Video über WebRTC (Multi-Kamera, NAT-traversal-fähig)
- Fahrzeugtelemetrie via MQTT (Geschwindigkeit, Batterie)
- Dead-man Switch und Emergency Stop als Safety-Mechanismen
- SAFE MODE: deterministischer Sicherheitszustand bei Verbindungsproblemen
- Audit Trail: vollständige Protokollierung aller Safety-Events

### Was AVOC nicht ist

- kein vollautonomes Fahrsystem
- kein Consumer-Produkt
- kein Offline-System

---

## 2. Voraussetzungen

### Pflicht

| Tool | Version | Zweck |
|------|---------|-------|
| Docker | >= 24 | Stack starten, Proto-Gen |
| Docker Compose | >= 2.x | Multi-Container Management |
| Go | >= 1.23 | Go-Services bauen und testen |
| Node.js | >= 22 | Frontend-Entwicklung |
| make | beliebig | Build-Befehle |

### Optional (für lokale Entwicklung ohne Docker)

| Tool | Zweck |
|------|-------|
| `protoc` | Proto-Code lokal generieren (alternativ via Docker: `make proto-gen-ts`) |
| `golangci-lint` | Go-Linting: `make lint` |
| `k6` | Lokale Lasttests ohne Docker |

### Empfohlene IDE-Plugins

- Go-Extension (VS Code: `golang.go`)
- TypeScript / ESLint für das Frontend

---

## 3. Schnellstart

```bash
# 1. Repository klonen
git clone <repo-url>
cd AutonomousVehicleOperationalControlCenter

# 2. Umgebungsvariablen einrichten
cp .env.example .env
# JWT_SECRET in .env auf einen sicheren Wert setzen (min. 32 Zeichen)

# 3. Protobuf-Code für TypeScript generieren (einmalig nach dem Klonen)
make proto-gen-ts

# 4. Gesamten Stack starten (Backend + Frontend + Infrastruktur)
make up
```

Nach dem Start sind folgende Services erreichbar:

| URL | Service |
|-----|---------|
| http://localhost:3000 | Frontend (React Dashboard) |
| http://localhost:8080 | Control Server (WebSocket + REST) |
| http://localhost:8081 | Auth Service (JWT) |
| http://localhost:8082 | Safety Service |
| http://localhost:8083 | Telemetry Service |
| http://localhost:8084 | WebRTC SFU |
| http://localhost:3001 | Grafana (Log-Dashboard, AVOC Session Dashboard) |
| http://localhost:3100 | Loki (Log-Aggregation API) |

> **Hinweis:** `gen/` und `frontend/src/gen/` sind gitignored und müssen nach jedem `git clone` oder nach Proto-Änderungen neu generiert werden.

### Lokales Frontend mit Hot-Reload

Für schnelle UI-Entwicklung ohne vollständigen Docker-Rebuild:

```bash
# Backend-Stack muss laufen (make up)
cd frontend
npm install
npm run dev
# → http://localhost:5173
```

---

## 4. Architekturübersicht

### Zwei orthogonale Hubs

Das System ist in zwei klar getrennte Hubs unterteilt:

```
CONTROL HUB (Rang 1 — Safety Truth)      VIDEO HUB (Rang 2 — Awareness only)
┌────────────────────────────────┐        ┌────────────────────────────────┐
│        Control Server          │        │        WebRTC SFU (Pion)       │
│  · 4-Layer State Machine       │        │  · Media Relay (Kamera-Streams)│
│  · Safety Decision Engine      │        │  · Server-seitiges Recording   │
│  · Session Manager (GSA)       │        │  · Multi-Operator Forwarding   │
│  · Failure Detection           │        │  · empfängt Session-Events     │
│  · Operator Handover           │        │    vom Control Hub             │
└──────┬───────────┬─────────────┘        └────────────────────────────────┘
       │           │
  Auth Service  Safety Bus   MQTT/Mosquitto
```

**Wichtigste Invariante:** Der VIDEO HUB darf den SYSTEM STATE (Safety) niemals direkt beeinflussen. Video-Probleme führen maximal zu DEGRADED, niemals zu SAFE MODE.

### Vier Kommunikationskanäle

| Kanal | Protokoll | Zweck | Latenz-Ziel |
|-------|-----------|-------|-------------|
| **Control** | WebSocket + Protobuf (sync ACK) | Steuerbefehle, < 100ms Roundtrip | < 100ms (CI Build-Fail) |
| **Video** | WebRTC SFU + coturn | Live-Video, NAT-traversal | 100–300ms (Soft-Ziel) |
| **Telemetry** | MQTT (Mosquitto) | Fahrzeugstatus, async | — |
| **Safety** | Go In-Memory Bus | Safety-Events, near-instant | Priority Channel |

### Session-Korrelation (ULID)

Alle Kanäle tragen eine gemeinsame `session_id`. So lassen sich Logs, Commands und Video-Frames einer Sitzung lückenlos zuordnen.

```
Vehicle-ID
  └── Session-ID (ULID — Root Anchor, überlebt SAFE MODE)
        └── Event-ID (ULID — pro Message / Command / Frame)
```

ULIDs sind zeitlich sortierbar und URL-safe. Der Control Server (GSA) ist der einzige Erzeuger von Session-IDs.

---

## 5. Services im Überblick

### Control Server (`cmd/control-server/`)

Das Herzstück des Systems. Verantwortlich für:

- Entgegennahme und Weiterleitung aller Steuerbefehle (WebSocket, Protobuf)
- 4-Layer State Machine (SYSTEM / CONTROL / MEDIA / OPERATOR STATE)
- Safety Decision Engine — klassifiziert Fehler als CRITICAL, DEGRADED oder OBSERVATION
- Global Session Authority (GSA) — einziger Session-Erzeuger
- JWT-Validierung im WebSocket-Handshake
- Operator Handover Management

**Port:** 8080

### Auth Service (`cmd/auth-service/`)

- JWT-Ausstellung für Operatoren und Fahrzeuge
- Operator-Rollen (ACTIVE_OPERATOR, OBSERVER, STANDBY)
- Handover-Token-Generierung

**Port:** 8081

### Safety Service (`cmd/safety-service/`)

- Safety Event Bus (Go, In-Memory)
- DDS-Interface-kompatibel (zukünftiger Austausch per ADR-002)
- Empfängt Safety-Events vom Control Server und verteilt sie

**Port:** 8082

### Telemetry Service (`cmd/telemetry-service/`)

- MQTT-Client (Eclipse Mosquitto)
- Bridget Fahrzeug-Telemetrie (Geschwindigkeit, Batterie) in den Stack
- Protobuf-Format, async

**Port:** 8083

### WebRTC SFU (`cmd/webrtc-sfu/`)

- Selective Forwarding Unit auf Basis von Pion (Go-native WebRTC)
- Empfängt Video-Streams vom Fahrzeug
- Leitet an alle verbundenen Operatoren weiter
- NAT-Traversal via coturn (STUN/TURN)
- Erhält Session-Events vom Control Server (SESSION_CREATED, SESSION_SAFE_MODE, etc.)

**Port:** 8084

### Frontend (`frontend/`)

React 18 + TypeScript SPA. Operatoren-Dashboard mit:

| Komponente | Funktion |
|------------|----------|
| `VideoPanel` | WebRTC Live-Video, MEDIA STATE Badge |
| `ControlPanel` | WASD/Joystick/Gamepad, 20 Hz Command Loop |
| `SafetyPanel` | Emergency Stop, Dead-man Switch (Spacebar) |
| `ConnectionPanel` | SYSTEM STATE, Latenz, Session-ID, Telemetrie |
| `SafeModeOverlay` | Fullscreen-Block, Operator-Ack für Recovery |

**Port:** 3000 (nginx im Docker), 5173 (Vite Dev-Server)

---

## 6. Projektstruktur

```
AutonomousVehicleOperationalControlCenter/
│
├── proto/                        # .proto Source — Single Source of Truth
│   ├── common.proto              # CorrelationHeader (session_id, event_id, ...)
│   ├── control.proto             # Steuerbefehle + ControlAck
│   ├── safety.proto              # Safety Events
│   ├── telemetry.proto           # Telemetrie
│   └── session.proto             # Session Events (SFU) + RecordingEntry
│
├── gen/                          # Generiert — gitignored, NIEMALS committen
│   ├── go/                       # protoc-gen-go Output
│   └── ts/                       # protoc-gen-es Output (TypeScript)
│
├── cmd/                          # Go Service Entry Points (main.go je Service)
│   ├── control-server/
│   ├── auth-service/
│   ├── safety-service/
│   ├── telemetry-service/
│   └── webrtc-sfu/
│
├── internal/                     # Go Service-interne Pakete (nicht extern nutzbar)
│   ├── controlserver/
│   │   ├── command/              # Command Engine: Protobuf-Parsing, Rate Limiting
│   │   ├── transport/            # WebSocket Layer: JWT-Verify, Heartbeat, Channel Close
│   │   ├── statemachine/         # 4-Layer State Machine
│   │   ├── safety/               # Safety Decision Module (Watchdogs, ACK-Timeout)
│   │   └── session/              # Session Manager / GSA, Handover
│   ├── authservice/
│   ├── safetyservice/            # Safety Event Bus
│   ├── telemetryservice/         # MQTT Client
│   ├── vehicleconnection/        # Vehicle WebSocket Handler
│   ├── webrtcsfu/                # WebRTC SFU Logik
│   └── recording/                # Session Recording Interface + MemoryRecorder
│
├── pkg/                          # Shared Go-Pakete (projektübergreifend nutzbar)
│   ├── ulid/                     # ULID-Wrapper
│   ├── logger/                   # slog-Wrapper — JSON, Event(), LOG_LEVEL ENV (ADR-017)
│   └── audit/                    # AuditWriter Interface + SQLiteAuditWriter + NoopWriter (ADR-018)
│
├── frontend/                     # React App
│   └── src/
│       ├── components/           # UI-Komponenten (VideoPanel, ControlPanel, ...)
│       ├── hooks/                # Custom Hooks (useControls, useWebRTC, ...)
│       └── lib/                  # ws-client (Protobuf ACK), api-client
│
├── infrastructure/
│   ├── compose/
│   │   └── docker-compose.yml    # Haupt-Stack (alle Services)
│   ├── docker/                   # Dockerfiles je Service
│   ├── coturn/                   # STUN/TURN Konfiguration
│   └── mosquitto/                # MQTT Broker Konfiguration
│
├── tests/
│   ├── unit/                     # Safety Test Suite (19 Szenarien — CI Gate)
│   ├── integration/              # Go Integration Tests (9 Tests)
│   ├── performance/              # Go Benchmark + k6 Latenz-Tests
│   └── e2e/                      # Playwright E2E Tests
│
├── docs/
│   ├── architecture.md           # Systemarchitektur (Details)
│   ├── requirements.md           # Funktionale & nicht-funktionale Anforderungen
│   ├── vision.md                 # Projektziele und Nicht-Ziele
│   ├── implementation-plan.md    # Sprint-Übersicht, ADR-Index
│   └── adr/                      # Architecture Decision Records (ADR-001 bis ADR-018)
│
├── tasks/
│   ├── current-sprint.md         # Aktiver Sprint
│   ├── backlog.md                # Offene Tasks
│   └── done.md                   # Abgeschlossene Tasks
│
├── go.mod                        # Go-Modul: avoc (Go 1.23)
├── Makefile                      # Alle Build/Test/Run-Befehle
├── .env.example                  # Vorlage für Umgebungsvariablen
├── CLAUDE.md                     # KI-Arbeitsanweisungen
├── CONTEXT.md                    # Glossar, Domänenwissen, offene Risiken
└── DECISIONS.md                  # Live-Übersicht aller ADRs
```

---

## 7. Die 4-Layer State Machine

Das Herzstück des Safety-Modells. Der System-Zustand ist kein einziger Wert, sondern **4 orthogonale Zustandsmaschinen**, die unabhängig voneinander laufen.

### SYSTEM STATE (Safety Master — entscheidet über SAFE MODE)

```
IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED
                                         │
                                    SAFE_MODE ← CRITICAL Trigger
                                         │
                                    RECOVERING → AUTHENTICATED
```

**CRITICAL Trigger** (lösen SAFE MODE aus):
- WebSocket-Disconnect
- Safety Bus Failure
- Dead-man Switch Timeout
- Command ACK Timeout
- Auth Invalidation
- Kein aktiver Operator

### CONTROL STATE (Command-Fluss)

```
CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED (bei SAFE_MODE)
                                      → CONTROL_LOST → CONTROL_RECOVERING
```

### MEDIA STATE (Video — niemals Auslöser für SAFE MODE)

```
MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED ⇄ MEDIA_DEGRADED → MEDIA_FAILED
```

`MEDIA_FAILED` führt zu `SYSTEM DEGRADED`, nicht zu `SAFE_MODE`. Dies ist eine **formale Invariante** des Systems.

### OPERATOR STATE (Menschliche Governance)

```
NO_OPERATOR → OPERATOR_ASSIGNED → ACTIVE_OPERATOR ⇄ HANDOVER_PENDING
```

`NO_OPERATOR` führt zu SAFE MODE. Es darf maximal 1 `ACTIVE_OPERATOR` gleichzeitig geben.

---

## 8. Sicherheitskonzepte

Diese Konzepte sind fundamental für das Verständnis des Systems:

### Dead-man Switch

Der Operator muss die Steuerung aktiv halten (Spacebar gedrückt halten). Loslassen oder Ausbleiben für > Timeout-Intervall → `CRITICAL` → `SAFE_MODE`.

### Emergency Stop

Sofortige Systemdeaktivierung. Umgeht alle anderen Ebenen. Immer verfügbar. → `CRITICAL` → `SAFE_MODE`.

### SAFE MODE

Der wichtigste Systemzustand bei Problemen:

- Fahrzeug stoppt sofort (Channel Close)
- Keine Steuerbefehle mehr möglich (CONTROL_BLOCKED)
- UI wechselt in Read-Only-Modus
- Recovery Checkpoint wird gespeichert (Vehicle-ID, Operator-ID, letzter State, Safety Reason)
- **Kein automatisches Resume** — Operator muss explizit bestätigen (Operator Ack)

### Operator Ack

Explizite Bestätigung des Operators — **vor** Aktivierung UND nach Recovery aus SAFE MODE. Das System startet und resumt niemals automatisch.

### Failure-Klassifizierung

| Klasse | Beispiel | Reaktion |
|--------|----------|----------|
| **CRITICAL** | WS Disconnect, Dead-man Timeout | Channel Close → SAFE_MODE |
| **DEGRADED** | Video verloren | DEGRADED State, Warnung im UI, Steuerung möglich |
| **OBSERVATION** | Auth Service down (laufende Session) | Neue Sessions blockiert, JWT lokal weiter gültig |

---

## 9. Schlüsselkonzepte & Glossar

| Begriff | Bedeutung |
|---------|-----------|
| **Operator** | Menschlicher Nutzer, der das Fahrzeug steuert |
| **Vehicle** | Das ferngesteuerte Fahrzeug |
| **Control Session** | Die primäre operative Einheit: 1 Vehicle + 1 Active Operator + 1 Control Server |
| **GSA** | Global Session Authority — der Control Server als einziger Session-Erzeuger |
| **Control Hub** | Control Server als Rang-1-Hub (Safety Truth) |
| **Video Hub** | WebRTC SFU als Rang-2-Hub (Awareness only) |
| **SAFE MODE** | Sicherheitszustand: Fahrzeug steht, keine Commands, UI read-only |
| **Dead-man Switch** | Muss aktiv gehalten werden — Loslassen = Auto-Stop |
| **Emergency Stop** | Sofortige Deaktivierung, bypasses alle Ebenen |
| **Operator Ack** | Explizite Operator-Bestätigung (vor Aktivierung + nach Recovery) |
| **ULID** | Universally Unique Lexicographically Sortable Identifier — Format der Session-IDs |
| **CorrelationHeader** | Protobuf-Feld in allen Schemas: session_id, event_id, vehicle_id, operator_id |
| **CRITICAL Failure** | Fehler der sofort Channel Close + SAFE MODE auslöst |
| **DEGRADED State** | Teilausfall (z.B. Video) — Steuerung weiterhin möglich |
| **AuditWriter** | Interface für sichere, synchrone Persistenz von Safety-Events in SQLite |
| **Safety Event Bus** | Go In-Memory Message Queue, DDS-Interface-kompatibel |

---

## 10. Tech Stack

### Backend (Go)

| Technologie | Version | Zweck | ADR |
|-------------|---------|-------|-----|
| Go | 1.23 | Alle Backend-Services | ADR-001 |
| `gorilla/websocket` | 1.5.1 | WebSocket Transport | — |
| `golang-jwt/jwt` | v5 | JWT Auth | ADR-004 |
| `pion/webrtc` | v4 | WebRTC SFU | ADR-014 |
| `eclipse/paho.mqtt` | 1.4.3 | MQTT Client | ADR-003 |
| `oklog/ulid` | v2 | Session-ID Generierung | ADR-016 |
| `google.golang.org/protobuf` | 1.34.2 | Protobuf Runtime | ADR-008 |
| `stretchr/testify` | 1.11.1 | Test-Assertions | ADR-006 |

### Frontend (TypeScript)

| Technologie | Zweck | ADR |
|-------------|-------|-----|
| React 18 + TypeScript | SPA Framework | ADR-013 |
| Vite | Build Tool + Dev-Server | ADR-013 |
| Tailwind CSS | Styling | ADR-013 |
| Shadcn/ui | Komponenten-Bibliothek | ADR-013 |
| protoc-gen-es | TypeScript Protobuf Code-Gen | ADR-012b/013 |
| Vitest + RTL | Component Tests | ADR-006 |
| Playwright | E2E Tests | ADR-006 |

### Infrastruktur

| Technologie | Zweck |
|-------------|-------|
| Docker Compose | Container-Orchestrierung (kein Kubernetes) |
| Eclipse Mosquitto | MQTT Broker |
| coturn | STUN/TURN Server für WebRTC NAT-Traversal |
| nginx | Frontend Reverse Proxy |
| Grafana Loki | Log-Aggregation (ADR-017) — Port 3100 |
| Grafana | Log-Visualisierung, AVOC Session Dashboard (ADR-017) — Port 3001 |
| Promtail | Docker-Log-Collector → Loki (ADR-017) |

---

## 11. Protobuf & Code-Generierung

### Warum Protobuf?

Alle Kanäle (Control, Safety, Telemetry, Session-Events) nutzen Protocol Buffers als einheitliches Nachrichtenformat. Vorteile: typsicher, binär-effizient, sprachübergreifend (Go + TypeScript).

### Schema-Struktur

```
proto/
├── common.proto      → CorrelationHeader (in allen anderen Schemas eingebunden)
├── control.proto     → ControlCommand, ControlAck
├── telemetry.proto   → TelemetryEvent
├── safety.proto      → SafetyEvent, SafetyEventType
└── session.proto     → SessionEvent (für SFU), RecordingEntry
```

> WebRTC Signaling (SDP/ICE) ist bewusst **außerhalb** des Protobuf-Schemas — es nutzt den WebRTC-Standard direkt.

### Code generieren

```bash
# Go-Code generieren (gen/go/)
make proto-gen

# TypeScript-Code generieren via Docker (kein lokales protoc nötig)
make proto-gen-ts

# gen/ ist gitignored — niemals committen
```

### Versioning-Regel (ADR-012)

- Keine Field-IDs ändern
- Keine Felder entfernen
- Neue Features als optionale Felder hinzufügen
- Neue Erkenntnisse → neues ADR

---

## 12. Entwicklungs-Workflow

### Alle Make-Befehle

```bash
# Stack
make up                 # Gesamten Stack starten (Docker Compose, inkl. Build)
make down               # Stack stoppen

# Build
make build              # Alle Go-Services kompilieren → bin/
make proto-gen          # Protobuf Go + TS Code generieren (lokal)
make proto-gen-ts       # Protobuf TypeScript Code generieren (via Docker)

# Tests
make test               # Alle Go-Tests
make test-safety        # Safety Test Suite — muss 19/19 grün bleiben (CI Gate)
make test-integration   # Integration Tests (Stack startet/stoppt automatisch)
make test-latency       # Go Benchmark ACK-Roundtrip <100ms (CI Build-Fail)
make test-k6            # k6 Lasttest 10 VU / 30s

# Frontend Tests
cd frontend && npm test           # Vitest Component-Tests (31 Tests)
cd frontend && npm run test:e2e   # Playwright E2E (benötigt laufenden Stack)

# Code-Qualität
make lint               # golangci-lint

# Aufräumen
make clean              # gen/ und bin/ löschen
```

### Latenz-Grenzwert

Der Control-Loop-ACK-Roundtrip **muss unter 100ms** bleiben. Dieser Wert ist ein **CI Build-Fail** — `make test-latency` schlägt fehl wenn p99 > 100ms. Das ist eine Safety-Anforderung (ADR-006/010).

### Umgebungsvariablen

Die Datei `.env` (aus `.env.example`) enthält:

| Variable | Beschreibung |
|----------|-------------|
| `JWT_SECRET` | JWT-Signierungsschlüssel (min. 32 Zeichen, sicher wählen) |
| `LOG_LEVEL` | Log-Level: `debug`, `info`, `warn`, `error` (default: `info`) |

`.env` ist gitignored — niemals committen.

---

## 13. Tests ausführen

### Übersicht der Test-Suites

| Suite | Befehl | Zweck | Anzahl |
|-------|--------|-------|--------|
| Safety Unit Tests | `make test-safety` | 19 Safety-Szenarien — CI Gate | 19/19 |
| Go Integration Tests | `make test-integration` | Service-Interaktion End-to-End | 9 |
| Go Latenz-Benchmark | `make test-latency` | ACK-Roundtrip < 100ms | — |
| k6 Lasttest | `make test-k6` | 10 VU / 30s Lastprofil | — |
| Vitest Component | `cd frontend && npm test` | React Komponenten | 31 |
| Playwright E2E | `cd frontend && npm run test:e2e` | Browser-seitig, benötigt Stack | — |

### Safety Test Suite — wichtigstes CI Gate

Die 19 Safety-Tests in `tests/unit/safety_test.go` prüfen alle kritischen Safety-Szenarien (Dead-man Timeout, Emergency Stop, State Transitions, etc.). Diese müssen **immer** grün bleiben. Vor jedem Commit der Safety-Logik berührt: `make test-safety`.

### Test-Stack

Integration- und Performance-Tests nutzen einen separaten Stack auf Ports 18080–18082 (kein Konflikt mit dem Dev-Stack):

```bash
# Manuell starten (falls automatischer Start Probleme macht)
docker compose -f tests/docker-compose.test.yml up --build -d
```

---

## 14. Beitragen zum Projekt

### Workflow-Prozess (aus CLAUDE.md)

Jede Änderung durchläuft einen definierten Prozess je nach Komplexität:

| Typ | Beispiel | Prozess |
|-----|---------|---------|
| **S** (< 30 Min) | Typo-Fix, eindeutiger Bugfix | Direkt implementieren + Test |
| **M** (30–180 Min) | Feature, Refactoring, neue Komponente | Analyse → Architektur → Task → Implement → Test → Doku |
| **L** (Architektur) | Neue Technologie, Systemänderung | Grill-Me + ADR zwingend vor Umsetzung |

Tasks laufen im System: `tasks/backlog.md` → `tasks/current-sprint.md` → `tasks/done.md`

### Neuen Go-Service hinzufügen

1. `cmd/<service-name>/main.go` mit `/health` Endpoint erstellen
2. `infrastructure/docker/go-service.Dockerfile` wiederverwenden (via `SERVICE_NAME` ARG)
3. Service in `infrastructure/compose/docker-compose.yml` + `tests/docker-compose.test.yml` eintragen
4. `logger.New("<service-name>")` aus `pkg/logger` für strukturiertes JSON-Logging verwenden

### Neuen Frontend-Component erstellen

1. `frontend/src/components/<Name>.tsx` — Komponente
2. `frontend/src/hooks/use<Name>.ts` — Custom Hook
3. `frontend/src/components/<Name>.test.tsx` — Vitest + RTL Test
4. Externe Dependencies mocken: `vi.mock('@/hooks/use...')`

### Proto-Schema ändern

```bash
# 1. proto/*.proto bearbeiten (Field-Versioning beachten: keine IDs ändern!)
# 2. Code neu generieren:
make proto-gen        # Go → gen/go/
make proto-gen-ts     # TypeScript → frontend/src/gen/
# gen/ niemals committen
```

### Safety-kritischen Code ändern

Dateien wie `detector.go`, `statemachine.go`, `websocket.go` erfordern besondere Sorgfalt:

1. `make test-safety` **vor** und **nach** der Änderung ausführen — 19/19 muss grün bleiben
2. Safety-Tests bei Bedarf erweitern
3. SAFE_MODE-Transitionen: erst `AuditWriter.WriteSync()` (ADR-018), dann Transition

### Neues ADR erstellen

1. `docs/adr/000-template.md` kopieren → `docs/adr/0XX-titel.md`
2. Alle Pflichtfelder ausfüllen (Kontext, Optionen, Entscheidung, Konsequenzen)
3. ADR in `DECISIONS.MD` und `docs/adr/README.md` eintragen
4. `docs/implementation-plan.md` ADR-Index aktualisieren

---

## 15. ADRs — Architekturentscheidungen

Alle wichtigen Architekturentscheidungen sind als **Architecture Decision Records (ADRs)** dokumentiert. ADRs sind **unveränderlich** — neue Erkenntnisse führen zu einem neuen ADR, nicht zur Änderung eines bestehenden.

### Übersicht der 18 ADRs

| ADR | Titel | Kern-Entscheidung |
|-----|-------|-------------------|
| ADR-001 | Backend Language | Go für alle Backend-Services |
| ADR-002 | Safety Channel | Go In-Memory Bus, DDS-Interface-kompatibel |
| ADR-003 | MQTT Broker | Eclipse Mosquitto |
| ADR-004 | Authentication | Separater Auth Service, JWT (Operator + Vehicle) |
| ADR-005 | Session Recording | Abstraktes Interface (Storage noch offen) |
| ADR-006 | Testing Strategy | testing + testify, k6, CI Safety Gate 19/19, <100ms |
| ADR-007 | System Runtime Topology | 2 Hubs (Control > Video), strikte Hierarchie |
| ADR-008 | Message Protocol | Protobuf als Application Bus |
| ADR-009 | Failure Model | CRITICAL / DEGRADED / OBSERVATION Klassifizierung |
| ADR-010 | Control Loop | Event-driven Stream, sync ACK, Channel Close bei CRITICAL |
| ADR-011 | System State Machine | 4-Layer (SYSTEM / CONTROL / MEDIA / OPERATOR) |
| ADR-012 | Message Flow Runtime | Field-based Protobuf Versioning |
| ADR-012b | Sync/Async Model | Welche Kanäle sync vs. async |
| ADR-013 | Frontend Tech Stack | React 18 + TypeScript + Vite + Tailwind + protoc-gen-es |
| ADR-014 | Video Streaming | WebRTC SFU (Pion/Go) + coturn |
| ADR-015 | Session Coordinator | Control Server als GSA |
| ADR-016 | Session Correlation ID | ULID als systemweite Korrelations-ID |
| ADR-017 | Logging Strategy | Hybrid: async slog → Loki + sync AuditWriter |
| ADR-018 | Audit Trail Strategy | SQLite WAL für garantierte Safety-Event-Persistenz |

Vollständige ADRs: [docs/adr/](docs/adr/) | Live-Übersicht: [DECISIONS.MD](DECISIONS.MD)

---

## 16. Troubleshooting

### 502 Bad Gateway nach Container-Rebuild

nginx cached Docker-IPs beim Start.

```bash
docker exec avoc-frontend-1 nginx -s reload
```

### `npm run dev` schlägt fehl: `Cannot find @/gen/control_pb.js`

Protobuf TypeScript-Dateien fehlen (gitignored).

```bash
make proto-gen-ts
```

### `npm run dev` schlägt fehl: `Cannot find @rollup/rollup-linux-x64-gnu`

`node_modules` wurde in Docker (Alpine/musl) als root installiert, ist inkompatibel mit dem Host.

```bash
docker run --rm \
  -v $(PWD)/frontend:/app -w /app node:22-alpine \
  sh -c 'rm -rf node_modules package-lock.json'
cd frontend && npm install
```

### Port-Konflikte beim Stack-Start

```bash
lsof -i :3000   # Frontend (nginx)
lsof -i :8080   # Control Server
lsof -i :8081   # Auth Service
lsof -i :8084   # WebRTC SFU
```

Test-Stack läuft auf Ports 18080–18082 — kein Konflikt mit dem Dev-Stack.

### WSL2: Services nicht erreichbar über `localhost`

WSL2 hat eine eigene IP-Adresse. `.env` und `frontend/vite.config.ts` anpassen:

```bash
hostname -I | awk '{print $1}'   # WSL2-IP ermitteln
```

### `make test-integration` schlägt fehl: Services nicht erreichbar

```bash
# Manuell mit mehr Wartezeit
docker compose -f tests/docker-compose.test.yml up --build -d
sleep 15
go test ./tests/integration/... -v -timeout 120s
docker compose -f tests/docker-compose.test.yml down
```

---

## 17. Weiterführende Dokumentation

| Dokument | Inhalt |
|----------|--------|
| [docs/architecture.md](docs/architecture.md) | Vollständige Systemarchitektur mit allen Details |
| [docs/requirements.md](docs/requirements.md) | Funktionale & nicht-funktionale Anforderungen |
| [docs/vision.md](docs/vision.md) | Projektziele, Nicht-Ziele, Erfolgskriterien |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Sprint-Übersicht, vollständiger ADR-Index |
| [docs/adr/](docs/adr/) | Alle Architecture Decision Records |
| [DECISIONS.MD](DECISIONS.MD) | Live-Übersicht aller Entscheidungen |
| [CONTEXT.MD](CONTEXT.MD) | Vollständiges Glossar, Domänenwissen, Risiken |
| [tasks/current-sprint.md](tasks/current-sprint.md) | Aktiver Sprint + offene Tasks |
| [tasks/backlog.md](tasks/backlog.md) | Backlog aller geplanten Aufgaben |
| [CLAUDE.MD](CLAUDE.MD) | KI-Arbeitsregeln (Prozess, ADR-Pflicht, Grill-Me) |

---

## 18. Wichtige Codebeispiele

Dieser Abschnitt zeigt die zentralen Muster und Mechanismen anhand des echten Quellcodes. Ziel: Ein neuer Entwickler versteht, wie die Kernkomponenten zusammenspielen.

---

### 18.1 State Machine — Transitionen und Validierung

**Datei:** [internal/controlserver/statemachine/state.go](internal/controlserver/statemachine/state.go)

Die State Machine ist der Safety-Kern des Systems. Transitionen sind durch eine Tabelle explizit erlaubt oder verboten — ungültige Versuche werden still verworfen.

```go
// Erlaubte Übergänge — jede nicht gelistete Kombination wird abgelehnt.
var validSystemTransitions = map[SystemState][]SystemState{
    StateIdle:          {StateConnecting},
    StateConnecting:    {StateAuthenticated, StateSafeMode},
    StateAuthenticated: {StateConnected, StateSafeMode},
    StateConnected:     {StateDegraded, StateSafeMode},
    StateDegraded:      {StateConnected, StateSafeMode},
    StateSafeMode:      {StateRecovering},
    StateRecovering:    {StateAuthenticated, StateSafeMode},
}

// TransitionSystem setzt den SYSTEM STATE und erzwingt abhängige CONTROL STATE-Regeln.
// Ungültige Transitionen werden geloggt und verworfen — kein Panic, kein Crash.
func (m *Machine) TransitionSystem(next SystemState) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if !isValidTransition(m.System, next) {
        log.Printf("[STATE] invalid transition rejected: %s → %s", m.System, next)
        return  // System bleibt im aktuellen Zustand
    }

    m.System = next
    switch next {
    case StateSafeMode:
        m.Control = ControlBlocked  // SAFE_MODE erzwingt CONTROL_BLOCKED
    case StateConnected:
        m.Control = ControlActive
    }
}
```

**Wichtige Invariante für Media:** Video-Fehler dürfen niemals SAFE_MODE auslösen (ADR-009 Invariant 1):

```go
// TransitionMedia: MEDIA_FAILED → SYSTEM DEGRADED (niemals SAFE_MODE)
func (m *Machine) TransitionMedia(next MediaState) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Media = next
    if (next == MediaFailed || next == MediaDegraded) && m.System == StateConnected {
        // Absichtlich nur DEGRADED — SAFE_MODE ist hier formal verboten
        m.System = StateDegraded
    }
}
```

**Neuer Zustand aus einem bestehenden lesen (Thread-safe):**

```go
sm := statemachine.New()
sysState, ctrlState, mediaState, opState := sm.Get()
```

---

### 18.2 Safety Watchdogs — DeadmanWatchdog & ACKTimeoutWatcher

**Datei:** [internal/controlserver/safety/detector.go](internal/controlserver/safety/detector.go)

#### DeadmanWatchdog

Der Watchdog läuft erst an, wenn der Operator den ersten `DEADMAN_HOLD` schickt (*Armed Pattern*). So verhindert er einen False Positive direkt nach dem Verbindungsaufbau.

```go
// Start: registriert die Session, startet den Timer NICHT.
// Der erste Reset()-Call bewaffnet den Watchdog.
func (w *DeadmanWatchdog) Start(sessionID, vehicleID string) { ... }

// Reset: beim ersten Aufruf → bewaffnet den Timer.
// Bei jedem weiteren Aufruf → setzt den Countdown zurück.
// Operator muss alle 1,5s einen DEADMAN_HOLD senden (Frontend-Intervall).
func (w *DeadmanWatchdog) Reset() {
    if !w.armed {
        w.armed = true
        w.timer = time.AfterFunc(w.timeout, w.fire)
        return
    }
    w.timer.Reset(w.timeout)
}

// fire: wird vom Timer aufgerufen wenn kein Reset() kam → CRITICAL
func (w *DeadmanWatchdog) fire() {
    w.sm.TransitionSystem(statemachine.StateSafeMode)
    w.publisher.PublishEvent(safetyservice.SafetyEvent{
        Type:   safetyservice.EventDeadmanTimeout,
        Reason: "dead-man switch timeout — operator released hold",
    })
}
```

#### ACKTimeoutWatcher

Jeder eingehende Command startet einen Timer. Kommt kein ACK innerhalb von 100ms → CRITICAL:

```go
// CommandReceived: startet den Per-Command-ACK-Timer
func (w *ACKTimeoutWatcher) CommandReceived(sessionID, vehicleID string) {
    w.pendingTimer = time.AfterFunc(w.timeout, w.fire)
}

// CommandACKed: bricht den Timer ab (happy path)
func (w *ACKTimeoutWatcher) CommandACKed() {
    if w.pendingTimer != nil {
        w.pendingTimer.Stop()
    }
}
```

Im Transport-Layer wird das Muster so verwendet:

```go
// websocket.go readLoop — Wrappt jeden Command mit ACK-Tracking
h.ackWatcher.CommandReceived(sess.ID, sess.VehicleID)
ackBytes, err = h.engine.Handle(msg, sess)
conn.WriteMessage(websocket.BinaryMessage, ackBytes)
h.ackWatcher.CommandACKed()  // Timer abbrechen — alles gut
```

---

### 18.3 Command Engine — Protobuf Routing

**Datei:** [internal/controlserver/command/engine.go](internal/controlserver/command/engine.go)

Der Command Engine parst eingehende Protobuf-Bytes, routet nach `CommandType` und antwortet mit einem `ControlAck`.

```go
func (e *Engine) Handle(rawMsg []byte, sess session.Session) ([]byte, error) {
    cmd := &controlv1.ControlCommand{}
    if err := proto.Unmarshal(rawMsg, cmd); err != nil {
        return e.ack(sess, "", false, "invalid protobuf message")
    }

    if !e.limiter.allow() {
        return e.ack(sess, cmd.Header.EventId, false, "rate limited")
    }

    switch cmd.Type {
    case controlv1.CommandType_COMMAND_TYPE_DEADMAN_HOLD:
        e.deadman.Reset()  // Watchdog zurücksetzen

    case controlv1.CommandType_COMMAND_TYPE_DEADMAN_RELEASE:
        // Bewusst kein Reset — Watchdog läuft ab → SAFE_MODE

    case controlv1.CommandType_COMMAND_TYPE_EMERGENCY_STOP:
        e.sm.TransitionSystem(statemachine.StateSafeMode)
        e.safetyPub.PublishEvent(safetyservice.SafetyEvent{
            Type:   safetyservice.EventEmergencyStop,
            Reason: "operator EMERGENCY_STOP command",
        })

    case controlv1.CommandType_COMMAND_TYPE_STEER,
         controlv1.CommandType_COMMAND_TYPE_THROTTLE,
         controlv1.CommandType_COMMAND_TYPE_BRAKE:
        // Weiterleitung an Fahrzeug (Vehicle Connection Layer)
    }

    return e.ack(sess, cmd.Header.EventId, true, "")
}
```

Der ACK enthält immer den vollständigen `CorrelationHeader` — so kann das Frontend die Antwort der richtigen Session und dem richtigen Event zuordnen:

```go
func (e *Engine) ack(sess session.Session, eventID string, success bool, errMsg string) ([]byte, error) {
    ack := &controlv1.ControlAck{
        Header: &commonv1.CorrelationHeader{
            SessionId:  sess.ID,
            EventId:    eventID,   // Echo des Command-EventIDs
            VehicleId:  sess.VehicleID,
            OperatorId: sess.OperatorID,
            Timestamp:  time.Now().UnixMilli(),
        },
        Success:  success,
        ErrorMsg: errMsg,
    }
    return proto.Marshal(ack)
}
```

Das Rate-Limiting (100 Commands/s) läuft über einen einfachen Token-Bucket ohne externe Dependency:

```go
// tokenBucket: erlaubt maxCommandsPerSecond = 100 Commands/s
func (b *tokenBucket) allow() bool {
    b.tokens += elapsed.Seconds() * b.rate  // Tokens nachfüllen
    if b.tokens < 1.0 { return false }      // Budget erschöpft
    b.tokens--
    return true
}
```

---

### 18.4 WebSocket Transport — JWT Handshake & SAFE_MODE Handling

**Datei:** [internal/controlserver/transport/websocket.go](internal/controlserver/transport/websocket.go)

Der JWT-Token wird im WebSocket-Handshake validiert — entweder als `Authorization: Bearer <token>` Header oder als `?token=` Query-Parameter:

```go
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
    tokenStr := extractToken(r)
    claims, err := h.validateJWT(tokenStr)
    if err != nil {
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }

    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()

    // Recovery-Pfad: SAFE_MODE → RECOVERING → AUTHENTICATED
    // Normal-Pfad:   IDLE → CONNECTING → AUTHENTICATED
    if current, _, _, _ := h.sm.Get(); current == statemachine.StateSafeMode {
        h.sm.TransitionSystem(statemachine.StateRecovering)
    } else {
        h.sm.TransitionSystem(statemachine.StateConnecting)
    }
    h.sm.TransitionSystem(statemachine.StateAuthenticated)

    go h.heartbeat(conn)
    h.readLoop(conn, claims)
}
```

Im ReadLoop: Commands in SAFE_MODE werden still verworfen, Disconnect → SAFE_MODE + Recovery Checkpoint:

```go
func (h *WSHandler) readLoop(conn *websocket.Conn, claims *Claims) {
    defer func() {
        h.deadman.Stop()
        sysState, _, _, _ := h.sm.Get()
        if sysState != statemachine.StateSafeMode {
            // WS-Disconnect → CRITICAL → SAFE_MODE (ADR-009/010)
            h.sm.TransitionSystem(statemachine.StateSafeMode)
        }
        // Recovery Checkpoint speichern
        h.sessionMgr.SaveCheckpoint(...)
        h.sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
    }()

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil { return }  // Verbindung geschlossen → defer greift

        sysState, ctrlState, _, _ := h.sm.Get()
        if sysState == statemachine.StateSafeMode || ctrlState == statemachine.ControlBlocked {
            continue  // Commands in SAFE_MODE still verwerfen (ADR-011)
        }
        // ... Command verarbeiten
    }
}
```

---

### 18.5 Session Manager (GSA) — Session-Lifecycle

**Datei:** [internal/controlserver/session/manager.go](internal/controlserver/session/manager.go)

Der Session Manager ist der einzige Ort im System, der Sessions erzeugt. Die Session-ID ist ein ULID und überlebt SAFE_MODE:

```go
// CreateSession: erzeugt eine neue Control Session (1 Vehicle + 1 Operator)
// ULID als Root Anchor — zeitlich sortierbar, URL-safe (ADR-016)
func (m *Manager) CreateSession(vehicleID, operatorID, operatorRole string) Session {
    s := Session{
        ID:           ulid.Generate(),  // pkg/ulid — einziger ULID-Erzeuger
        VehicleID:    vehicleID,
        OperatorID:   operatorID,
        OperatorRole: operatorRole,
        CreatedAt:    time.Now(),
    }
    m.current = &s
    return s
}

// SaveCheckpoint: friert den Session-Zustand bei SAFE_MODE-Eintritt ein.
// Basis für RECOVERING → CONNECTED re-Aktivierung ohne Auto-Resume.
func (m *Manager) SaveCheckpoint(sysState, ctrlState, safetyReason string) {
    m.checkpoint = &RecoveryCheckpoint{
        SessionID:        m.current.ID,
        LastSystemState:  sysState,
        LastControlState: ctrlState,
        SafetyReason:     safetyReason,  // z.B. "WS_DISCONNECT" oder "DEADMAN_TIMEOUT"
        CheckpointTS:     time.Now(),
    }
}

// PushSFUEvent: sendet Session-Events asynchron an den Video Hub (ADR-015).
// Der SFU konsumiert, interpretiert aber niemals — er ist ein Dumb Media Router.
func (m *Manager) PushSFUEvent(eventType string) {
    go m.sfuPublisher.PublishSessionEvent(eventType, s.ID, s.OperatorID)
}
```

---

### 18.6 Protobuf Schema — ControlCommand & CorrelationHeader

**Datei:** [proto/control.proto](proto/control.proto)

```protobuf
// Alle Commands tragen einen CorrelationHeader (aus common.proto).
// session_id + event_id + vehicle_id + operator_id + timestamp
// → vollständige Rekonstruierbarkeit über alle Kanäle (ADR-016)
message ControlCommand {
  avoc.common.v1.CorrelationHeader header = 1;
  CommandType                       type  = 2;
  float                             value = 3;  // normalisiert [-1.0, 1.0]
}

// ControlAck ist die synchrone Antwort vom Server (<100ms Ziel).
// Enthält denselben CorrelationHeader → Frontend kann ACK der Command zuordnen.
message ControlAck {
  avoc.common.v1.CorrelationHeader header    = 1;
  bool                              success   = 2;
  string                            error_msg = 3;
}

enum CommandType {
  COMMAND_TYPE_UNSPECIFIED     = 0;
  COMMAND_TYPE_STEER           = 1;  // value: [-1.0 links, 1.0 rechts]
  COMMAND_TYPE_THROTTLE        = 2;  // value: [-1.0 rückwärts, 1.0 vorwärts]
  COMMAND_TYPE_BRAKE           = 3;  // value: [0.0, 1.0]
  COMMAND_TYPE_SPEED           = 4;
  COMMAND_TYPE_EMERGENCY_STOP  = 5;
  COMMAND_TYPE_DEADMAN_HOLD    = 6;
  COMMAND_TYPE_DEADMAN_RELEASE = 7;
}
```

**Versioning-Regel:** Field-IDs (1, 2, 3, ...) dürfen **niemals** geändert werden. Neue Felder werden als optional hinzugefügt. Felder entfernen → neues ADR.

---

### 18.7 Frontend: WSClient — Verbindung & ACK-Latenz

**Datei:** [frontend/src/lib/ws-client.ts](frontend/src/lib/ws-client.ts)

Der WSClient misst die Roundtrip-Latenz pro Command automatisch:

```typescript
export class WSClient {
  private pendingAckTs = 0  // Timestamp des gesendeten Commands

  connect(token: string): void {
    // Token als Query-Parameter (WebSocket unterstützt keine Custom-Header im Browser)
    const url = `${proto}://${window.location.host}/ws?token=${token}`
    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'  // Protobuf = binär

    this.ws.onmessage = (e) => {
      const latency = this.pendingAckTs > 0 ? Date.now() - this.pendingAckTs : 0
      this.pendingAckTs = 0
      if (latency > 0) this.onAck?.(latency)  // Callback mit gemessener Latenz

      // Protobuf ControlAck parsen — error_msg aus Server surfacen
      const ack = fromBinary(ControlAckSchema, new Uint8Array(e.data))
      if (!ack.success && ack.errorMsg) this.onAckError?.(ack.errorMsg)
    }

    this.ws.onclose = () => this.onClose?.()  // → SAFE_MODE im Frontend
  }

  send(bytes: Uint8Array): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.pendingAckTs = Date.now()  // Startzeitpunkt für Latenz-Messung
    this.ws.send(bytes)
  }
}
```

---

### 18.8 Frontend: useDeadmanSwitch — Server-Watchdog am Leben erhalten

**Datei:** [frontend/src/hooks/useDeadmanSwitch.ts](frontend/src/hooks/useDeadmanSwitch.ts)

Der Hook sendet alle 1,5s ein `DEADMAN_HOLD` Protobuf-Command solange der Operator die Leertaste (oder den Button) hält. Der Server-seitige Watchdog läuft auf 10s Timeout — loslassen → Timeout → SAFE_MODE.

```typescript
const DEADMAN_INTERVAL_MS = 1500  // Alle 1,5s senden (Server-Timeout: 10s)

export function useDeadmanSwitch(wsClient, sessionId, vehicleId, operatorId, enabled) {
  const [isActive, setIsActive] = useState(false)

  const sendHold = useCallback(() => {
    const header = create(CorrelationHeaderSchema, {
      sessionId, eventId: generateULID(),
      vehicleId, operatorId,
      timestamp: BigInt(Date.now()),
    })
    const cmd = create(ControlCommandSchema, {
      header,
      type: CommandType.DEADMAN_HOLD,  // protoc-gen-es v2: Kurzname (kein Präfix)
      value: 1.0,
    })
    wsClient.send(toBinary(ControlCommandSchema, cmd))
  }, [wsClient, sessionId, vehicleId, operatorId])

  // Interval läuft nur solange gehalten
  useEffect(() => {
    if (!isActive || !enabled) return
    sendHold()                                      // Sofort beim Aktivieren senden
    const id = setInterval(sendHold, DEADMAN_INTERVAL_MS)
    return () => clearInterval(id)
  }, [isActive, enabled, sendHold])

  // Keyboard: Spacebar
  useEffect(() => {
    const onKeyDown = (e) => { if (e.code === 'Space' && !e.repeat) activate() }
    const onKeyUp   = (e) => { if (e.code === 'Space') deactivate() }
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    return () => { /* cleanup */ }
  }, [activate, deactivate])

  return {
    isActive,
    buttonProps: { onMouseDown: activate, onMouseUp: deactivate, onMouseLeave: deactivate },
  }
}
```

---

### 18.9 Frontend: useControls — 20 Hz Command Loop mit Prioritäten

**Datei:** [frontend/src/hooks/useControls.ts](frontend/src/hooks/useControls.ts)

Der Control-Loop läuft mit 20 Hz (alle 50ms). Eingabequellen haben eine feste Priorität: **Gamepad > Joystick > Keyboard**.

```typescript
const INTERVAL_MS = 50  // 20 Hz

useEffect(() => {
  if (!enabled) return
  const id = setInterval(() => {
    const s = speedRef.current  // Speed Multiplier [0.0, 1.0]

    // Priorität 1: Gamepad (wenn verbunden)
    const gp = Array.from(navigator.getGamepads()).find(g => g?.connected)
    if (gp) {
      const steerVal    = Math.abs(gp.axes[0]) > 0.1 ? gp.axes[0] * s : 0  // Deadzone 0.1
      const throttleVal = Math.abs(gp.axes[1]) > 0.1 ? -gp.axes[1] * s : 0
      const brakeVal    = gp.buttons[6]?.value ?? 0  // L2-Trigger
      if (steerVal) sendCmd(CommandType.STEER, steerVal)
      if (throttleVal) sendCmd(CommandType.THROTTLE, throttleVal)
      setActiveMode('gamepad')
      return  // Gamepad hat Vorrang — Keyboard/Joystick werden ignoriert
    }

    // Priorität 2: Virtueller Joystick (Touch/Maus-Drag)
    if (joyActiveRef.current) {
      const { x, y } = joyPosRef.current  // normalisiert [-1, 1]
      sendCmd(CommandType.STEER, x * s)
      sendCmd(CommandType.THROTTLE, y * s)
      setActiveMode('joystick')
      return
    }

    // Priorität 3: Keyboard (WASD oder Pfeiltasten)
    const keys = heldKeys.current  // Set<string> mit gedrückten Key-Codes
    let sv = 0, tv = 0
    if (keys.has('KeyA') || keys.has('ArrowLeft'))  sv -= 1
    if (keys.has('KeyD') || keys.has('ArrowRight')) sv += 1
    if (keys.has('KeyW') || keys.has('ArrowUp'))    tv += 1
    if (keys.has('KeyS') || keys.has('ArrowDown'))  tv -= 1
    if (sv || tv) {
      sendCmd(CommandType.STEER, sv * s)
      sendCmd(CommandType.THROTTLE, tv * s)
      setActiveMode('keyboard')
    }
  }, INTERVAL_MS)
  return () => clearInterval(id)
}, [enabled, sendCmd])
```

Wenn `enabled = false` (z.B. SAFE_MODE), wird der Interval sofort gestoppt und alle gedrückten Keys werden gecleart:

```typescript
useEffect(() => {
  if (!enabled) {
    heldKeys.current.clear()
    joyActiveRef.current = false
    setJoyPos({ x: 0, y: 0 })
  }
}, [enabled])
```

---

### 18.10 Safety Tests — Muster & Aufbau

**Datei:** [tests/unit/safety_test.go](tests/unit/safety_test.go)

Alle 19 Safety-Tests folgen demselben Aufbaumuster: State Machine + Mock-Publisher + Session Manager.

```go
// Gemeinsames Setup für alle Safety-Tests
func newTestSetup(t *testing.T) (sm *statemachine.Machine, pub *mocks.MockSafetyPublisher, ...) {
    sm = statemachine.New()
    pub = &mocks.MockSafetyPublisher{}
    sfuPub = &mocks.MockSFUPublisher{}
    mgr = session.NewManager(sfuPub)
    return
}

// Hilfsfunktion: bringt die State Machine in den CONNECTED-Zustand
func connectSession(t *testing.T, sm *statemachine.Machine, mgr *session.Manager) session.Session {
    sm.TransitionSystem(statemachine.StateConnecting)
    sm.TransitionSystem(statemachine.StateAuthenticated)
    ok := sm.TransitionToConnected()
    require.True(t, ok)
    return mgr.CreateSession("vehicle-1", "operator-1", "ACTIVE_OPERATOR")
}

// Beispiel: Test für Dead-man Timeout → SAFE_MODE
func TestSafety_DeadmanTimeout_TriggersSafeMode(t *testing.T) {
    sm, pub, _, mgr := newTestSetup(t)
    sess := connectSession(t, sm, mgr)

    deadman := csafety.NewDeadmanWatchdog(50*time.Millisecond, sm, pub)
    deadman.Start(sess.ID, sess.VehicleID)
    deadman.Reset()  // Arming: Watchdog startet jetzt den Timer

    time.Sleep(100 * time.Millisecond)  // Timeout abwarten

    sys, ctrl, _, _ := sm.Get()
    assert.Equal(t, statemachine.StateSafeMode, sys)
    assert.Equal(t, statemachine.ControlBlocked, ctrl)
    assert.Equal(t, safetyservice.EventDeadmanTimeout, pub.LastEvent.Type)
}

// Beispiel: Ungültige Transition muss abgelehnt werden
func TestSafety_InvalidTransitionRejected(t *testing.T) {
    sm, _, _, _ := newTestSetup(t)

    sm.TransitionSystem(statemachine.StateConnected)  // IDLE → CONNECTED: ungültig

    sys, _, _, _ := sm.Get()
    assert.Equal(t, statemachine.StateIdle, sys)  // muss IDLE geblieben sein
}
```

**Neuen Safety-Test schreiben:** `newTestSetup` + `connectSession` + gewünschten Trigger auslösen + `sm.Get()` prüfen. Safety-Tests müssen immer 19/19 grün bleiben: `make test-safety`.

---

*Stand: Sprint 7 abgeschlossen ✅ | Alle 7 Sprints · 42 Tasks · 18 ADRs abgeschlossen (2026-06-04)*
