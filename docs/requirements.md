# Requirements — Teleoperation Control System

Stand: 2026-06-03 (aktualisiert nach ADR-001 bis ADR-018)

---

## Vehicle Connection Requirements

- Secure session establishment
- JWT-based authentication für WebSocket-Verbindung (Operator + Vehicle, separater Auth Service — ADR-004)
- Operator-Rollen: Active Operator, Observer, Standby (ADR-011 OPERATOR STATE)
- Connection quality monitoring mit Latenzanzeige (<100ms Ziel für Control Loop)
- Auto-disconnect bei schlechter Verbindungsqualität
- Auto-reconnect mit exponential backoff nach CRITICAL Failure
- 30s Heartbeat (ping/pong)
- Exklusiver Active Operator pro Session (max. 1 gleichzeitig)

---

## Video Streaming Requirements

- Real-time Kamera-Feeds über WebRTC SFU (Pion/Go — ADR-014)
- NAT Traversal via ICE/STUN/TURN (coturn) — Vehicle ↔ Internet ↔ OCC Szenario
- **Kamera-Modell:**
  - 1 Primary Stream (immer aktiv, an alle Operatoren geforwardet)
  - 1–2 Secondary Streams (on-demand, Operator subscribed bei Bedarf)
- Adaptive Bitrate via WebRTC RTCP Feedback (built-in)
- Multi-Operator Video: alle Operatoren (Active + Observer) empfangen denselben Primary Stream
- Server-seitiges Video Recording (primär, Audit-fähig)
- Client-seitiges Recording (optional, via Browser MediaRecorder API)
- Video-Ausfall → DEGRADED State (kein Auto-Stop, kein SAFE MODE — ADR-009/011)
- QoS-Latenzziel Video: 100–300ms (kein Safety-Hartziel — ADR-014)
- WebRTC Signaling via bestehenden WebSocket-Kanal (außerhalb Protobuf-Schema — ADR-008/014)

---

## Manual Control Requirements

- Virtual Joystick Interface
- Keyboard Support
- Gamepad Support
- Speed Control Slider
- Emergency Stop Button (CRITICAL — triggert SAFE MODE, Channel Close)
- Dead-man Switch (Mandatory Active Hold — Loslassen = CRITICAL Trigger)
- Control nur im SYSTEM STATE = CONNECTED oder DEGRADED erlaubt
- Control im SAFE MODE vollständig blockiert (CONTROL_BLOCKED)

---

## Safety Requirements

- Operator Acknowledgment vor Steuerungsaufnahme (AUTHENTICATED → CONNECTED)
- Operator Acknowledgment nach Recovery aus SAFE MODE (RECOVERING → CONNECTED)
- Dead-man Switch (mandatory active hold — Timeout = CRITICAL)
- Emergency Stop (sofortige Systemdeaktivierung — bypasses alle Ebenen)
- Auto-Stop bei CRITICAL Failure (Channel Close — ADR-010)
- Command ACK Timeout = CRITICAL Trigger → SAFE MODE (ADR-009)
- No Active Operator = CRITICAL Trigger → SAFE MODE
- Session Recording für Audit (abstraktes Interface — ADR-005)
- SAFE MODE: Fahrzeug steht, keine Commands, Channel geschlossen, UI read-only
- Kein automatisches Resume nach CRITICAL Failure — immer Operator-Ack
- Recovery Checkpoint bei SAFE_MODE-Eintritt (Vehicle-ID, Operator-ID, letzter System/Control State, Safety Reason Code, Session-ID — ADR-015)
- Recovery = Neu-Aktivierung eines validierten Zustands, kein automatisches Wiederherstellen

---

## State Machine Requirements (ADR-011)

Das System implementiert ein 4-Layer State Machine Modell:

### SYSTEM STATE (Safety Truth — Master)
Zustände: `IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED → SAFE_MODE → RECOVERING`

### CONTROL STATE (Command Flow)
Zustände: `CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED → CONTROL_LOST → CONTROL_RECOVERING`
Regel: SAFE_MODE ⇒ CONTROL_BLOCKED. VIDEO_LOSS ⇒ kein CONTROL STATE Change.

### MEDIA STATE (WebRTC/Video Health)
Zustände: `MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED → MEDIA_DEGRADED → MEDIA_FAILED`
Regel: MEDIA_FAILED → SYSTEM DEGRADED. Niemals SAFE_MODE.

