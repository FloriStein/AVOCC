# Teleoperation System Architecture

Stand: 2026-06-14 (aktualisiert nach ADR-001 bis ADR-022, Sprint 14)

---

## Overview

Das Teleoperation System besteht aus **zwei orthogonalen Hubs** und **vier Kommunikationskanälen**:

### Hub-Hierarchie (ADR-007)

```
CONTROL HUB (Rang 1 — Safety Truth)       VIDEO HUB (Rang 2 — Awareness only)
Control Server (Go)                        MediaMTX (WHIP/WHEP Router — ADR-020)
  · State Machine (4 Layer)                 · WHIP Ingestion (Larix Broadcaster, 5G)
  · Safety Decision Engine                  · WHEP Distribution (Operator Browser)
  · Session Manager (GSA)                   · Auth-Hook → Control Server (einzige Auth-Instanz)
  · Failure Detection                        · SAFE_MODE-Kick via Management API (:9997)
  · Operator Handover
  · MediaMTX Auth-Hook (POST /internal/media/auth)
  · MediaMTX SAFE_MODE-Kick (KickVehicle)

WebRTC SFU (Pion/Go) — passiv: Session-Event-Subscriber, kein Media-Routing (ADR-020)
```

**Invariante:** `CONTROL HUB > VIDEO HUB`. Video Hub darf System State nie beeinflussen außer via DEGRADED-Annotation.

### Vier Kommunikationskanäle

1. Control Channel (WebSocket, Protobuf)
2. Video Channel (WHIP/WHEP via MediaMTX + coturn für ICE)
3. Telemetry Channel (MQTT / Mosquitto)
4. Safety Channel (Safety Event Bus, Go In-Memory)

---

## Communication Architecture

### WebSocket Layer (Control Channel)

- WSS secured connection (ADR-004)
- JWT authentication im Handshake (Operator + Vehicle)
- Protobuf-Messages (ADR-008), CorrelationHeader in jeder Message (ADR-016)
- Event-driven Stream, synchroner ACK-based Loop (ADR-010/012b)
- Heartbeat alle 30 Sekunden
- Auto-reconnect mit exponential backoff
- Channel Close bei CRITICAL Safety Event (ADR-010)

### MQTT Layer (Telemetry Channel)

- Vehicle Telemetry + Status Updates
- Eclipse Mosquitto Broker (ADR-003)
- Protobuf-Messages (ADR-008), CorrelationHeader trägt Session-ID (ADR-016)
- Asynchron, fire-and-forget (ADR-012b)

### Safety Event Bus Layer (Safety Channel — ADR-002)

- Go-Service, In-Memory Message Queue
- DDS-Interface-kompatibel (späterer Austausch per ADR)
- Protobuf-Messages (ADR-008), CorrelationHeader trägt Session-ID (ADR-016)
- Asynchron, fire-and-forget (ADR-012b)
- Safety Events: `PublishSafetyEvent`, `EmergencyStop`, `GetSafetyState`

### WHIP/WHEP Layer (Video Channel — ADR-020)

- **MediaMTX** als WHIP/WHEP Router (ersetzt Pion SFU als Media-Layer)
- **WHIP** (WebRTC-HTTP Ingestion Protocol): Larix Broadcaster (Smartphone, 5G) → MediaMTX Port 8889
- **WHEP** (WebRTC-HTTP Egress Protocol): Browser → nginx `/whep/` Proxy → MediaMTX Port 8889
- **ICE/NAT Traversal**: STUN + TURN via coturn (TURN_USER/TURN_PASSWORD aus SSM)
- **Auth**: Einziger Mechanismus — `externalAuthenticationURL` → `POST /internal/media/auth` (Control Server)
  - Publish (WHIP): Bearer Token == WHIP_STREAM_KEY → 200 / 401
  - Read (WHEP): JWT + aktive Operator-Session → 200 / 401
- **SAFE_MODE**: Control Server ruft `GET /v3/webrtcsessions/list` → `POST kick/{id}` auf MediaMTX Management API (:9997)
- **vehicleId-Routing**: Pfad-Regex `~^vehicle-.*`; vehicleId wird dynamisch aus der Vehicle Registry bezogen (VehicleSelector, ADR-022) — `vehicle-001` ist auto-geseedet
- **ICE non-trickle**: Browser wartet vollständige ICE-Gathering (`icegatheringstatechange → complete`) vor WHEP-Request

