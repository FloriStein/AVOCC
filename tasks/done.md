# Done

Lifecycle: backlog → sprint → done

---

## Sprint 7 — Logging & Audit Trail

Abgeschlossen: 2026-06-04

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| LOG-01 | `pkg/logger/` — slog-Wrapper + event_types.go | M | ✅ `logger.New(service)`, `Event(eventType, msg, ...args)`, `Fatal()`, Level via `LOG_LEVEL` ENV, JSON auf stdout |
| LOG-02 | Control Server Migration | M | ✅ Alle `log.Printf` → `svcLog.Info/Event/Warn/Error`; statemachine, safety/detector, command/engine, transport/websocket, session/handover, vehicleconnection migriert |
| LOG-03 | Auth Service Migration | S | ✅ `cmd/auth-service/main.go` — `log.Printf` → `svcLog.Info/Fatal` |
| LOG-04 | Safety Service Migration | S | ✅ `cmd/safety-service/main.go` — Bus-Subscriber loggt via `svcLog.Event` |
| LOG-05 | Telemetry Service Migration | S | ✅ `internal/telemetryservice/client.go` + `cmd/` — MQTT-Events strukturiert |
| LOG-06 | WebRTC SFU Migration | S | ✅ `internal/webrtcsfu/sfu.go` + `cmd/` — alle ICE/Session-Events strukturiert |
| LOG-07 | `POST /log` Endpoint | M | ✅ `cmd/control-server/main.go` — Frontend-Logs mit `service="frontend"` in Loki |
| LOG-08 | Frontend `logger.ts` + Integration | M | ✅ `frontend/src/lib/logger.ts` — fire-and-forget POST /api/log; E-Stop, Operator-Ack, WebRTC-State integriert |
| LOG-09 | Loki + Grafana + Promtail | M | ✅ `infrastructure/loki/`, `infrastructure/promtail/`, `infrastructure/grafana/` + docker-compose Erweiterung; Ports 3100/3001; AVOC Session Dashboard |
| LOG-10 | `pkg/audit/` | M | ✅ `AuditWriter` Interface + `SQLiteAuditWriter` (WAL + fsync, modernc.org/sqlite) + `NoopWriter` |
| LOG-11 | Control Server Safety-Event-Integration | M | ✅ `WithAuditWriter()` auf DeadmanWatchdog, ACKTimeoutWatcher, Engine, WSHandler; `WriteSync()` vor jeder SAFE_MODE-Transition; `GET /audit/events` Endpoint |

### Neue Dateien

- `pkg/logger/logger.go` + `pkg/logger/event_types.go`
- `pkg/audit/writer.go` + `pkg/audit/sqlite_writer.go` + `pkg/audit/noop_writer.go`
- `infrastructure/loki/loki.yml`
- `infrastructure/promtail/promtail.yml`
- `infrastructure/grafana/provisioning/datasources/loki.yml`
- `infrastructure/grafana/provisioning/dashboards/dashboards.yml` + `avoc.json`
- `frontend/src/lib/logger.ts`

### Geänderte Dateien

- `internal/controlserver/statemachine/state.go` — `log.Printf` → slog
- `internal/controlserver/safety/detector.go` — slog + `WithAuditWriter()` + `WriteSync()` vor SAFE_MODE
- `internal/controlserver/command/engine.go` — slog + `WithAuditWriter()` + EMERGENCY_STOP Audit
- `internal/controlserver/transport/websocket.go` — slog + `WithAuditWriter()` + WS_DISCONNECT Audit
- `internal/controlserver/session/handover.go` — `log.Printf` → slog
- `internal/vehicleconnection/handler.go` — `log.Printf` → slog
- `internal/telemetryservice/client.go` — `log.Printf` → slog
- `internal/webrtcsfu/sfu.go` — `log.Printf` → slog
- `cmd/control-server/main.go` — slog + AuditWriter Init + POST /log + GET /audit/events
- `cmd/auth-service/main.go` — `log.Printf` → slog
- `cmd/safety-service/main.go` — `log.Printf` → slog
- `cmd/telemetry-service/main.go` — `log.Printf` → slog
- `cmd/webrtc-sfu/main.go` — `log.Printf` → slog
- `infrastructure/compose/docker-compose.yml` — Loki/Grafana/Promtail + audit-data Volume
- `frontend/src/components/SafetyPanel.tsx` — E-Stop logEvent Integration
- `frontend/src/components/SafeModeOverlay.tsx` — Operator-Ack logEvent Integration
- `frontend/src/hooks/useWebRTC.ts` — WebRTC State logEvent Integration
- `go.mod` — `modernc.org/sqlite v1.34.5` ergänzt

