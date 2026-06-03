# Backlog

Lifecycle: backlog → sprint → done
Typen: S (<30 Min), M (30–180 Min), L (Architektur, ADR-pflichtig)

Stand: 2026-06-03 — aktualisiert nach Sprint 1/2/3

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

---

## EPIC: Teleoperation System (Backend)

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| BE-04 | Control Input System — Command Routing & Protobuf-Parsing | M | BE-02 ✅, BE-09 ✅ | ADR-007/010/012b/016; WS-Handler `readLoop` hat `TODO BE-04`-Kommentar; echtes Protobuf-Parsing (`ControlCommand`), DEADMAN_HOLD/RELEASE erkennen, Command-Type-Routing, Rate Limiting, Backpressure; ControlAck als Protobuf zurücksenden (ersetzt `{"ack":true}`); FE-Latenzanzeige wird dann exakt |
| BE-05 | MQTT Telemetry Service — Mosquitto Client + Pub/Sub | M | INFRA-01 ✅ | ADR-003/008/016; `internal/telemetryservice/` leer; Fahrzeugstatus (speed, battery, GPS) empfangen & publizieren; CorrelationHeader in MQTT Messages; Mosquitto läuft bereits (Port 1883) |
| BE-07 | Session Recording — Interface + Mock Adapter | M | BE-03 ✅, BE-06 ✅, INFRA-01 ✅ | ADR-005/016; `SessionRecorder`-Interface; StartSession, RecordControlEvent, RecordSafetyEvent, RecordStateSnapshot; session_id (ULID) als Root Key; Mock-Adapter für Tests; Storage-ADR noch offen (ADR-005 Folge) |
| BE-08 | WebRTC SFU Service — Pion/Go Media Server | M | INFRA-01 ✅, BE-09 ✅ | ADR-014/015; `internal/webrtcsfu/` leer; `cmd/webrtc-sfu/main.go` nur Health-Endpoint; Pion/Go SFU; Primary Stream immer aktiv; Secondary Streams on-demand; Adaptive Bitrate via RTCP; server-seitiges Recording; Session Event Consumer (SESSION_CREATED, OPERATOR_ASSIGNED, OPERATOR_HANDOVER, SESSION_DEGRADED, SESSION_SAFE_MODE, SESSION_ENDED via HTTP-Push vom Control Server) |

---

## EPIC: Frontend System

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| FE-05 | Control Panel UI — Joystick, Keyboard, Gamepad | M | FE-02 ✅, BE-04 | Virtuelle Steuerung (Joystick-SVG oder via Lib), Speed Slider; Control-Buttons in `frontend/src/App.tsx` Footer sind disabled — nach BE-04 aktivieren; CONTROL_BLOCKED → UI deaktiviert |
| FE-06 | Video Stream Panel — WebRTC Multi-Kamera UI | M | FE-01 ✅, BE-08 | ADR-014; `RTCPeerConnection` browser-nativ; SDP/ICE Signaling über bestehenden WS-Kanal; Primary Stream always-on; Secondary on-demand; MEDIA STATE Anzeige; DEGRADED-Warnung bei MEDIA_FAILED; Video Panel ist aktuell schwarze Placeholder-Box |
| FE-07 | Teleoperation Dashboard — Finales Layout & Integration | M | FE-03 ✅, FE-04 ✅, FE-05, FE-06, FE-08 ✅ | Zusammenführung aller UI-Module; Grid-Layout ggf. anpassen wenn Video aktiv; DEGRADED-State visuell deutlicher; Session-ID in Header; Operator-Handover UI (aktuell nur API, kein UI) |

---

## EPIC: Containerization

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| DC-04 | Local Dev Environment — README + Makefile finalisieren | S | DC-03 ✅ | `.env.example` ✅, `Makefile` (proto-gen, proto-gen-ts, build, up, down, test, test-safety, lint) ✅; fehlt: `README.MD` mit Setup-Anleitung (docker-compose up, lokale proto-gen, Vite dev-proxy), Troubleshooting-Hinweise für WSL/Docker-Socket |

---

## EPIC: Testing

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | DC-03 ✅ | ADR-006; Docker Compose für Tests (Mosquitto, WS, Safety, STUN/TURN); Playwright WebRTC-Flags (`--allow-insecure-localhost`); Basis für TEST-05 |
| TEST-04 | Frontend Test Infrastructure — Jest + RTL + Playwright | M | FE-01 ✅ | ADR-006; Vite-kompatible Test-Konfiguration (Vitest statt Jest empfohlen — Jest hat ESM-Probleme mit Vite); erste Component-Tests für `SafetyPanel`, `ConnectionPanel`, `SafeModeOverlay`; Playwright E2E (non-blocking, Retry 3×) |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | BE-02 ✅, BE-04, TEST-03 | ADR-006; k6 + Go Benchmarks; ACK-Roundtrip <100ms (nach BE-04: echter Protobuf-ACK statt `{"ack":true}`); Build-Fail bei Verletzung; WebRTC E2E non-blocking |

---

## Offene Entscheidungen (blockieren Tasks)

| Entscheidung | Blockiert | Referenz |
|---|---|---|
| Session Recording Storage (DB / Files / Object Storage) | BE-07 | ADR-005 Folge |
| Prioritätsmodell Channels vs. Header-Flag | BE-04 (teilweise) | ADR-008 Folge |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |

---

## Phasen-Übersicht (Restarbeit)

```
Phase 4 — Core Backend Services
  BE-04 (Command Routing + Protobuf-Parsing) — Gate für FE-05, TEST-05
  BE-05 (MQTT Telemetry)
  BE-07 (Session Recording)
  BE-08 (WebRTC SFU) — Gate für FE-06

Phase 5 — Feature Completion (Frontend)
  FE-05 (Control Panel) — wartet auf BE-04
  FE-06 (Video Panel)   — wartet auf BE-08
  FE-07 (Dashboard)     — wartet auf FE-05 + FE-06

Phase 6 — Testing & Quality Gates
  TEST-03 (Integration Test Infra)
  TEST-04 (Frontend Test Infra — Vitest statt Jest)
  TEST-05 (Latenz-Tests CI) — wartet auf BE-04 + TEST-03
  DC-04   (README + Makefile)
```
