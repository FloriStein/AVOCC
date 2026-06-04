# Backlog

Lifecycle: backlog → sprint → done
Typen: S (<30 Min), M (30–180 Min), L (Architektur, ADR-pflichtig)

Stand: 2026-06-04 — aktualisiert nach Sprint 1/2/3/4/5/6 + ADR-017/018 (Logging & Audit Trail)

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

## Offene Entscheidungen (blockieren Tasks)

| Entscheidung | Blockiert | Referenz |
|---|---|---|
| Session Recording Storage (DB / Files / Object Storage) | nach Sprint 7 | ADR-005 Folge — MemoryRecorder als Platzhalter |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store (SQLite Volume) | nach LOG-10 | ADR-019 möglich — SQLite-Volume-Sicherung für Produktivbetrieb |

---

## Phasen-Übersicht (Restarbeit)

```
Phase 6 — Testing & Quality Gates ✅ (abgeschlossen 2026-06-04)
  TEST-03 ✅ Integration Test Infra (docker-compose.test.yml, 9 Go Tests)
  TEST-04 ✅ Frontend Test Infra — Vitest 31/31 + Playwright E2E
  TEST-05 ✅ Latenz-Tests CI (Go Benchmark p99=0ms, k6 p99=244µs)
  DC-04   ✅ README Troubleshooting + Contributor Guide

Phase 7 — Logging & Audit Trail ✅ (abgeschlossen 2026-06-04)
  LOG-01..11 alle abgeschlossen — Safety Regression 19/19 ✅
```