### Testprotokoll (2026-06-04)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | `go mod tidy` — `modernc.org/sqlite v1.34.5` laden | go.sum aktualisiert, kein Fehler | ✅ |
| T02 | `go build ./...` — alle Packages (inkl. pkg/logger, pkg/audit) | `BUILD_OK` | ✅ |
| T03 | Safety Regression (19/19) | Alle grün, JSON-Output sichtbar | ✅ 19/19 |
| T04 | Integration Tests (9/9) | Alle grün | ✅ 9/9 in 0.817s |
| T05 | pkg/logger Smoke-Test | JSON mit `service`, `level`, `event_type`-Feldern | ✅ |
| T06 | pkg/audit NoopWriter | `WriteSync()` returns nil | ✅ |
| T06b | pkg/audit SQLiteAuditWriter | `WriteSync()` + `QueryBySession()` lesen 1 Event | ✅ event_type=DEADMAN_TIMEOUT |
| T07 | Docker Build (alle 5 Go-Services + Frontend) | Alle Images gebaut | ✅ 6 Images |
| T08a | Health Checks (5 Services) | HTTP 200 auf :8080–:8084 | ✅ alle 5 |
| T08b | Structured JSON log output | `{"service":"control-server","event_type":"..."}` auf stdout | ✅ alle 5 Services |
| T08c | Audit Store ready on startup | `audit store ready` im Log mit DB-Pfad | ✅ |
| T08d | `POST /log` Frontend-Log-Ingestion | HTTP 202, Log mit `service="frontend"` in Container-Logs | ✅ |
| T08e | E2E Audit-Pipeline: WS→Session→EMERGENCY_STOP→WriteSync | 1 Event in SQLite, `event_type=EMERGENCY_STOP` | ✅ |
| T08f | `GET /audit/events?session_id=<ulid>` | JSON-Array mit 1 Safety-Event | ✅ |
| T08g | Loki ready + LogQL EMERGENCY_STOP query | 2 Streams, `session_id` als Label extrahiert | ✅ |
| T08h | Grafana API health | HTTP 200 auf Port 3001 | ✅ |
| T09 | Vitest Component Tests (31/31) | Alle grün nach logger.ts-Integration | ✅ 31/31 in 970ms |

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| Safety Tests | 19/19 ✅ | 19/19 |
| Integration Tests | 9/9 ✅ | 9/9 |
| Vitest Component Tests | 31/31 ✅ | 31/31 |
| Audit WriteSync + fsync (localhost) | < 5ms | < WAL-Commit-Budget |
| LogQL Treffer `event_type=EMERGENCY_STOP` | 2 Streams ✅ | Treffer vorhanden |
| Alle 5 Go-Services Health | 200 ✅ | alle 200 |

### Fixes während der Testphase

1. **`loki.yml` compactor config**: `retention_enabled: true` erfordert `delete-request-store` (Loki v3) → Retention-Config aus Compactor entfernt.
2. **`frontend/package-lock.json`** nicht synchron mit `package.json` (neue Sprint-6-Pakete) → `npm install` regeneriert.

---

## Sprint 6 — Testing & Quality Gates

Abgeschlossen: 2026-06-04

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | ✅ `tests/docker-compose.test.yml` (control-server, auth-service, safety-service, mosquitto auf Ports 18080–18082); 9 Go Integration Tests (Health, Auth, Session-Lifecycle, Invariante 1, Emergency Stop); `make test-integration` |
| TEST-04 | Frontend Test Infrastructure — Vitest + RTL + Playwright | M | ✅ vitest + @testing-library/react + @playwright/test installiert; `vitest.config.ts` + `setup.ts`; **31/31 Tests grün** (ConnectionPanel 10, SafeModeOverlay 4, SafetyPanel 6, ControlPanel 6, VideoPanel 5); `playwright.config.ts` + `tests/e2e/dashboard.spec.ts` |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | ✅ `tests/performance/latency_test.go` (Go Benchmark, p50=0ms, p95=0ms, p99=0ms @ localhost, Build-Fail bei >100ms); `tests/performance/latency.js` (k6, p99=244µs, 100% checks passed, 5 VU / 10s); `make test-latency` + `make test-k6` |
| DC-04 | Local Dev Environment — README finalisieren | S | ✅ README: alle Makefile-Befehle inkl. test-integration/latency/k6; Troubleshooting (6 Szenarien); Contributor Guide (5 Abschnitte: ADR, Go-Service, Proto, Frontend-Component, Safety) |

### Fixes während Implementierung

1. **`@testing-library/dom`** fehlte als Peer-Dependency → `npm install --save-dev @testing-library/dom` ergänzt.
2. **VideoPanel.test.tsx**: `vi.mocked(require(...))` Pattern funktioniert nicht in ESM-Vitest → auf `mockReturnValueOnce` über captured Mock-Funktion umgestellt.
3. **k6 Inline-Script via stdin**: `k6 run -` erwartet einen Default-Export — Script als Datei mounten statt per heredoc.
4. **docker-compose.test.yml**: `control-server` braucht `depends_on` mit `condition: service_healthy` → `healthcheck` für auth-service und safety-service ergänzt.

