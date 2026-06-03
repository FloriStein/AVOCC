# Backlog

Lifecycle: backlog → sprint → done
Typen: S (<30 Min), M (30–180 Min), L (Architektur, ADR-pflichtig)

---

## EPIC: Infrastructure & Contracts

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| INFRA-01 | Proto Schema Repository — `.proto` + `CorrelationHeader` + ULID | M | — | ADR-008/016; control/telemetry/safety/session.proto; CorrelationHeader (session_id, event_id, vehicle_id, operator_id, timestamp) in allen Schemas; ULID-Lib für Go (oklog/ulid) + TS konfigurieren; Code-Gen Config für protoc-gen-go + protoc-gen-es |

---

## EPIC: Teleoperation System (Backend)

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | — | ADR-004; Go; POST /auth/*/login, /token/validate; Operator-Rollen (Active/Observer/Standby) |
| BE-02 | Control Server — WebSocket Setup + JWT Auth Middleware | M | BE-01, INFRA-01 | ADR-007/010/011; WSS, JWT-Validierung, Heartbeat 30s; CONNECTING→AUTHENTICATED Transition; CorrelationHeader in jeder Nachricht (ADR-016) |
| BE-03 | Safety Event Bus — Interface + In-Memory Implementierung | M | INFRA-01 | ADR-002/008; PublishSafetyEvent, EmergencyStop, GetSafetyState; CorrelationHeader in Safety Events |
| BE-05 | MQTT Telemetry Service — Mosquitto Client + Pub/Sub | M | INFRA-01 | ADR-003/008/016; Fahrzeugstatus empfangen & publizieren; CorrelationHeader in MQTT Messages |
| BE-06 | Vehicle Connection Service — Session Management | M | BE-01, BE-02 | ADR-015; löst SAFE_MODE bei Disconnect aus; Disconnect-Erkennung; kein Session-State-Ownership (GSA = Control Server) |
| BE-07 | Session Recording — Interface + Mock Adapter | M | BE-03, BE-06, INFRA-01 | ADR-005/016; StartSession, RecordControlEvent, RecordSafetyEvent; session_id als Root Key; Storage-ADR noch offen |
| BE-08 | WebRTC SFU Service — Pion/Go Media Server | M | INFRA-01, BE-09 | ADR-014/015; Primary + Secondary Streams; Adaptive Bitrate; server-seitiges Recording; Multi-Operator Forwarding; Session Event Consumer (SESSION_CREATED, OPERATOR_ASSIGNED, OPERATOR_HANDOVER, SESSION_DEGRADED, SESSION_SAFE_MODE, SESSION_ENDED) |
| BE-09 | Control Server — 4-Layer State Machine + Session Manager (GSA) | M | BE-02, BE-03 | ADR-011/015/016; SYSTEM/CONTROL/MEDIA/OPERATOR State; GSA: ULID Session-ID bei CONNECTING→CONNECTED generieren; Recovery Checkpoint bei SAFE_MODE; SFU Event-Push |
| BE-10 | Control Server — Failure Detection & Recovery | M | BE-09 | ADR-009/010/011/015; Disconnect-Detection, Channel Close, Exponential Backoff, Command ACK Timeout, RECOVERING→SAFE_MODE Fallback; Checkpoint laden bei Recovery |
| BE-11 | STUN/TURN Service — coturn Setup & Config | S | — | ADR-014; NAT Traversal für Vehicle ↔ Internet ↔ OCC; Docker-Container; STUN Port 3478 |
| BE-12 | Operator Handover Logic — Active/Observer/Standby | M | BE-09, BE-01 | ADR-011/015; OPERATOR STATE; HANDOVER_PENDING Transition; exklusiver Active Operator; Handover-Token via Auth Service; SFU EVENT: OPERATOR_HANDOVER |
| BE-04 | Control Input System — Command Routing & Processing | M | BE-02, BE-09 | ADR-007/010/012b/016; Event-driven Stream, Rate Limiting, Backpressure; Safety Bus async; Auth async+lokale JWT; CorrelationHeader in jedem Command |

---

## EPIC: Frontend System

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| FE-01 | React Projekt Setup — Vite + TypeScript + Tailwind + Shadcn | S | — | ADR-013; ESLint, tsconfig, Ordnerstruktur |
| FE-09 | Frontend — Protobuf Adapter (protoc-gen-es) | M | INFRA-01, FE-01 | ADR-012b/013/016; TypeScript-Klassen, build-time, gitignored; CorrelationHeader in allen ausgehenden Messages |
| FE-02 | WebSocket Client Integration — Verbindungsmanagement | M | FE-01, FE-09, BE-02 | ADR-008/010/011/012b; Sync ACK-based; Reconnect nach Channel Close; SYSTEM STATE im Client spiegeln; Session-ID aus Server-Response lesen |
| FE-03 | Connection Status Visualization | S | FE-02 | ADR-016; Latenzanzeige, SYSTEM STATE Indikator, Session-ID und Operator-Rolle anzeigen |
| FE-04 | Safety Control UI — Emergency Stop + Dead-man Switch | M | FE-02, BE-03 | ADR-009/015; Safety-kritisch; Emergency Stop = CRITICAL Trigger; Operator Ack vor Aktivierung UND nach Recovery |
| FE-08 | SAFE MODE UI + Operator Ack Flow | M | FE-02, BE-09 | ADR-009/011/015; SAFE MODE Overlay blockiert Steuerung; visueller SAFE_MODE Indikator; Resume-Bestätigung (Operator Ack) |
| FE-05 | Control Panel UI — Joystick, Keyboard, Gamepad | M | FE-02, BE-04 | Virtuelle Steuerung, Speed Slider |
| FE-06 | Video Stream Panel — WebRTC Multi-Kamera UI | M | FE-01, BE-08 | ADR-014; RTCPeerConnection, Primary always-on, Secondary on-demand; MEDIA STATE Anzeige; DEGRADED Warnung bei MEDIA_FAILED |
| FE-07 | Teleoperation Dashboard — Gesamtlayout & Routing | M | FE-03, FE-04, FE-05, FE-06, FE-08 | Zusammenführung aller UI-Module |

---

## EPIC: Containerization

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| DC-01 | Dockerfile — Frontend (React) | S | FE-01 | Multi-stage build (Vite + nginx serving) |
| DC-02 | Dockerfile — Backend Services (Go) | M | BE-01, BE-02, BE-03 | Multi-stage Go build; je Service ein eigenes Image |
| DC-03 | Docker Compose — Multi-Service Orchestrierung | M | DC-01, DC-02, BE-11 | Services: frontend, control-server, auth-service, safety-service, telemetry-service, mosquitto, webrtc-sfu, stun-turn; Netzwerk, Volumes, Env-Vars |
| DC-04 | Local Dev Environment Setup | S | DC-03 | .env.example, Makefile (proto-gen, build, up), README |

---

## EPIC: Testing

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| TEST-01 | Go Test Infrastructure — testify Setup + Mock Pattern | S | BE-01 | ADR-006; testify einrichten; Mock-Pattern für Safety Bus, MQTT und SFU Event Stream |
| TEST-02 | Safety Test Suite — Dedicated `safety_test.go` | M | BE-03, BE-09, TEST-01 | ADR-006/009/011; Dead-man Timeout, Emergency Stop, WS Disconnect, Command ACK Timeout, No Active Operator → SAFE_MODE; MEDIA_FAILED → DEGRADED (nicht SAFE_MODE); Recovery Checkpoint validieren |
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | DC-03 | ADR-006; Docker Compose für Tests (Mosquitto, WS, Safety, STUN/TURN); Playwright WebRTC-Flags (--allow-insecure-localhost) |
| TEST-04 | Frontend Test Infrastructure — Jest + RTL + Playwright | M | FE-01 | ADR-006; Test-Frameworks einrichten; erste Component-Tests; Playwright E2E-Basis (non-blocking, Retry 3×) |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | BE-02, TEST-03 | ADR-006; k6 + Go Benchmarks; ACK-Roundtrip <100ms; Build-Fail bei Verletzung; WebRTC E2E non-blocking |
