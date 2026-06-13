# Backlog

Lifecycle: backlog → sprint → done
Typen: S (<30 Min), M (30–180 Min), L (Architektur, ADR-pflichtig)

Stand: 2026-06-12 — aktualisiert nach Sprint 12 (Vehicle Registry, ADR-022)

---

## Abgeschlossen (Referenz)

| ID | Sprint | Beschreibung |
|----|--------|-------------|
| INFRA-01 | 1 | Proto Schemas + CorrelationHeader + ULID |
| FE-01 | 1 | React + TypeScript + Vite + Tailwind Setup |
| BE-01 | 1 | Auth Service JWT (Operator + Vehicle) |
| BE-02 | 1 | Control Server WebSocket + JWT Middleware |
| BE-03 | 1 | Safety Event Bus (In-Memory) |
| BE-11 | 1 | coturn STUN/TURN Setup |
| DC-01 | 1 | Dockerfile Frontend |
| DC-02 | 1 | Dockerfile Backend Services |
| DC-03 | 1 | Docker Compose Orchestrierung |
| TEST-01 | 2 | Go Test Infrastructure (testify + Mocks) |
| TEST-02 | 2 | Safety Test Suite (19/19 Szenarien) |
| BE-06 | 2 | Vehicle Connection Service |
| BE-09 | 2 | Session Manager (GSA) + State Machine Erweiterung |
| BE-10 | 2 | DeadmanWatchdog + ACKTimeoutWatcher |
| BE-12 | 2 | Operator Handover Logic |
| FE-09 | 3 | Protobuf Adapter + Build-Pipeline |
| FE-02 | 3 | WebSocket Client + State-Polling |
| FE-08 | 3 | SAFE MODE Overlay + Operator Ack Flow |
| FE-04 | 3 | Emergency Stop + Dead-man Switch |
| FE-03 | 3 | Connection Status Panel |
| INFRA-02 | 4 | Proto-Gen Fix (module=avoc, korrekte Verzeichnisstruktur) |
| BE-04 | 4 | Command Engine — Protobuf-Parsing, DEADMAN_HOLD/RELEASE, Rate Limiting, ControlAck |
| BE-05 | 4 | MQTT Telemetry Service — Paho v1.4.3, vehicle/+/telemetry Subscribe |
| BE-07 | 4 | Session Recording — Interface + MemoryRecorder, Control Server Integration |
| BE-08 | 4 | WebRTC SFU — Pion/Go, Session Event Consumer, SDP Signaling |

---

## EPIC: Frontend System

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| FE-05 | Control Panel UI — Joystick, Keyboard, Gamepad | M | ✅ Sprint 5 | `useControls.ts` + `ControlPanel.tsx` |
| FE-06 | Video Stream Panel — WebRTC Multi-Kamera UI | M | ✅ Sprint 5 | `useWebRTC.ts` + `VideoPanel.tsx` + SFU Track-Fix |
| FE-07 | Teleoperation Dashboard — Finales Layout & Integration | M | ✅ Sprint 5 | `App.tsx` + `useTelemetry.ts` + Dashboard integriert |

---

## EPIC: Containerization

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| DC-04 | Local Dev Environment — README finalisieren | S | ✅ Sprint 6 | Troubleshooting (6 Szenarien), Contributor Guide (5 Abschnitte), alle Makefile-Befehle |

---

## EPIC: Testing

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | ✅ Sprint 6 | docker-compose.test.yml, 9 Go Integration Tests, make test-integration |
| TEST-04 | Frontend Test Infrastructure — Vitest + RTL + Playwright | M | ✅ Sprint 6 | 31/31 Vitest Tests grün, playwright.config.ts, E2E Baseline |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | ✅ Sprint 6 | Go Benchmark p99=0ms, k6 p99=244µs, make test-latency + make test-k6 |

---

---