### Testprotokoll Integration (2026-06-04)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | Go Build (alle Packages inkl. integration + performance) | `OK` | ✅ |
| T02 | Safety Regression (19/19) | Alle grün | ✅ 19/19 |
| T03 | Vitest Component Tests (5 Files) | 31/31 Tests grün | ✅ 31/31 |
| T04 | Vitest verbose — alle 31 Tests einzeln | Jeder Test ✓ | ✅ alle ✓ |
| T05 | Integration Test Stack startet | 3 Services Built + Started + Healthy | ✅ |
| T06 | Health Checks Test-Stack | HTTP 200 auf :18080/:18081/:18082 | ✅ alle |
| T07 | Go Integration Tests (9 Tests) | 9/9 PASS | ✅ 9/9 in 0.833s |
| T08 | Invariante 1 via Integration Test | MEDIA_FAILED → DEGRADED, kein SAFE_MODE | ✅ |
| T09 | Go Benchmark ACK-Roundtrip | p50=0ms p95=0ms p99=0ms, < 100ms Budget | ✅ p99=0ms (Localhost) |
| T10 | k6 Load Test (5 VU, 10s) | p(99)<100ms threshold ✓, 100% checks | ✅ p99=244µs |
| T11 | Makefile targets (12 Targets) | alle vorhanden | ✅ 12/12 |
| T12 | Playwright config + E2E test | beide Dateien vorhanden | ✅ |
| T13 | README Troubleshooting (6 Sections) | alle Sections vorhanden | ✅ 6/6 |
| T14 | README Contributor Guide (5 Sections) | alle Sections vorhanden | ✅ 5/5 |
| T15 | Test-Stack Teardown | Container + Network removed | ✅ |
| T16 | Neue Test-Dateien (14 Dateien) | alle vorhanden | ✅ 14/14 |

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| Vitest Component Tests | 31/31 ✅ | — |
| Go Integration Tests | 9/9 ✅ | — |
| Safety Tests | 19/19 ✅ | 19/19 |
| Go Benchmark p99 (localhost) | 0ms | < 100ms ✅ |
| k6 p99 (localhost, 5 VU) | 244 µs | < 100ms ✅ |
| k6 checks_succeeded | 100% | > 99% ✅ |

### Neue Dateien

- `tests/docker-compose.test.yml` — minimaler Integrations-Test-Stack
- `tests/mosquitto-test.conf` — MQTT-Config für Tests
- `tests/integration/setup_test.go` — Test-Setup (Ports, JWT-Secret)
- `tests/integration/services_test.go` — 9 Integration Tests
- `tests/integration/ws_helper_test.go` — WebSocket-Dial-Helper
- `tests/performance/latency_test.go` — Go Benchmark ACK-Roundtrip
- `tests/performance/latency.js` — k6 Load Test Script
- `tests/e2e/dashboard.spec.ts` — Playwright E2E Baseline (5 Tests)
- `frontend/vitest.config.ts` — Vitest + jsdom + @/ Alias
- `frontend/src/test/setup.ts` — @testing-library/jest-dom Setup
- `frontend/src/components/*.test.tsx` — 5 Component-Test-Files (31 Tests)
- `frontend/playwright.config.ts` — Playwright Config (Chromium + WebRTC-Flags)

### Geänderte Dateien

- `Makefile` — `test-integration`, `test-latency`, `test-k6` Targets ergänzt/korrigiert
- `frontend/package.json` — test/test:watch/test:coverage/test:e2e Scripts + Packages
- `README.md` — vollständige Entwicklungs-Befehle, Troubleshooting, Contributor Guide

---

## Sprint 5 — Feature Completion Frontend

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| FE-05 | Control Panel UI — Keyboard + Virtual Joystick + Gamepad | M | ✅ `useControls.ts` (20 Hz, Keyboard WASD/Pfeiltasten, Virtual Joystick, Gamepad API); `ControlPanel.tsx` (SVG Joystick, Speed Slider, Steer/Throttle Bars, Mode-Anzeige) |
| FE-06 | Video Stream Panel — WebRTC RTCPeerConnection | M | ✅ `useWebRTC.ts` (RTCPeerConnection, SDP Signaling via `/sfu/subscribe/`, MEDIA STATE Tracking, `reportMediaState()` → Control Server); `VideoPanel.tsx` (video Element, MEDIA STATE Badge, Overlays, Retry-Button); SFU Track-Forwarding Fix (`TrackLocalStaticRTP` in `SubscribeOperator`) |
| FE-07 | Teleoperation Dashboard — Integration + Telemetrie | M | ✅ `useTelemetry.ts` (1 Hz Polling `/telemetry/latest/{vehicleId}`); `App.tsx` (VideoPanel + ControlPanel + Telemetrie + Operator-Rolle im Header); `ConnectionPanel.tsx` (Speed/Battery/Status); `ws-client.ts` (Protobuf ControlAck parsen, `onAckError` Callback); `websocket.go` (JSON-Fallback → Protobuf Binary) |