### OPERATOR STATE (Human Governance)
Zustände: `NO_OPERATOR → OPERATOR_ASSIGNED → ACTIVE_OPERATOR ⇄ HANDOVER_PENDING → RECOVERING_OPERATOR`
Regel: NO_OPERATOR → SYSTEM SAFE_MODE. Max. 1 ACTIVE_OPERATOR pro Session.

---

## Session Requirements (ADR-015)

- **Primäre Session:** Control Session = 1 Vehicle + 1 Active Operator + 1 Control Server Instanz
- **Session-Hierarchie:**
  1. Vehicle Runtime Context (Transport: WebRTC/MQTT/WS)
  2. Control Session (Safety Layer — einzige Safety-relevante Einheit)
  3. Operator Session (Identity Layer — JWT/Login)
- Max. 1 aktive Control Session pro Vehicle
- Operator-Wechsel via Handover möglich — Control Session bleibt bestehen
- Vehicle-Reconnect via RECOVERING möglich — Control Session bleibt logisch bestehen
- Control Session endet bei: CRITICAL Failure ohne Recovery, explizitem Session-Ende, Timeout
- **Global Session Authority (GSA):** Control Server ist einziger Session-Erzeuger, -Verwalter und -Zerstörer
- Session State in-memory (ephemeral), Recovery Checkpoint bei SAFE_MODE
- SFU empfängt Session Context via Event-Push vom Control Server (SESSION_CREATED, OPERATOR_HANDOVER, SESSION_SAFE_MODE, SESSION_ENDED)

---

## Operator Handover Requirements (ADR-011/014/015)

- Multi-Operator-Betrieb: ein Active Operator + beliebig viele Observer/Standby
- Steuerungsübergabe (Handover) zwischen Operatoren unterstützt
- HANDOVER_PENDING State: Übergabe muss von beiden Seiten bestätigt werden
- Während HANDOVER_PENDING: aktueller Operator behält Steuerung
- Observer sehen denselben Video Primary Stream wie Active Operator
- Auth Service verwaltet Operator-Rollen und Handover-Token (ADR-004)

---

## Communication Requirements

- **WebSocket (WSS):** Real-time Control, Protobuf-Messages, Synchron ACK-based (ADR-010/012b)
- **MQTT (Mosquitto):** Telemetry & Status Updates, asynchron (ADR-003/012b)
- **Safety Event Bus (Go, In-Memory):** Safety-critical Events, asynchron, DDS-Interface-kompatibel (ADR-002/012b)
- **WebRTC SFU (Pion/Go):** Video Streaming, NAT Traversal via coturn (ADR-014)
- **Message Format:** Protocol Buffers (Protobuf) für Control/Telemetry/Safety — ADR-008
- **WebRTC Signaling:** SDP/ICE außerhalb Protobuf-Schema (Media Layer — ADR-008/014)
- **Protobuf Versioning:** Field-based, keine Breaking Changes, CI Schema-Gate (ADR-012)
- **Code Generation:** Build-time via protoc, gitignored, protoc-gen-es (TS) + protoc-gen-go (ADR-012b)
- **Session Correlation ID:** ULID, generiert vom Control Server bei `CONNECTING → CONNECTED` (ADR-016)
- **CorrelationHeader** in allen `.proto`-Schemas: `session_id`, `event_id`, `vehicle_id`, `operator_id`, `timestamp`
- **Identifier-Hierarchie:** `Vehicle-ID → Session-ID (ULID) → Event-ID (ULID)` über alle Kanäle
- **JWT = Identity only** — kein Session Context im JWT (ADR-016)
- **Session-ID überlebt SAFE_MODE** als Root-Anchor; Recovery = neue Execution Branch unter gleicher Session-ID

---

## Performance Requirements

- **Control Loop Latenz:** < 100ms (ACK-Roundtrip Client → Control Server, CI Build-Fail bei Verletzung — ADR-010/006)
- **Video Latenz:** QoS-Ziel 100–300ms (kein Safety-Hartziel — ADR-014)
- **Safety Events:** near-instant (Safety Event Bus Priority Channel)
- High reliability connection recovery (Exponential Backoff)
- Graceful degradation bei schlechter Netzwerkqualität (DEGRADED State)
- Rate Limiting und Backpressure im Control Input System (ADR-010)

---

## Frontend Requirements (ADR-013)