## EPIC: Logging (ADR-017/018)

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| LOG-01 | `pkg/logger/` — slog-Wrapper + event_types.go | M | ✅ Sprint 7 | `logger.New(service)`, `Event()`, JSON stdout, `LOG_LEVEL` ENV |
| LOG-02 | Control Server Migration | M | ✅ Sprint 7 | statemachine, safety, command, transport, session, vehicleconnection migriert |
| LOG-03 | Auth Service Migration | S | ✅ Sprint 7 | cmd/auth-service/main.go — structured JSON |
| LOG-04 | Safety Service Migration | S | ✅ Sprint 7 | cmd/safety-service/main.go — Bus-Events via Event() |
| LOG-05 | Telemetry Service Migration | S | ✅ Sprint 7 | telemetryservice/client.go — MQTT-Events strukturiert |
| LOG-06 | WebRTC SFU Migration | S | ✅ Sprint 7 | webrtcsfu/sfu.go — ICE/Session-Events strukturiert |
| LOG-07 | `POST /log` Endpoint — Frontend Log-Ingestion | M | ✅ Sprint 7 | HTTP 202, `service="frontend"` in Loki |
| LOG-08 | Frontend `logger.ts` + Integration | M | ✅ Sprint 7 | fire-and-forget; E-Stop, Operator-Ack, WebRTC integriert |
| LOG-09 | Loki + Grafana + Promtail Docker Compose | M | ✅ Sprint 7 | Ports 3100/3001; Docker-Label-Discovery; AVOC Session Dashboard |
| LOG-10 | `pkg/audit/` — AuditWriter + SQLiteAuditWriter | M | ✅ Sprint 7 | WAL+fsync, NoopWriter, QueryBySession(), modernc.org/sqlite |
| LOG-11 | Control Server Safety-Event-Integration | M | ✅ Sprint 7 | WriteSync() vor SAFE_MODE in detector.go/engine.go/websocket.go; GET /audit/events |

**Abhängigkeitspfad:**
```
LOG-01 → LOG-02..06 (parallel) → LOG-07 → LOG-08
LOG-01 → LOG-09 (parallel zu allem)
LOG-10 → LOG-11 (nach LOG-02)
```

---

## EPIC: Deployment (Sprint 8) ✅

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| DEPLOY-01 | ADR-019 — Deployment-Strategie (Docker Hub + EC2 + SSM) | L | ✅ Sprint 8 | — |
| DEPLOY-02 | Makefile `build-prod` + `push` — linux/amd64, Docker Hub Tags | M | ✅ Sprint 8 | DEPLOY-01 |
| DEPLOY-03 | `docker-compose.prod.yml` — `image:` statt `build:`, prod Konfiguration | M | ✅ Sprint 8 | DEPLOY-02 |
| DEPLOY-04 | `scripts/setup-ssm.sh` + `scripts/deploy.sh` — SSM-Integration | M | ✅ Sprint 8 | DEPLOY-01 |
| DEPLOY-05 | coturn EC2-Konfiguration — `external-ip` via `TURN_EXTERNAL_IP` | M | ✅ Sprint 8 | DEPLOY-01 |
| DEPLOY-06 | Grafana Security — Login-Form + Admin-Credentials aus SSM | S | ✅ Sprint 8 | DEPLOY-03 |
| DEPLOY-07 | EC2 Bootstrap Guide — Checkliste für ersten Deploy ab null | M | ✅ Sprint 8 | DEPLOY-03, DEPLOY-04, DEPLOY-05 |

---

## EPIC: Video Stream (Sprint 9) ✅

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| STREAM-01 | ADR-020 — MediaMTX als WHIP/WHEP Router | L | ✅ Sprint 9 | — |
| STREAM-02 | `infrastructure/mediamtx/mediamtx.yml` + Docker Service | M | ✅ Sprint 9 | STREAM-01 |
| STREAM-03 | nginx: `/whep/` Proxy | S | ✅ Sprint 9 | STREAM-02 |
| STREAM-04 | `useWebRTC.ts` → WHEP-Protokoll + vehicleId-Prop | M | ✅ Sprint 9 | STREAM-02 |
| STREAM-05 | Control Server: `/internal/media/auth` + SAFE_MODE → MediaMTX API | M | ✅ Sprint 9 | STREAM-02 |
| STREAM-06 | TURN in MediaMTX ICE-Config + Compose env | S | ✅ Sprint 9 | STREAM-02 |
| STREAM-07 | CDK Port 8889 + SSM `whip-stream-key` + setup-ssm.sh | S | ✅ Sprint 9 | — |
| STREAM-08 | `docker-compose.prod.yml`: mediamtx + deploy.sh Update | S | ✅ Sprint 9 | STREAM-02 |
| STREAM-09 | Larix Setup Guide + E2E Smoke Test | S | ✅ Sprint 9 | STREAM-07, STREAM-08 |

---