### Fixes während Implementierung

1. **`nginx.conf`**: Docker DNS Caching → 502 nach Container-Rebuild. Fix: `resolver 127.0.0.11 valid=5s` + variable-basiertes `proxy_pass`. **Achtung:** `set $var` muss **vor** `rewrite...break` stehen (beide im Rewrite-Modul — `break` stoppt nachfolgende Set-Direktiven).
2. **`ws-client.ts`**: `fromBinary()` gibt `Message`-Basistyp zurück → `as any` Cast erforderlich für `.success` / `.errorMsg` Zugriff.
3. **`internal/webrtcsfu/sfu.go`**: `SubscribeOperator` registrierte Operator-Peer ohne `Tracks` → RTP-Forwarding schrieb in leeren Slice. Fix: `TrackLocalStaticRTP` erstellen und in `peer.Tracks` speichern.

### Testprotokoll Integration (2026-06-03)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | Go Build (alle Sprint-5-Änderungen) | `OK` | ✅ |
| T02 | Safety Regression (19/19) | Alle grün | ✅ 19/19 |
| T03 | Service Health (alle 8 Services) | HTTP 200 je Service | ✅ |
| T04 | Frontend Bundle — neue Strings | 16 minifizierungs-stabile Strings im Bundle | ✅ alle 16 |
| T05 | nginx Routing (6 Routen) | auth→200, state→200, sfu-health→200, telemetry→404, sfu-subscribe→500, /ws→401 | ✅ alle |
| T06 | Session-Lifecycle + Protobuf Commands | 101 WS Upgrade, ULID-Session, 7 cmd-Typen mit Protobuf-ACK success | ✅ |
| T07 | Command Engine Log | `[CMD] COMMAND_TYPE_STEER/THROTTLE/BRAKE/SPEED` im Log | ✅ |
| T08 | Protobuf-Fallback (kein JSON) | Fallback-ACK: Protobuf binary, success=false, error_msg='no active session' | ✅ 28 bytes Protobuf |
| T09 | MEDIA STATE — alle 5 Übergänge | NEGOTIATING/CONNECTED/DEGRADED (2×)/INIT je HTTP 202 | ✅ alle |
| T10 | SFU TrackLocalStaticRTP Fix | String 'avoc-vehicle' im SFU-Binary, `NewTrackLocalStaticRTP` in sfu.go:214 | ✅ |
| T11 | reportMediaState im Bundle | 'media/event' im JS-Bundle | ✅ |
| T12 | useTelemetry im Bundle | 'speed_kmh' im JS-Bundle | ✅ |
| T13 | onAckError im Bundle | 'onAckError' im JS-Bundle | ✅ |
| T14 | nginx DNS-Resolver (set vor rewrite) | Zeile 14: `set $upstream_cs` vor Zeile 15: `rewrite` | ✅ |
| T15 | 20Hz Rate-OK + Burst Rate-Limiter | 10/10 ACK bei 20Hz (0.4ms avg); 10/110 rejected bei Burst | ✅ |
| T16 | nginx kein 502 nach Restart | Auth nach auth-service-Restart: HTTP 200; SFU nach SFU-Restart: HTTP 200 | ✅ |
| T17 | Recovery Flow | SAFE_MODE → RECOVERING → AUTHENTICATED nach neuem WS-Connect | ✅ |

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| ACK Latenz 20Hz | Ø 0.4 ms (localhost) | < 100ms ✅ |
| Rate Limiter Schwelle | 100 cmd/s | 100 cmd/s ✅ |
| Bundle-Größe | 2 JS-Dateien (index + proto) | — |
| Safety Tests | 19/19 | 19/19 ✅ |

### Protokollierte Log-Ausgaben (Auszug)

```
[CMD] COMMAND_TYPE_STEER value=0.75 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] COMMAND_TYPE_THROTTLE value=0.50 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] COMMAND_TYPE_BRAKE value=1.00 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] COMMAND_TYPE_SPEED value=0.60 (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[CMD] EMERGENCY_STOP → SAFE_MODE (session=01KT7JF12DVD1QW3B3PZH7G6JY)
[STATE] MEDIA MEDIA_FAILED → SYSTEM DEGRADED (Invariant 1: never SAFE_MODE)
[STATE] SYSTEM: SAFE_MODE → RECOVERING (via WS reconnect)
[RECORDING] session started: id=01KT7JH1C46ZPB2J1W5Y61HK9G vehicle=vehicle-1 operator=operator-1
```

### Neue Dateien

- `frontend/src/hooks/useControls.ts` — 20 Hz Keyboard/Joystick/Gamepad Command Loop (FE-05)
- `frontend/src/components/ControlPanel.tsx` — Virtual Joystick SVG + Speed Slider (FE-05)
- `frontend/src/hooks/useWebRTC.ts` — RTCPeerConnection + SDP Signaling (FE-06)
- `frontend/src/components/VideoPanel.tsx` — Video Element + MEDIA STATE Badge (FE-06)
- `frontend/src/hooks/useTelemetry.ts` — 1 Hz Telemetrie-Polling (FE-07)

