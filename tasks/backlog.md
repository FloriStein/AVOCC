# Backlog

Lifecycle: backlog → sprint → done
Typen: S (<30 Min), M (30–180 Min), L (Architektur, ADR-pflichtig)

Stand: 2026-06-03 — aktualisiert nach Sprint 1/2/3/4/5 + ADR-017 (Logging)

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
| DC-04 | Local Dev Environment — README finalisieren | S | DC-03 ✅ | README.md Grundstruktur vorhanden ✅; `make proto-gen-ts` vorhanden ✅; fehlt: Troubleshooting-Abschnitt (WSL, Docker-Socket, Port-Konflikte), Contributor-Guide |

---

## EPIC: Testing

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | DC-03 ✅ | ADR-006; Docker Compose für Tests; Playwright WebRTC-Flags (`--allow-insecure-localhost`); Basis für TEST-05 |
| TEST-04 | Frontend Test Infrastructure — Vitest + RTL + Playwright | M | FE-01 ✅ | ADR-006; **Vitest statt Jest** (ESM-kompatibel mit Vite); erste Component-Tests für `SafetyPanel`, `ConnectionPanel`, `SafeModeOverlay`; Playwright E2E-Basis |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | BE-02 ✅, BE-04 ✅, TEST-03 | ADR-006; k6 + Go Benchmarks; ACK-Roundtrip <100ms mit echtem Protobuf ControlAck (BE-04 ist fertig); Build-Fail bei Verletzung |

---

---

## EPIC: Logging (ADR-017)

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| LOG-01 | `pkg/logger/` — strukturierter slog-Wrapper (shared) | M | — | `log/slog` (Go stdlib, keine ext. Dep.); JSON-Handler; `Event()`-API mit session_id, event_id, event_type; Level-Konfiguration per ENV |
| LOG-02 | Control Server Migration — `log.Printf` → strukturierter Logger | M | LOG-01 | Alle `[CMD]`, `[DEADMAN]`, `[STATE]`, `[SESSION]`, `[RECORDING]` Calls migrieren; Event-Type-Enum aus ADR-017 |
| LOG-03 | Auth Service Migration | S | LOG-01 | Wenige Log-Calls; Login, Token-Ausstellung, Fehler |
| LOG-04 | Safety Service Migration | S | LOG-01 | EmergencyStop, Bus-Events |
| LOG-05 | Telemetry Service Migration | S | LOG-01 | MQTT connect/disconnect, Telemetry-Events |
| LOG-06 | WebRTC SFU Migration | S | LOG-01 | Session-Events, ICE-State-Changes |
| LOG-07 | `POST /log` Endpoint — Frontend Log-Ingestion | M | LOG-02 | Control Server empfängt strukturierte Frontend-Logs; validiert session_id; schreibt mit `service: "frontend"` |
| LOG-08 | Frontend `logger.ts` + Integration | M | LOG-07 | `src/lib/logger.ts` Utility; alle relevanten FE-Events (Emergency Stop, Deadman, WebRTC, WS-Reconnect, Operator-Ack); batching optional |
| LOG-09 | Loki + Grafana + Promtail Docker Compose | M | LOG-01 | `infrastructure/loki/`, `infrastructure/promtail/`, `infrastructure/grafana/`; Docker-Label-Discovery; JSON-Pipeline; provisioned Datasource + AVOC-Dashboard |
| LOG-10 | `pkg/audit/` — AuditWriter Interface + SQLiteAuditWriter (ADR-018) | M | LOG-01 | `modernc.org/sqlite` (pure Go, kein CGO); WAL-Modus; `WriteSync()` + fsync; NoopWriter für Tests; Schema: `audit_events` Tabelle mit Indizes auf session_id, event_type, timestamp |
| LOG-11 | Control Server Safety-Event-Integration — AuditWriter auf kritischem Pfad | M | LOG-10, LOG-02 | Alle Safety-Trigger in `detector.go`, `engine.go`, `websocket.go` schreiben synchron via `AuditWriter.WriteSync()` **vor** SAFE_MODE-Transition; `POST /audit/events` Query-Endpoint |

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
| Session Recording Storage (DB / Files / Object Storage) | zukünftige Recording-Qualität | ADR-005 Folge — MemoryRecorder als Platzhalter |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Audit Trail Strategy (Safety-Log-Garantie) | nach LOG-09 | ADR-018 — Loki ist kein Audit-Store |

---

## Phasen-Übersicht (Restarbeit)

```
Phase 6 — Testing & Quality Gates
  TEST-03 (Integration Test Infra)
  TEST-04 (Frontend Test Infra — Vitest)
  TEST-05 (Latenz-Tests CI) — BE-04 ✅: echter Protobuf-ACK jetzt messbar
  DC-04   (README Troubleshooting)

Phase 7 — Logging (ADR-017)
  LOG-01 pkg/logger shared package
  LOG-02..06 Backend-Migration (parallel)
  LOG-07 Frontend Log-Endpoint
  LOG-08 Frontend logger.ts
  LOG-09 Loki + Grafana + Promtail Infra
```