## EPIC: Vehicle Registry (Sprint 12) ✅

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| VEH-REG-01 | ADR-022 — Vehicle Registry Architecture | L | ✅ Sprint 12 | `docs/adr/022-vehicle-registry.md` |
| VEH-REG-02 | `pkg/audit/sqlite_writer.go` — `DB() *sql.DB` getter | S | ✅ Sprint 12 | Shared WAL-Connection für vehicleregistry |
| VEH-REG-03 | `internal/vehicleregistry/` — VehicleStore, SQLiteVehicleStore, NoopVehicleStore | M | ✅ Sprint 12 | ErrNotFound-Sentinel; ConnectionChecker Interface |
| VEH-REG-04 | `cmd/control-server/main.go` — Store-Init + `GET/POST/DELETE /vehicles` | M | ✅ Sprint 12 | SeedDefault vehicle-001; 404 bei DELETE nicht-existent; 409 bei aktiver Session |
| VEH-REG-05 | `frontend/src/lib/api-client.ts` + `useVehicles.ts` | S | ✅ Sprint 12 | `VehicleInfo`, `listVehicles()`, 2s-Polling Hook |
| VEH-REG-06 | `frontend/src/hooks/useSession.ts` — `startSession(vehicleId)` | M | ✅ Sprint 12 | VEHICLE_ID-Hardcoding entfernt; expliziter Operator-Entscheid |
| VEH-REG-07 | `frontend/src/components/VehicleSelector.tsx` | M | ✅ Sprint 12 | Dropdown mit Online-Indikator + "Session starten"-Button |
| VEH-REG-08 | `SafetyPanel.tsx` + `ControlPanel.tsx` + `ConnectionPanel.tsx` + `App.tsx` | M | ✅ Sprint 12 | vehicleId-Prop-Chain; Auto-Start entfernt |

---

## EPIC: Vehicle Connectivity & Feedback (Sprint 11) ✅

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| VEH-01 | ADR-021 — Vehicle Connectivity & Feedback Architecture | L | ✅ Sprint 11 | `docs/adr/021-vehicle-connectivity-feedback.md` |
| VEH-02 | `proto/vehicle.proto` — VehicleCommandAck | S | ✅ Sprint 11 | Protobuf: header + command_event_id + received + received_at_ms |
| VEH-03 | `proto/telemetry.proto` — Actuation Fields 7–11 | S | ✅ Sprint 11 | steer/throttle/brake commanded + actual |
| VEH-04 | `internal/vehicleconnection/registry.go` — Registry + ForwardCommand | M | ✅ Sprint 11 | VehicleForwarder-Interface-Implementierung; thread-safe |
| VEH-05 | `internal/vehicleconnection/ackstore.go` — AckStore | S | ✅ Sprint 11 | Latest-ACK je vehicleID, sync.RWMutex |
| VEH-06 | `internal/controlserver/command/engine.go` — VehicleForwarder | M | ✅ Sprint 11 | Interface + WithVehicleForwarder(); kritischer Gap geschlossen |
| VEH-07 | `cmd/control-server/main.go` — Verdrahtung + ACK-Endpoint | M | ✅ Sprint 11 | `GET /vehicle/ack/latest/{vehicleID}` |
| VEH-08 | `cmd/vehicle-mock/main.go` — Docker-Service | L | ✅ Sprint 11 | JWT self-gen, WS, Protobuf decode, ACK send, MQTT lerp 15% |
| VEH-09 | `vehicle-mock.Dockerfile` + Compose + nginx | M | ✅ Sprint 11 | `/vehicle/` Proxy-Location, docker-compose dev+prod |
| VEH-10 | `useVehicleAck.ts` — Frontend Hook | S | ✅ Sprint 11 | 500ms-Polling `/vehicle/ack/latest/` |
| VEH-11 | `InputIndicatorPanel.tsx` — Fahrzeug-Feedback UI | M | ✅ Sprint 11 | Lenkrad-SVG (steerActual×120°), ActuationBars, AckBadge |
| VEH-12 | Tests: 7 Go + 7 TypeScript | M | ✅ Sprint 11 | 26/26 Go Unit, 41/41 Frontend Tests grün |

---