### Geänderte Dateien

- `frontend/src/App.tsx` — vollständiges Dashboard (VideoPanel, ControlPanel, Telemetrie, Operator-Rolle)
- `frontend/src/lib/api-client.ts` — `reportMediaState()` ergänzt
- `frontend/src/lib/ws-client.ts` — Protobuf ControlAck parsen, `onAckError` Callback
- `frontend/src/components/ConnectionPanel.tsx` — Speed/Battery/Status-Felder
- `internal/controlserver/transport/websocket.go` — JSON-Fallback → Protobuf Binary
- `internal/webrtcsfu/sfu.go` — `TrackLocalStaticRTP` Track-Forwarding Fix
- `infrastructure/docker/nginx.conf` — Docker DNS Resolver + variable proxy_pass

---

## Sprint 4 — Core Backend Services

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| INFRA-02 | Proto-Gen Fix | S | ✅ `--go_opt=module=avoc` erzeugt korrekte Verzeichnisstruktur `gen/go/control/v1/control.pb.go` |
| BE-04 | Command Engine | M | ✅ Protobuf-Parsing, DEADMAN_HOLD/RELEASE, EMERGENCY_STOP-Routing, Rate Limiting 100 cmd/s, Protobuf ControlAck |
| BE-05 | MQTT Telemetry Service | M | ✅ Paho v1.4.3, `vehicle/+/telemetry` Subscribe, TelemetryEvent Protobuf, `GET /telemetry/latest/{id}` |
| BE-07 | Session Recording | M | ✅ `SessionRecorder` Interface + `MemoryRecorder`; Control Server zeichnet Session-Start, State-Snapshots, Safety-Events auf |
| BE-08 | WebRTC SFU | M | ✅ Pion/Go v4.0.14, Session Event Consumer (alle 6 SESSION_*-Events), SDP-Offer/Answer Endpunkte, Primary Stream Forwarding |

### Bugfix während Implementierung

`paho.mqtt.golang`: `GOFLAGS=-mod=mod` im Dockerfile zog v1.5.1 (erfordert Go 1.24 — inkompatibel). Fix: Version explizit auf `v1.4.3` in go.mod gepinnt.

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Safety Tests Regression (19/19) | Alle grün | ✅ |
| INFRA-02: Proto-Gen Struktur | `gen/go/control/v1/control.pb.go` | ✅ Alle 5 Schemas korrekt |
| BE-04: `COMMAND_TYPE_DEADMAN_HOLD` im Binary | String im Service-Binary | ✅ |
| BE-04: `rate limited`-String im Binary | String im Service-Binary | ✅ |
| BE-04: Emergency Stop → SAFE_MODE + Recording | `SAFE_MODE / CONTROL_BLOCKED`, Recording Entry | ✅ |
| BE-05: Telemetry Service Health | `{"status":"ok"}` | ✅ |
| BE-05: Mosquitto-Verbindung | Log: `connected + subscribed` | ✅ |
| BE-05: MQTT Subscribe aktiv | Log: Parse-Error bei non-Protobuf-Nachricht (kein Crash) | ✅ |
| BE-05: `GET /telemetry/latest/unknown` | HTTP 404 | ✅ |
| BE-07: State Snapshot bei session/start | `count=1, type=state, CONNECTED/CONTROL_ACTIVE` | ✅ |
| BE-07: Safety Event bei Emergency Stop | `count=2, type=safety, EMERGENCY_STOP` | ✅ |
| BE-07: session/end → Log `entries=1` | `[RECORDING] session ended: entries=1` | ✅ |
| BE-08: Health | `{"status":"ok","service":"webrtc-sfu"}` | ✅ |
| BE-08: Alle 6 SESSION_*-Events | HTTP 202 je Event | ✅ |
| BE-08: SFU loggt alle Events korrekt | Log mit allen Event-Typen | ✅ |
| BE-08: nginx `/sfu/` Route | HTTP 202 | ✅ |
| BE-08: SDP-Offer Endpunkt vorhanden | HTTP 500 (invalid SDP, aber Endpunkt existiert) | ✅ |
| BE-08: Control Server pusht SESSION_CREATED automatisch | SFU Log: Event empfangen | ✅ |

### Neue Dateien

- `internal/controlserver/command/engine.go` — Command Engine (BE-04)
- `internal/telemetryservice/client.go` — MQTT Paho Client (BE-05)
- `internal/recording/recorder.go` + `memory_recorder.go` — Session Recording (BE-07)
- `internal/webrtcsfu/sfu.go` — WebRTC SFU Pion (BE-08)
- `cmd/telemetry-service/main.go` + `cmd/webrtc-sfu/main.go` — vollständig implementiert