**WebRTC SFU (Pion/Go) — passiver Session-Event-Subscriber (ADR-020):**
- Empfängt SESSION_* Events vom Control Server (ADR-015) — wie bisher
- Kein ausgehender Call zu MediaMTX oder anderen Services
- Kein Media-Routing mehr (von ADR-014 übernommen durch ADR-020)

---

## Session Architecture (ADR-015/016)

### Session-Hierarchie

```
1. Vehicle Runtime Context   (Transport: WebRTC/MQTT/WS/5G)
2. Control Session           (Safety Layer — primäre Einheit)
   └── 1 Vehicle + 1 Active Operator + 1 Control Server
3. Operator Session          (Identity Layer — JWT/Login)
```

### Global Session Authority (GSA)

Der **Control Server** ist einziger Session-Erzeuger, -Verwalter und -Zerstörer:
- Session-ID generieren (ULID, Zeitpunkt: `CONNECTING → CONNECTED`)
- Session State als Single Source of Truth führen
- Recovery Checkpoint bei SAFE_MODE speichern
- Session Events asynchron an SFU pushen

### SFU Session Events (passiv empfangen, ADR-020)

```
SESSION_CREATED       → SFU protokolliert (kein Routing-Effekt)
OPERATOR_ASSIGNED     → SFU protokolliert
OPERATOR_HANDOVER     → SFU protokolliert
SESSION_DEGRADED      → SFU protokolliert
SESSION_SAFE_MODE     → Control Server kickt MediaMTX-Sessions direkt (kein SFU-Umweg)
SESSION_ENDED         → SFU protokolliert
```

MediaMTX übernimmt alle Media-Routing-Verantwortlichkeiten. Der SFU hat keine ausgehenden Calls.

### Session Correlation ID (ADR-016)

```
Format:    ULID (zeitlich sortierbar, URL-safe, distributed-safe)
Erzeuger:  Control Server (Session Manager)
Zeitpunkt: CONNECTING → CONNECTED

Hierarchie:
  Vehicle-ID
    └── Session-ID (ULID — Root Anchor, überlebt SAFE_MODE)
          └── Event-ID (ULID — pro Message/Command/Frame)

CorrelationHeader (in allen .proto Schemas):
  session_id, event_id, vehicle_id, operator_id, timestamp

JWT = Identity (Wer bist du?) ≠ Session-ID (In welchem Kontext?)
```

---

## State Machine Architecture (ADR-011)

4-Layer Model — kein monolithischer State, 4 orthogonale Maschinen:

```
OPERATOR STATE (Human Governance)
  NO_OPERATOR → ASSIGNED → ACTIVE ⇄ HANDOVER_PENDING
  NO_OPERATOR → SYSTEM SAFE_MODE

SYSTEM STATE (Safety Truth — Master)
  IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED
  CONNECTED/DEGRADED → SAFE_MODE → RECOVERING → AUTHENTICATED

CONTROL STATE (Command Flow, abhängig von SYSTEM STATE)
  CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED → CONTROL_LOST → CONTROL_RECOVERING
  SAFE_MODE ⇒ CONTROL_BLOCKED

MEDIA STATE (WebRTC, unabhängig — beeinflusst nur DEGRADED)
  MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED → MEDIA_DEGRADED → MEDIA_FAILED
  MEDIA_FAILED → SYSTEM DEGRADED (niemals SAFE_MODE)
```

---

## Safety Architecture (ADR-009/010/011)

- Dead-man Switch überschreibt alle Inputs → CRITICAL → SAFE_MODE
- Emergency Stop bypasses alle Ebenen → CRITICAL → SAFE_MODE
- Command ACK Timeout → CRITICAL → SAFE_MODE
- Auto-Stop bei Disconnect → Channel Close (ADR-010)
- Session Recording für Auditierbarkeit (abstraktes Interface — ADR-005)