## EPIC: WebRTC ICE Migration (Sprint 10) ✅

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| WEBRTC-01 | CDK Security Group: 3478 TCP/UDP, 8189 UDP, 49152–65535 UDP | S | ✅ Sprint 10 | — |
| WEBRTC-02 | `mediamtx.yml`: `webrtcIPsFromInterfaces: false`, ICEServers2 entfernen, Port 8189 | S | ✅ Sprint 10 | — |
| WEBRTC-03 | coturn `network_mode: host`, `relay-ip`, `external-ip=PUBLIC/PRIVATE` | M | ✅ Sprint 10 | WEBRTC-01 |
| WEBRTC-04 | mediamtx UDP-Port 8889 → 8189 | S | ✅ Sprint 10 | WEBRTC-02 |
| WEBRTC-05 | `deploy.sh`: `TURN_PRIVATE_IP` aus EC2 IMDS (IMDSv2) | S | ✅ Sprint 10 | WEBRTC-03 |
| WEBRTC-06 | control-server: `GET /ice-config` Endpoint | M | ✅ Sprint 10 | — |
| WEBRTC-07 | control-server env: `TURN_USER`, `TURN_PASSWORD`, `TURN_EXTERNAL_IP` | S | ✅ Sprint 10 | WEBRTC-06 |
| WEBRTC-08 | `useWebRTC.ts`: DTLS-Fix, `/api/ice-config` fetch, 5s Gathering-Timeout | M | ✅ Sprint 10 | WEBRTC-06 |
| WEBRTC-09 | Deploy auf EC2 + E2E Smoke Test | M | ✅ Sprint 10 (E2E offen) | WEBRTC-01–08 |

---

## Offene Entscheidungen (blockieren zukünftige Tasks)

| Entscheidung | Blockiert | Referenz |
|---|---|---|
| Session Recording Storage (DB / Files / Object Storage) | offen | ADR-005 Folge — MemoryRecorder als Platzhalter |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store (SQLite Volume → S3) | offen | ADR-018 Folge — S3-Bucket im CDK vorhanden |
| Migration zu AWS ECR | offen | ADR-019 Folge — für Produktivbetrieb |
| HTTPS / TLS-Terminierung auf EC2 | offen | ADR-019 Folge — für Testphase HTTP akzeptabel |
| MQTT-Authentifizierung (Mosquitto Passwort-File) | offen | Port 1883 aktuell ohne Auth offen |
| Multi-Vehicle / vehicleId-Routing in MediaMTX | ✅ ADR-022 | VehicleSelector + SQLite-Registry; `~^vehicle-.*`-Regex aktiv |
| E2E Smoke Test mit aktiver WHIP-Quelle | offen | WEBRTC-09 Rest — Browser WiFi + 5G ICE-Pair verifizieren |
| Dev-Stack SSL-Fix: nginx.conf dev/prod trennen | offen | Sprint-10-Regression — `make up` startet Frontend nicht ohne SSL-Cert |
| vehicle-mock in Makefile GO_SERVICES | offen | build-prod + push übersehen vehicle-mock |
| session_id in MQTT-TelemetryEvent (vehicle-mock) | offen | Log-Korrelations-Gap: vehicle-mock hat keinen Operator-Session-Kontext |

---

## Phasen-Übersicht

```
Phase 6 — Testing & Quality Gates ✅ (abgeschlossen 2026-06-04)
  TEST-03 ✅  TEST-04 ✅  TEST-05 ✅  DC-04 ✅

Phase 7 — Logging & Audit Trail ✅ (abgeschlossen 2026-06-04)
  LOG-01..11 ✅ — Safety Regression 19/19 ✅

Phase 8 — EC2 Deployment via Docker Hub ✅ (abgeschlossen 2026-06-05)
  DEPLOY-01..07 ✅

Phase 9 — Video Stream: Larix WHIP → MediaMTX → Browser ✅ (abgeschlossen 2026-06-05)
  STREAM-01..09 ✅

Phase 10 — Browser WebRTC ICE Migration ✅ (deployed 2026-06-10)
  WEBRTC-01..09 ✅ (E2E Smoke Test offen)

Phase 11 — Vehicle Connectivity & Feedback ✅ (abgeschlossen 2026-06-11)
  VEH-01..12 ✅ — Go Build + 26/26 Unit Tests + 41/41 Frontend Tests grün

Phase 12 — Vehicle Registry ✅ (abgeschlossen 2026-06-12)
  VEH-REG-01..08 ✅ — ADR-022; SQLite vehicles-Tabelle; VehicleSelector; VEHICLE_ID-Hardcoding entfernt
```