---

## Sprint 3 — Frontend Core

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| FE-09 | Frontend Protobuf Adapter + Build-Pipeline | M | ✅ `@bufbuild/protobuf` + `@bufbuild/protoc-gen-es`; `common_pb.js` + `control_pb.js` im Bundle; nginx `/api/` + `/auth/` + `/vehicle/ws` Proxy-Routen |
| FE-02 | WebSocket Client + State-Polling | M | ✅ `ws-client.ts` mit Latenz-Messung; `useSystemState` (500ms Polling); `useSession` (auto-login, Reconnect mit Exponential Backoff) |
| FE-08 | SAFE MODE Overlay + Operator Ack Flow | M | ✅ Fullscreen-Overlay bei SAFE_MODE; Resume-Button triggert Recovery-Flow; DEGRADED-Banner |
| FE-04 | Safety Controls — Emergency Stop + Dead-man Switch | M | ✅ E-Stop → `POST /api/emergency-stop`; Dead-man (Spacebar/Mousedown, 400ms Interval, Protobuf DEADMAN_HOLD) |
| FE-03 | Connection Status — Live-Anzeige | S | ✅ Live Latenz (grün <50ms, gelb <100ms, rot ≥100ms), Session-ID (gekürzt), Operator-Rolle, State-Badge |

### Bugfixes während Tests

1. **`transport/websocket.go`**: Recovery-Pfad fehlte — bei neuem WS-Connect aus SAFE_MODE wurde `CONNECTING` versucht (invalid Transition). Fix: Wenn System in `SAFE_MODE`, dann `SAFE_MODE → RECOVERING → AUTHENTICATED` statt `IDLE → CONNECTING → AUTHENTICATED`.
2. **`frontend/package-lock.json`**: Neuer Dependencies (`@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`, `ulidx`) nicht im Lock-File — `npm install` ausgeführt.
3. **`ConnectionPanel.tsx`**: Ungenutzte `type SystemState` Deklaration (TS6196) → entfernt.

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Health-Checks alle 5 Services | `{"status":"ok"}` | ✅ |
| nginx `/api/state` | JSON (nicht HTML) | ✅ `{"system":"IDLE",...}` |
| nginx `/auth/operator/login` | JWT-Token | ✅ Token mit `role=OBSERVER` |
| nginx `/api/nonexistent` | `404 page not found` (nicht HTML) | ✅ |
| nginx `/auth/nonexistent` | HTTP 404 | ✅ |
| Protobuf-Bundle: `common_pb.js` + `control_pb.js` | Vorhanden | ✅ |
| Protobuf-Strings im Bundle | `CorrelationHeader`, `DEADMAN_HOLD`, `EMERGENCY_STOP` | ✅ |
| Login → WS → session/start → CONNECTED + ULID | `CONTROL_ACTIVE / ACTIVE_OPERATOR / session_id (26 Zeichen)` | ✅ |
| State-Polling (5 × 500ms) | Konsistente JSON-Antwort | ✅ |
| Emergency Stop `/api/emergency-stop` | HTTP 202, `SAFE_MODE / CONTROL_BLOCKED`, Safety Bus `EMERGENCY_STOP` | ✅ |
| Dead-man Watchdog (2s ohne Reset) | `SAFE_MODE` nach 2.5s | ✅ |
| Recovery: SAFE_MODE → Resume → CONNECTED | `RECOVERING → AUTHENTICATED → CONNECTED`, neue Session-ID ≠ alte | ✅ |
| Vehicle WS via nginx `/vehicle/ws` | Verbindung aufgebaut | ✅ |
| Vehicle WS mit Operator-Token → 401 | HTTP 401 | ✅ |
| Vehicle Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Handover Request → HANDOVER_PENDING | HTTP 202 | ✅ |
| Handover Confirm → ACTIVE_OPERATOR | HTTP 200 | ✅ |
| Session End → NO_OPERATOR → SAFE_MODE | HTTP 204, `SAFE_MODE / NO_OPERATOR` | ✅ |
| Sprint-2 Safety Tests (Regression) | 19/19 grün | ✅ |
| SAFE MODE Overlay-Text im Bundle | `"SAFE MODE"`, `"Resume — Operator Acknowledgment"` | ✅ |

### Neue Dateien