**Formale Invarianten:**
```
INVARIANT 1: Media Layer SHALL NOT influence SAFE_MODE transitions
             except via DEGRADED annotation evaluated by the Control Hub.

INVARIANT 2: SAFE_MODE transitions are exclusively triggered by
             Control, Safety Bus, or Operator-level failures.

INVARIANT 3: Control Hub is Single Source of Truth for Session State.
             Conflicting states resolve in favor of Control Hub.
```

---

## REST API — Authentifizierung (Sprint 14)

Alle schreibenden REST-Endpoints sind durch `requireJWT`-Middleware geschützt (Bearer-Token-Prüfung via `github.com/golang-jwt/jwt/v5`):

**Geschützt (11 Endpoints):** `POST /session/start`, `POST /session/end`, `POST /handover/request`, `POST /handover/confirm`, `POST /handover/cancel`, `POST /media/event`, `POST /emergency-stop`, `GET /audit/events`, `GET /recording/`, `POST /vehicles`, `DELETE /vehicles/{id}`

**Bewusst offen:** `GET /state`, `GET /health`, `GET /vehicles`, `GET /ice-config`, `GET /vehicle/ack/latest/{id}`, `POST /log` — Polling ohne Session-Kontext, nicht-sensitive Lesezugriffe, oder Fire-and-forget Logger (muss vor Login feuern können).

---

## Control Server — Interne Modulstruktur

Ein Service, 5 logische Module:

```
1. Transport Layer      → WebSocket, JWT Verify, Heartbeat, Channel Close
2. Command Engine       → Input Validation, Rate Limiting, Backpressure, Routing
3. State Machine Engine → SYSTEM / CONTROL / MEDIA / OPERATOR STATE
4. Safety Decision Module → CRITICAL/DEGRADED Klassifizierung, Invarianten-Enforcement
5. Session Manager (GSA) → Session-ID (ULID), Operator-Rollen, Handover, SFU Event Push
```

---

## Latency Targets

| Kanal | Ziel | Typ |
|-------|------|-----|
| Control Loop | < 100ms (ACK-Roundtrip) | Hard — CI Build-Fail |
| Video | 100–300ms QoS-Ziel | Soft — kein Safety-Hartziel |
| Safety Events | near-instant | Priority Channel |

---

## Frontend System (ADR-013)

- Framework: React 18+ + TypeScript
- Build Tool: Vite
- Styling: Tailwind CSS
- Component Library: Shadcn/ui
- Protobuf Code-Gen: protoc-gen-es (TypeScript-Klassen, build-time, gitignored)
- Rendering: SPA (Single Page Application)
- Communication:
  - WebSocket (Control, Protobuf, Sync ACK-based — ADR-012b)
  - WebRTC RTCPeerConnection (Video, browser-nativ)
  - MQTT (optional Telemetry Display)

### UI Module

| Komponente | Implementierung | Sprint |
|------------|----------------|--------|
| **Video Panel** | `VideoPanel.tsx` + `useWebRTC.ts` — RTCPeerConnection, WHEP-Protokoll via `/whep/{vehicleId}/whep`, ICE non-trickle Gathering, MEDIA STATE Badge, DEGRADED-Overlay, `onVideoLatency`-Callback (Sprint 14) | Sprint 5/9/14 ✅ |
| **Control Panel** | `ControlPanel.tsx` + `useControls.ts` — Keyboard WASD/Pfeiltasten, Virtual Joystick SVG, Gamepad API, Speed Slider, 20 Hz Protobuf Command Loop | Sprint 5 ✅ |
| **Safety Panel** | `SafetyPanel.tsx` + `useDeadmanSwitch.ts` — Emergency Stop, Dead-man Switch (Spacebar/Button), SAFE MODE Indikator; `token`-Prop (Sprint 14) | Sprint 3/14 ✅ |
| **Connection Status Panel** | `ConnectionPanel.tsx` — SYSTEM STATE, **Dual-Channel-Latenz: Control (WS-ACK-RTT) + Video (WebRTC ICE-RTT)**, Session-ID (ULID), Operator-Rolle, Speed/Battery (Telemetrie), VehicleSelector (Sprint 12) | Sprint 3/5/12/14 ✅ |
| **SAFE MODE Overlay** | `SafeModeOverlay.tsx` — Fullscreen-Block, Operator-Ack-Button für Recovery | Sprint 3 ✅ |
| **Backend-Unreachable-Banner** | `App.tsx` + `useSystemState.ts` — Rotes Banner + ControlPanel-Sperre nach 3 fehlgeschlagenen State-Polls (1,5s) | Sprint 14 ✅ |
| **Vehicle Selector** | `VehicleSelector.tsx` + `useVehicles.ts` — Dropdown mit Online-Indikator, Session-Start-Button | Sprint 12 ✅ |
| **Input Indicator Panel** | `InputIndicatorPanel.tsx` + `useVehicleAck.ts` — Lenkrad-SVG, ActuationBars, AckBadge | Sprint 11 ✅ |
| **Operator Panel** | Handover-Anfrage, Observer-Liste | nicht implementiert (kein eigener Sprint geplant) |