- Web Application (Browser-based)
- **Sprache:** TypeScript
- **Framework:** React 18+
- **Build Tool:** Vite
- **Styling:** Tailwind CSS
- **Component Library:** Shadcn/ui
- **Protobuf:** protoc-gen-es (TypeScript-Klassen, build-time generiert)
- Real-time Communication mit Backend via WebSocket (Protobuf, sync ACK)
- WebRTC RTCPeerConnection für Video-Empfang (browser-nativ)
- Modular UI:
  - **Video Panel:** Primary Stream (always on) + Secondary Streams (on-demand), MEDIA STATE Anzeige
  - **Control Panel:** Joystick, Keyboard, Gamepad, Speed Slider
  - **Safety Panel:** Emergency Stop, Dead-man Switch, SAFE MODE Indikator, Operator Ack Flow
  - **Connection Status Panel:** SYSTEM STATE, Latenzanzeige, Operator-Rolle (Active/Observer)
  - **Operator Panel:** Handover-Anfrage, Observer-Liste
- UI blockiert Steuerung bei CONTROL_BLOCKED (SAFE MODE Overlay)
- DEGRADED State sichtbar im UI (Video-Warnung), Control bleibt möglich
- Reconnect-Logik nach Channel Close (ADR-010)

---

## Deployment Requirements (ADR-001/CONTEXT)

- System vollständig containerisiert via Docker
- Docker Compose orchestriert alle Services
- Lokale Entwicklung vollständig via `docker-compose up`
- **Services:**
  - `frontend` — React App (Vite Build, nginx serving)
  - `control-server` — Go (WebSocket, 4-Layer State Machine, Safety Decision, Session Manager/GSA, ULID Generation)
  - `auth-service` — Go (JWT, Operator-Rollen, Handover-Token)
  - `safety-service` — Go (Safety Event Bus, In-Memory)
  - `telemetry-service` — Go (MQTT Bridge / Mosquitto Client)
  - `webrtc-sfu` — Go/Pion (Media Server, Recording, Multi-Operator)
  - `stun-turn` — coturn (NAT Traversal für Vehicle ↔ Internet ↔ OCC)
  - `mosquitto` — Eclipse Mosquitto (MQTT Broker)
  - `loki` — Grafana Loki (Log-Aggregation — Phase 7)
  - `grafana` — Grafana (Log-Visualisierung, Session-Dashboards — Phase 7)
- Kein Kubernetes in dieser Phase
- Full local reproducibility via `docker-compose up`
- CI benötigt Docker-Environment (Integration Tests, WebRTC Browser-Flags)

---

## Logging Requirements (ADR-017/018)

### Strukturiertes Logging

- Alle Services (Backend + Frontend) loggen strukturiert als JSON
- Jeder Log-Eintrag enthält: `time`, `level`, `service`, `session_id`, `event_id`, `vehicle_id`, `operator_id`, `event_type`, `msg`
- Kein Freitext-Logging — typisierter `event_type` aus definiertem Katalog
- Frontend sendet Logs via `POST /api/log` an Control Server (niemals direkt an Loki)

### Log-Klassen

| Klasse | Verlust erlaubt | Anforderung |
|--------|-----------------|-------------|
| Technical Log | Ja | async slog → stdout → Loki |
| Audit Log | Nein | async slog → stdout → Loki |
| Safety Event | Niemals | **synchron** via `AuditWriter.WriteSync()` → SQLite + Loki |

### Safety-Log-Garantie

- Safety Events (`EMERGENCY_STOP`, `DEADMAN_TIMEOUT`, `SAFE_MODE_ENTERED`, `SAFE_MODE_EXITED`, `COMMAND_ACK_TIMEOUT`, `SAFETY_BUS_FAILURE`, `OPERATOR_HANDOVER_COMPLETED`, `SESSION_STARTED`, `SESSION_ENDED`) müssen garantiert persistiert werden
- Persistenz erfolgt **vor** dem Abschluss der SAFE_MODE-Transition (fsync)
- Loki-Ausfall darf Safety-Event-Persistenz nicht beeinflussen
- Audit Store: SQLite WAL-Modus (embedded, kein extra Service — ADR-018)

### Session-Rekonstruktion

- Eine vollständige Teleoperation-Session muss über Frontend-, Backend-, Safety-, Telemetry- und Video-Ereignisse hinweg rekonstruierbar sein
- Korrelation über `session_id` (ULID, ADR-016) als primären Anker
- Grafana-Dashboard: LogQL-Abfragen wie `{session_id="01J..."}` über alle Services
- SQLite-Query: `SELECT * FROM audit_events WHERE session_id=? ORDER BY timestamp`

### Latenz-Anforderung

- Technical-Log-Overhead: `<1ms` (async slog, stdout-Puffer)
- Safety-Event-Overhead: `~1–5ms` (fsync) — akzeptabel, da Safety Events selten
- Control-Loop-Budget `<100ms` (ADR-010) darf durch Logging nicht verletzt werden