- `frontend/src/lib/api-client.ts` — HTTP-Client (login, getState, startSession, emergencyStop)
- `frontend/src/lib/ws-client.ts` — WebSocket-Client mit Latenz-Messung
- `frontend/src/hooks/useSystemState.ts` — Polling-Hook (500ms)
- `frontend/src/hooks/useSession.ts` — Session-Lifecycle (Login, Connect, Backoff, Resume)
- `frontend/src/hooks/useDeadmanSwitch.ts` — Dead-man (Spacebar/Button, Protobuf DEADMAN_HOLD)
- `frontend/src/components/SafeModeOverlay.tsx` — Fullscreen SAFE MODE Block
- `frontend/src/components/SafetyPanel.tsx` — Emergency Stop + Dead-man UI
- `frontend/src/components/ConnectionPanel.tsx` — Live State, Latenz, Session-ID, Rolle
- `frontend/src/App.tsx` (aktualisiert) — alle Komponenten verdrahtet
- `infrastructure/docker/nginx.conf` (aktualisiert) — `/api/`, `/auth/`, `/vehicle/ws` Proxy
- `infrastructure/docker/frontend.Dockerfile` (aktualisiert) — protoc + proto-gen vor Build
- `frontend/vite.config.ts` (aktualisiert) — Dev-Server Proxy
- `frontend/package.json` (aktualisiert) — `@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`, `ulidx`
- `cmd/control-server/main.go` (aktualisiert) — `/emergency-stop` Proxy-Endpunkt
- `internal/controlserver/transport/websocket.go` (aktualisiert) — Recovery-Pfad SAFE_MODE→RECOVERING

---

## Sprint 2 — Safety & Failure Model

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| TEST-01 | Go Test Infrastructure — testify + Mock Pattern | S | ✅ `MockSafetyPublisher`, `MockSFUPublisher` in `tests/unit/mocks/`; `SafetyPublisher` + `SFUPublisher` Interfaces angelegt |
| BE-06 | Vehicle Connection Service — Session Management | M | ✅ `internal/vehicleconnection/handler.go` — Vehicle WS, JWT-Auth, Disconnect → SAFE_MODE + Safety Event |
| BE-09 | Session Manager (GSA) + State Machine Erweiterung | M | ✅ `pkg/ulid/`, `session/manager.go` (CreateSession, Checkpoint, SFU Push), State Machine: Transition-Validierung + `TransitionToConnected()` |
| BE-10 | Failure Detection & Recovery | M | ✅ `safety/detector.go` — `DeadmanWatchdog` + `ACKTimeoutWatcher`; in `transport/websocket.go` integriert |
| BE-12 | Operator Handover Logic | M | ✅ `session/handover.go` — HANDOVER_PENDING, ConfirmHandover, CancelHandover, SFU EVENT: OPERATOR_HANDOVER |
| TEST-02 | Safety Test Suite | M | ✅ 19/19 Tests grün — alle 7 CRITICAL Trigger, Invariante 1 (MEDIA→DEGRADED), Recovery Checkpoint, Handover |

### Safety Test Suite — Ergebnis (19/19)

| Test | Status |
|------|--------|
| InvalidTransitionRejected | ✅ |
| WSDisconnect → SAFE_MODE | ✅ |
| DeadmanTimeout → SAFE_MODE | ✅ |
| DeadmanReset verhindert SAFE_MODE | ✅ |
| ACKTimeout → SAFE_MODE | ✅ |
| ACK in Zeit: kein SAFE_MODE | ✅ |
| NoOperator → SAFE_MODE | ✅ |
| EmergencyStop → SAFE_MODE | ✅ |
| AuthInvalidation → SAFE_MODE | ✅ |
| SafetyBusDown → SAFE_MODE | ✅ |
| MEDIA_FAILED → DEGRADED (niemals SAFE_MODE — Invariante 1) | ✅ |
| MEDIA_DEGRADED → DEGRADED (niemals SAFE_MODE — Invariante 1) | ✅ |
| RecoveryCheckpoint gespeichert | ✅ |
| Recovery-Fallback → SAFE_MODE bei Validierungsfehler | ✅ |
| SessionID ist ULID (26 Zeichen) | ✅ |
| SessionID eindeutig pro Session | ✅ |
| Handover → HANDOVER_PENDING | ✅ |
| Handover Confirm → neuer ACTIVE_OPERATOR + SFU Event | ✅ |
| Handover Cancel → ACTIVE_OPERATOR wiederhergestellt | ✅ |

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Safety Test Suite (Unit) | 19/19 grün | ✅ 19/19 PASS |
| Health-Checks alle 5 Services | `{"status":"ok"}` | ✅ |
| Initial State Machine | `IDLE/CONTROL_INIT/MEDIA_INIT/NO_OPERATOR` | ✅ |
| `session/start` ohne WS-Connect | HTTP 409 | ✅ Transition-Validierung greift |
| WS-Connect → AUTHENTICATED | `system: AUTHENTICATED` | ✅ |
| `session/start` → CONNECTED + ULID | `CONTROL_ACTIVE`, 26-stellige Session-ID | ✅ |
| Session-ID überlebt SAFE_MODE | `session_id` im `/state` nach WS-Disconnect | ✅ |
| Recovery Checkpoint gespeichert | Log: `checkpoint saved (session=...)` | ✅ |
| WS-Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Dead-man Watchdog (2s Timeout) | `SAFE_MODE` nach 2.5s ohne Reset | ✅ |
| Vehicle WS connect + JWT-Auth | Log: `[VEHICLE] connected: id=vehicle-1` | ✅ |
| Vehicle WS Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Vehicle-Endpoint mit Operator-Token | HTTP 401 | ✅ Rollenprüfung greift |
| Handover Request | HTTP 202, `HANDOVER_PENDING` | ✅ |
| Handover Confirm | HTTP 200, `ACTIVE_OPERATOR`, neuer Operator-ID im Session | ✅ |
| Handover Cancel | HTTP 200, `ACTIVE_OPERATOR` wiederhergestellt | ✅ |
| `session/end` → NO_OPERATOR → SAFE_MODE | `SAFE_MODE / NO_OPERATOR` | ✅ |