---

## Projekt-Verzeichnisstruktur

```
AutonomousVehicleOperationalControlCenter/
├── proto/                        # .proto Source — Single Source of Truth (ADR-008)
│   ├── common.proto              # CorrelationHeader (shared, ADR-016)
│   ├── control.proto
│   ├── safety.proto
│   ├── telemetry.proto
│   └── session.proto
├── gen/                          # Generated — gitignored, nie committen
│   ├── go/                       # protoc-gen-go output
│   └── ts/                       # protoc-gen-es output
├── cmd/                          # Go Service Entry Points
│   ├── control-server/
│   ├── auth-service/
│   ├── safety-service/
│   ├── telemetry-service/
│   └── webrtc-sfu/
├── internal/                     # Go Service-interne Pakete
│   ├── controlserver/
│   │   ├── command/              # Command Engine — Protobuf Parsing, Rate Limiting (BE-04)
│   │   ├── transport/            # WebSocket Layer
│   │   ├── statemachine/         # 4-Layer State Machine (ADR-011)
│   │   ├── safety/               # Safety Decision Module (ADR-009)
│   │   └── session/              # Session Manager / GSA (ADR-015/016)
│   ├── mediamtx/                 # MediaMTX Management API Client — KickVehicle (ADR-020)
│   ├── recording/                # Session Recording Interface + MemoryRecorder (BE-07)
│   ├── telemetryservice/         # MQTT Telemetry Client — Paho (BE-05)
│   ├── vehicleconnection/        # Vehicle WebSocket Handler (BE-06)
│   └── webrtcsfu/                # WebRTC SFU Pion/Go — passiver Session-Event-Subscriber (ADR-020)
├── pkg/                          # Shared Go-Pakete
│   ├── ulid/                     # ULID-Wrapper (ADR-016)
│   ├── logger/                   # Strukturierter slog-Wrapper — JSON, Event-Type-Katalog (ADR-017)
│   └── audit/                    # AuditWriter Interface + SQLiteAuditWriter (ADR-018)
├── frontend/                     # React App (ADR-013)
│   └── src/
│       ├── components/           # VideoPanel, ControlPanel, SafetyPanel, ConnectionPanel, SafeModeOverlay
│       ├── hooks/                # useControls, useWebRTC, useTelemetry, useSession, useSystemState, useDeadmanSwitch
│       └── lib/                  # ws-client (Protobuf ACK), api-client
├── infrastructure/               # Docker & Compose
│   ├── docker/                   # Dockerfiles je Service, nginx.conf
│   ├── compose/                  # docker-compose.yml + docker-compose.prod.yml
│   ├── mediamtx/                 # mediamtx.yml — WHIP/WHEP Router Config (ADR-020)
│   ├── coturn/                   # STUN/TURN Config
│   ├── mosquitto/                # MQTT Broker Config
│   └── AWS/                      # CDK Stack (EC2, Security Groups, ADR-019)
├── tests/                        # Test-Suites
│   ├── unit/
│   ├── integration/
│   └── e2e/
├── go.mod                        # module avoc
├── Makefile                      # proto-gen, build, up, test, lint
└── .gitignore                    # gen/ gitignored
```