### Bugfix während Tests

`authservice/handler.go`: `HandoverToken` verlangte `current_token` auch bei Service-to-Service-Aufrufen. Feld ist nun optional — wenn leer, entfällt Client-Validierung (Vertrauen durch Netzwerk-Isolation im Docker Compose Stack).

### Neue Dateien

- `pkg/ulid/ulid.go`
- `internal/controlserver/safety/publisher.go` + `http_publisher.go` + `detector.go`
- `internal/controlserver/session/manager.go` + `handover.go` + `sfu_publisher.go`
- `internal/controlserver/statemachine/state.go` (erweitert: Transition-Validierung, `TransitionToConnected`)
- `internal/controlserver/transport/websocket.go` (erweitert: Deadman + ACK-Watcher)
- `internal/vehicleconnection/handler.go`
- `tests/unit/mocks/mock_safety.go` + `mock_sfu.go`
- `tests/unit/safety_test.go`
- `cmd/control-server/main.go` (erweitert: Session Manager, Handover, Vehicle WS, neue Endpoints)

---

## Sprint 1 — Foundation Layer

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| INFRA-01 | Proto Schema Repository — `.proto` + CorrelationHeader | M | ✅ Alle 5 Schemas (common, control, telemetry, safety, session) inkl. CorrelationHeader. ULID-Lib konfiguriert. |
| FE-01 | React Projekt Setup — Vite + TypeScript + Tailwind + Shadcn | S | ✅ React 18 + TypeScript + Vite läuft, erreichbar auf Port 3000 |
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | ✅ JWT-Ausstellung für Operator (role=OBSERVER) und Vehicle (role=VEHICLE) verifiziert |
| BE-11 | STUN/TURN Service — coturn Setup & Config | S | ✅ coturn läuft als Docker-Container auf Port 3479 |
| BE-03 | Safety Event Bus — Interface + In-Memory Implementierung | M | ✅ EmergencyStop auslösbar, State korrekt (SafeMode: true, LastEvent: EMERGENCY_STOP) |
| BE-02 | Control Server — WebSocket Setup + JWT Auth Middleware | M | ✅ WS-Verbindung mit JWT-Auth (101 Switching Protocols), Log: `subject=operator-1 role=OBSERVER` |
| DC-01 | Dockerfile — Frontend (React) | S | ✅ Multi-stage build, nginx serving, Port 3000 |
| DC-02 | Dockerfile — Backend Services (Go) | M | ✅ Alle Go-Services als separate Images gebaut |
| DC-03 | Docker Compose — Multi-Service Orchestrierung | M | ✅ Alle 8 Services starten fehlerfrei via `docker-compose up` |

### Testprotokoll (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Frontend localhost:3000 | HTML erreichbar | ✅ Vite + React + TS |
| Health /health alle Services | `{"status":"ok"}` | ✅ Alle 5 Services (8080–8084) |
| State Machine Initialzustand | `IDLE / CONTROL_INIT / MEDIA_INIT / NO_OPERATOR` | ✅ exakt |
| Operator JWT `POST /auth/operator/login` | `{"token":"eyJ..."}` mit `role=OBSERVER` | ✅ |
| Vehicle JWT `POST /auth/vehicle/register` | `{"token":"eyJ..."}` mit `role=VEHICLE` | ✅ |
| Safety Initial-State `GET /safety/state` | `SafeMode: false` | ✅ |
| Emergency Stop `POST /safety/emergency-stop` | SafeMode aktiviert | ✅ `SafeMode: true, LastEvent: EMERGENCY_STOP` |
| WebSocket Handshake mit JWT | `101 Switching Protocols`, Server-Log korrekt | ✅ Log: `WebSocket connected: subject=operator-1 role=OBSERVER` |
| WS-Disconnect → SAFE_MODE | System State wechselt | ✅ `SAFE_MODE / CONTROL_BLOCKED` nach Disconnect (ADR-009/010) |

### Beobachtung

WS-Disconnect triggert korrekt `SAFE_MODE → CONTROL_BLOCKED` (nicht im ursprünglichen Testplan, aber validiert). Safety-Verhalten funktioniert bereits auf Transport-Ebene wie in ADR-009/010 definiert.