**Code-Gen:** `make proto-gen` → erzeugt `gen/go/` und `gen/ts/` aus `proto/*.proto`.

---

## Proto Schema Struktur (ADR-008/012/016)

```
proto/
├── common.proto      → CorrelationHeader (shared across all domains — ADR-016)
├── control.proto     → Control Commands + ControlAck
├── telemetry.proto   → Telemetry Events
├── safety.proto      → Safety Events (alle CRITICAL Trigger als SafetyEventType)
└── session.proto     → Session Events (SFU Push) + RecordingEntry
```

WebRTC Signaling (SDP/ICE) ist **bewusst außerhalb** des Protobuf-Schemas.

---

## Container Architecture (Docker Compose)

Alle Komponenten laufen containerisiert. Keine Kubernetes-Abhängigkeit.

### Services

| Service | Technologie | Zweck |
|---------|-------------|-------|
| `frontend` | React/Vite, nginx | SPA serving; nginx routet `/whep/` → MediaMTX |
| `control-server` | Go | WebSocket, State Machine, GSA, MediaMTX Auth-Hook + SAFE_MODE-Kick |
| `auth-service` | Go | JWT Ausstellung, Operator-Rollen, Handover-Token |
| `safety-service` | Go | Safety Event Bus (In-Memory, DDS-ready) |
| `telemetry-service` | Go | MQTT Bridge / Mosquitto Client |
| `mosquitto` | Eclipse Mosquitto | MQTT Broker |
| `webrtc-sfu` | Go / Pion | Passiver Session-Event-Subscriber (ADR-020); kein Media-Routing |
| `mediamtx` | bluenviron/mediamtx | WHIP/WHEP Router; Management API :9997 (ADR-020) |
| `stun-turn` | coturn | STUN/TURN für NAT Traversal; ICE-Credentials für MediaMTX + Browser |
| `loki` | Grafana Loki | Log-Aggregation (Phase 7) |
| `promtail` | Grafana Promtail | Log-Collector (Docker-Label-Discovery → Loki) |
| `grafana` | Grafana | Log-Visualisierung, Session-Dashboards (Phase 7) |

### Design Principles

- Jeder Service in isoliertem Container
- Keine geteilten Runtime-Dependencies
- Kommunikation nur via definierte Protokolle
- Volle lokale Reproduzierbarkeit via `docker-compose up`
- CI benötigt Docker + STUN/TURN (WebRTC E2E Tests non-blocking — ADR-006)

---

## Logging Architecture (ADR-017/018)

### Drei Log-Klassen

| Klasse | Verlust | Pfad |
|--------|---------|------|
| Technical Log | Erlaubt | async → slog → stdout → Docker → Loki |
| Audit Log | Nicht erlaubt | async → slog → stdout → Docker → Loki |
| **Safety Event** | **Niemals** | sync → `AuditWriter.WriteSync()` → SQLite (WAL) + async → Loki |

### Hybrid Sync/Async-Pipeline

```
                    ┌───────────────────┐
                    │  Control Server   │
                    └─────────┬─────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼

      Technical Logger                 Safety Logger
    (async — slog → stdout)        (sync — AuditWriter.WriteSync)

              │                               │
              ▼                               ▼

      Docker → Promtail          SQLite WAL (pkg/audit)
              │                               │
              ▼                               ▼
           Loki                         Audit Store
              │                   (garantierte Persistenz)
              ▼
           Grafana
```

**Invariante:** SAFE_MODE-Transition erst nach erfolgreichem `AuditWriter.WriteSync()` + fsync.

### Frontend Log-Ingestion

```
Browser → POST /api/log → Control Server → slog (service="frontend") → Loki
```

Keine direkte Loki-Verbindung aus dem Browser — Authentifizierung und Session-Kontext bleiben zentral.

### Pflichtfelder (alle Log-Einträge)

```json
{
  "time": "2026-06-03T19:00:00Z", "level": "INFO", "service": "control-server",
  "session_id": "01JTXY...", "event_id": "01JTXY...",
  "vehicle_id": "vehicle-001", "operator_id": "operator-1",
  "event_type": "SAFE_MODE_ENTERED", "msg": "Dead-man timeout"
}
```
