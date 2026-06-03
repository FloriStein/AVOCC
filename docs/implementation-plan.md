# Implementation Plan — Teleoperation System

Stand: 2026-06-03
Status: Phase 1/2/3 abgeschlossen ✅ — Phase 4/5/6 offen (alle ADRs abgeschlossen — 16 ADRs)

---

## 1. Executive Summary

Wir bauen ein sicheres, modulares Echtzeit-Teleoperation-System zur Fernsteuerung von Fahrzeugen über das offene Internet (Vehicle ↔ Internet ↔ OCC, uncontrolled routing). Die Architektur ist vollständig durch 14 ADRs entschieden. Die Implementierung startet mit einem stabilen Fundament (Proto-Schema, Auth, Safety Event Bus, Control Server) und wächst iterativ durch 6 Phasen zur vollständigen Lösung.

**Nicht-Verhandelbar:**
- Safety First — SAFE MODE ist nicht überbrückbar, Video darf SAFE MODE nie triggern
- <100ms Control Loop Latenz (ACK-Roundtrip, CI Build-Fail)
- Kein automatisches Resume nach CRITICAL Failure — immer Operator-Ack
- Alle Services laufen in Docker Compose
- 4-Layer State Machine: SYSTEM / CONTROL / MEDIA / OPERATOR

---

## 2. Entschiedene Technologien

| Bereich | Technologie | ADR |
|---------|-------------|-----|
| Backend Sprache | Go | ADR-001 |
| Frontend Framework | React 18+ | ADR-013 |
| Frontend Sprache | TypeScript | ADR-013 |
| Build Tool | Vite | ADR-013 |
| Styling | Tailwind CSS | ADR-013 |
| Component Library | Shadcn/ui | ADR-013 |
| Message Format | Protocol Buffers (Protobuf) — Application Bus | ADR-008 |
| Protobuf Code-Gen | protoc-gen-es (TypeScript), build-time, gitignored | ADR-012b/013 |
| Control Channel | WebSocket (WSS + JWT), Event-driven Stream, sync ACK | ADR-004/010/012b |
| Telemetry Channel | MQTT via Eclipse Mosquitto, async | ADR-003/012b |
| Safety Channel | Safety Event Bus (Go, In-Memory, DDS-ready), async | ADR-002/012b |
| Video Channel | WebRTC SFU (Pion/Go) + coturn (STUN/TURN) | ADR-014 |
| WebRTC Signaling | SDP/ICE — Media Layer, bewusst außerhalb Protobuf | ADR-014/008 |
| Authentifizierung | Separater Auth Service (Operator + Vehicle JWT) + Operator-Rollen | ADR-004 |
| Session Recording | Abstraktes Interface (Persistenz-ADR offen) | ADR-005 |
| Deployment | Docker Compose (kein Kubernetes) | CONTEXT |
| Test-Framework (Go) | testing + testify | ADR-006 |
| Test-Framework (Frontend) | Jest + RTL + Playwright (WebRTC-Flags) | ADR-006 |
| Latenz-Tests | k6 + Go Benchmarks, CI Build-Fail bei >100ms | ADR-006 |

---

## 3. Systemarchitektur (Übersicht)

**Zwei orthogonale Hubs (ADR-007):**
- **Control Hub** (Control Server): Safety Truth, Command Routing, State Machine
- **Video Hub** (WebRTC SFU): Media Relay, Recording, Multi-Operator Forwarding

```
                    ┌───────────────────────────────┐
  React Frontend ──▶│         Control Server         │
  (Browser)         │  · WebSocket Control (Protobuf)│
                    │  · SYSTEM STATE Machine        │
                    │  · CONTROL STATE Machine       │
                    │  · Failure Detection           │
                    │  · Operator Handover Logic     │
                    └─────────────┬─────────────────┘
                                  │
         ┌────────────────────────┼────────────────────┐
         │                        │                    │
  ┌──────▼──────┐      ┌──────────▼──────┐  ┌─────────▼──────┐
  │ Auth Service│      │ Safety Event Bus│  │ MQTT Telemetry │
  │ (JWT)       │      │ (Mock DDS)      │  │ (Mosquitto)    │
  └─────────────┘      └─────────────────┘  └────────────────┘

  React Frontend ──▶│         WebRTC SFU            │◀── Vehicle
  (Browser)         │  · Primary Stream (always on) │
                    │  · Secondary Streams (on-demand│
                    │  · Server-side Recording       │
                    │  · Multi-Operator Forwarding   │
                    └───────────────┬───────────────┘
                                    │ NAT Traversal
                             ┌──────▼──────┐
                             │   coturn    │
                             │ (STUN/TURN) │
                             └─────────────┘
```

---

## 4. Control Server — Interne Modulstruktur

Der Control Server ist logisch in 5 Module unterteilt — **ein Service, keine Microservices**. Die Trennung ist konzeptuell, um Testbarkeit und Debuggbarkeit zu gewährleisten.

```
┌─────────────────────────────────────────────────┐
│              Control Server (Go)                │
│                                                 │
│  ┌─────────────────┐  ┌─────────────────────┐  │
│  │ 1. Transport    │  │ 2. Command Engine   │  │
│  │    Layer        │  │                     │  │
│  │ · WebSocket     │  │ · Input Validation  │  │
│  │ · JWT Verify    │  │ · Rate Limiting     │  │
│  │ · Heartbeat     │  │ · Backpressure      │  │
│  │ · Channel Close │  │ · Routing           │  │
│  └────────┬────────┘  └──────────┬──────────┘  │
│           │                      │             │
│  ┌────────▼──────────────────────▼──────────┐  │
│  │         3. State Machine Engine          │  │
│  │   SYSTEM / CONTROL / MEDIA / OPERATOR    │  │
│  └─────────────────────┬────────────────────┘  │
│                         │                      │
│  ┌──────────────────────▼──────────────────┐   │
│  │       4. Safety Decision Module         │   │
│  │ · CRITICAL/DEGRADED Klassifizierung     │   │
│  │ · Invarianten-Enforcement               │   │
│  │ · SAFE_MODE Trigger                     │   │
│  └──────────────────────┬──────────────────┘   │
│                         │                      │
│  ┌──────────────────────▼──────────────────┐   │
│  │          5. Session Manager             │   │
│  │ · Single Source of Truth (Session State)│   │
│  │ · Operator-Rollen (Active/Observer)     │   │
│  │ · Handover-Koordination                 │   │
│  │ · SFU Session Context Bereitstellung    │   │
│  └─────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

**Wichtig:** Der Session Manager ist der einzige Punkt, der Session Context an den Video Hub (SFU) bereitstellt. Der SFU fragt nie aktiv Zustand ab — er empfängt ihn (push, async).

### Session Correlation (ADR-016)

```
Erzeuger: Control Server (Session Manager)
Zeitpunkt: CONNECTING → CONNECTED
Format: ULID (zeitlich sortierbar, URL-safe, distributed-safe)

Identifier-Hierarchie:
  Vehicle-ID
    └── Session-ID (ULID — Root Anchor)
          └── Event-ID (ULID — pro Message/Command/Frame)

CorrelationHeader in allen .proto Schemas:
  session_id, event_id, vehicle_id, operator_id, timestamp

JWT = Identity (Wer bist du?) ≠ Session-ID (In welchem Kontext?)
```

Session-ID überlebt SAFE_MODE als Root-Referenz. Recovery = neue Execution Branch unter derselben Session-ID.

---

## 5. State Machine — 4-Layer Model (ADR-011)

```
┌─────────────────────────────────────────────────┐
│  OPERATOR STATE (Human Governance)              │
│  NO_OPERATOR → ASSIGNED → ACTIVE ⇄ HANDOVER    │
│  NO_OPERATOR → SYSTEM SAFE_MODE                │
└─────────────────────┬───────────────────────────┘
                      │ beeinflusst
                      ▼
┌─────────────────────────────────────────────────┐
│  SYSTEM STATE (Safety Truth — Master)           │
│  IDLE → CONNECTING → AUTHENTICATED → CONNECTED  │
│  CONNECTED/DEGRADED → SAFE_MODE → RECOVERING   │
│                                                 │
│  CRITICAL Trigger → SAFE_MODE:                 │
│    WS Disconnect, Safety Bus Failure,           │
│    Dead-man Timeout, Command ACK Timeout,       │
│    Auth Invalidation, No Active Operator        │
│                                                 │
│  DEGRADED Trigger (Control bleibt möglich):    │
│    Video Lost, Partial Telemetry Loss           │
└──────────────┬──────────────────────────────────┘
               │ bestimmt
    ┌──────────┴──────────────┐
    ▼                         ▼
┌───────────────┐    ┌─────────────────────┐
│ CONTROL STATE │    │    MEDIA STATE      │
│ CONTROL_INIT  │    │ MEDIA_INIT          │
│ CONTROL_ACTIVE│    │ MEDIA_NEGOTIATING   │
│ CONTROL_BLOCKED    │ MEDIA_CONNECTED     │
│ (bei SAFE_MODE)    │ MEDIA_DEGRADED ──▶ DEGRADED
│ CONTROL_LOST  │    │ MEDIA_FAILED   ──▶ DEGRADED
│ CONTROL_RECOV.│    │ (nie SAFE_MODE!)   │
└───────────────┘    └─────────────────────┘
```

**Safety Rule:** `MEDIA_FAILED → DEGRADED` — niemals `SAFE_MODE`. Video = Awareness only.

---

## 6. Failure Model (ADR-009)

| Klasse | Trigger | Reaktion |
|--------|---------|----------|
| **CRITICAL** | WS Disconnect, Safety Bus down, Dead-man Timeout, Command ACK Timeout, Auth Invalidation, No Operator | Channel Close → SAFE_MODE |
| **DEGRADED** | Video Lost (MEDIA_FAILED), Video Degraded, Partial Telemetry | DEGRADED State, Warnung im UI, Control weiterhin möglich |
| **OBSERVATION** | Auth Service down (laufende Session) | Neue Sessions blockiert, lokale JWT-Validierung weiterhin aktiv |

**Recovery:** Channel Close → Reconnect (Exp. Backoff) → Safety Bus Validierung → Operator-Ack → CONNECTED

---

## 7. Proto-Schema Struktur (ADR-008/012b/016)

```
proto/
├── common.proto      → CorrelationHeader (shared — ADR-016)
├── control.proto     → Control Commands + ControlAck (WebSocket, Protobuf)
├── telemetry.proto   → Telemetry Events (MQTT, Protobuf)
├── safety.proto      → Safety Events (Safety Bus, Protobuf)
└── session.proto     → Session Events (SFU Push) + RecordingEntry (Protobuf)
```

WebRTC Signaling (SDP/ICE) ist **bewusst außerhalb** des Protobuf-Schemas — standardisiertes Media Layer Protokoll.

Priorität: `Safety > Control > Telemetry` *(technische Durchsetzung: ADR-008 Folge)*

---

## 8. Implementierungsphasen

### Phase 1 — Foundation & Contracts ✅ *(Sprint 1, abgeschlossen 2026-06-03)*

**Ziel:** Lauffähiges Grundgerüst. `docker-compose up` bringt alle Core-Services hoch.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| INFRA-01 | Proto Schema Repository — alle `.proto` Definitionen | M | — |
| FE-01 | React Projekt Setup (Vite + TS + Tailwind + Shadcn) | S | — |
| BE-01 | Auth Service — JWT Ausstellung (Operator + Vehicle) | M | — |
| BE-03 | Safety Event Bus — Interface + In-Memory | M | INFRA-01 |
| BE-02 | Control Server — WebSocket + JWT Auth Middleware | M | BE-01, INFRA-01 |
| BE-11 | STUN/TURN Service — coturn Setup | S | — |
| DC-01 | Dockerfile Frontend | S | FE-01 |
| DC-02 | Dockerfile Backend Services | M | BE-01, BE-02, BE-03 |
| DC-03 | Docker Compose Orchestrierung (inkl. SFU + coturn) | M | DC-01, DC-02, BE-11 |

**Abhängigkeitspfad:**
```
INFRA-01 ──────────────────┐
FE-01    → DC-01 ──────────┤
BE-01    → BE-02 ─────────▶ DC-02 → DC-03 ✓
BE-03 (braucht INFRA-01) ──┘
BE-11 ─────────────────────┘
```

**Sprint DoD:**
- Proto-Schemas versioniert, Code-Gen (protoc-gen-es + protoc-gen-go) läuft
- JWT-Ausstellung für Operator + Vehicle funktioniert
- Control Server akzeptiert WSS-Verbindungen mit JWT-Auth
- Safety Event Bus antwortet auf EmergencyStop
- coturn läuft in Docker, STUN erreichbar
- React App im Browser erreichbar (leere Shell)
- `docker-compose up` startet alle Services fehlerfrei

---

### Phase 2 — Safety & Failure Model ✅ *(Sprint 2, abgeschlossen 2026-06-03)*

**Ziel:** 4-Layer State Machine vollständig implementiert. SAFE MODE funktioniert deterministisch.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| BE-09 | Control Server — 4-Layer State Machine + Session Manager (GSA, ULID, Checkpoint) | M | BE-02, BE-03 |
| BE-10 | Control Server — Failure Detection & Recovery | M | BE-09 |
| BE-06 | Vehicle Connection Service — Session Management + SAFE MODE Trigger | M | BE-01, BE-02 |
| BE-12 | Operator Handover Logic — Active/Observer/Standby | M | BE-09, BE-01 |
| TEST-01 | Go Test Infrastructure — testify + Mock Pattern | S | BE-01 |
| TEST-02 | Safety Test Suite — `safety_test.go` (alle CRITICAL Trigger) | M | BE-03, BE-09, TEST-01 |

**Phase DoD:**
- SYSTEM STATE Machine mit allen 7 Zuständen korrekt implementiert
- CONTROL, MEDIA, OPERATOR STATE implementiert und entkoppelt
- Alle CRITICAL Trigger lösen Auto-Stop + Channel Close aus (Safety Test Suite grün)
- Command ACK Timeout löst SAFE_MODE aus
- Operator-Ack Flow + Handover funktionieren
- MEDIA_FAILED triggert DEGRADED, niemals SAFE_MODE (verifiziert)

---

### Phase 3 — Frontend Core ✅ *(Sprint 3, abgeschlossen 2026-06-03)*

**Ziel:** Frontend kommuniziert mit Backend, SAFE MODE sichtbar und bedienbar.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| FE-09 | Frontend — Protobuf Adapter (protoc-gen-es) | M | INFRA-01, FE-01 |
| FE-02 | WebSocket Client Integration (sync ACK, Reconnect nach Channel Close) | M | FE-01, FE-09, BE-02 |
| FE-08 | SAFE MODE UI + Operator Ack Flow | M | FE-02, BE-09 |
| FE-04 | Safety Control UI — Emergency Stop + Dead-man Switch | M | FE-02, BE-03 |
| FE-03 | Connection Status Visualization (SYSTEM STATE sichtbar) | S | FE-02 |

**Phase DoD:**
- Frontend sendet/empfängt TypeScript Protobuf-Klassen via WebSocket
- SYSTEM STATE wird im UI korrekt gespiegelt (CONNECTED/DEGRADED/SAFE_MODE)
- SAFE MODE blockiert Steuerung, Operator-Ack Flow bedienbar
- Emergency Stop und Dead-man Switch funktionieren

---

### Phase 4 — Core Backend Services

**Ziel:** Alle Backend-Services vollständig implementiert, Video-Channel aktiv.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| BE-04 | Control Input System — Command Routing (Event-driven, Rate Limiting) | M | BE-02, BE-09 |
| BE-05 | MQTT Telemetry Service — Mosquitto Client + Pub/Sub | M | INFRA-01 |
| BE-07 | Session Recording — Interface + Mock Adapter | M | BE-03, BE-06, INFRA-01 |
| BE-08 | WebRTC SFU Service — Pion/Go (Primary + Secondary Streams, Recording) | M | INFRA-01 |

---

### Phase 5 — Feature Completion (Frontend)

**Ziel:** Vollständige Teleoperation-UI nutzbar. Video-Stream im Browser aktiv.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| FE-05 | Control Panel UI — Joystick, Keyboard, Gamepad, Speed Slider | M | FE-02, BE-04 |
| FE-06 | Video Stream Panel — WebRTC Multi-Kamera (Primary + Secondary on-demand) | M | FE-01, BE-08 |
| FE-07 | Teleoperation Dashboard — Gesamtlayout & Routing | M | FE-03, FE-04, FE-05, FE-06, FE-08 |

---

### Phase 6 — Testing & Quality Gates

**Ziel:** Vollständige Test-Infrastruktur, CI läuft durch, Latenz-Ziel verifiziert.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| TEST-03 | Integration Test Infrastructure — Docker (inkl. STUN/TURN, WebRTC-Flags) | M | DC-03 |
| TEST-04 | Frontend Test Infrastructure — Jest + RTL + Playwright | M | FE-01 |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms Build-Fail) | M | BE-02, TEST-03 |
| DC-04 | Local Dev Environment Setup (.env.example, Makefile, README) | S | DC-03 |

---

## 9. Vollständige Task-Übersicht

```
31 Tasks gesamt / 5 Epics

Phase 1 (Sprint 1): INFRA-01, FE-01, BE-01, BE-02, BE-03, BE-11, DC-01, DC-02, DC-03
Phase 2:           BE-06, BE-09, BE-10, BE-12, TEST-01, TEST-02
Phase 3:           FE-02, FE-03, FE-04, FE-08, FE-09
Phase 4:           BE-04, BE-05, BE-07, BE-08
Phase 5:           FE-05, FE-06, FE-07
Phase 6:           TEST-03, TEST-04, TEST-05, DC-04
```

---

## 10. Offene Folge-Entscheidungen

| Entscheidung | Blockiert | Wann nötig |
|---|---|---|
| Prioritätsmodell technisch (Channels vs. Header-Flag) | Phase 4 | ADR-008 Folge |
| Session Recording Storage (DB/Files/Object Storage) | BE-07 | ADR-005 Folge, vor Phase 4 |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |

---

## 11. ADR-Index (16 ADRs)

| ADR | Titel | Entscheidung |
|-----|-------|-------------|
| [ADR-001](adr/001-backend-language.md) | Backend Sprache | Go |
| [ADR-002](adr/002-safety-channel.md) | Safety Channel | Safety Event Bus (kein DDS, DDS-ready) |
| [ADR-003](adr/003-mqtt-broker.md) | MQTT Broker | Eclipse Mosquitto |
| [ADR-004](adr/004-authentication.md) | Authentifizierung | Separater Auth Service (Operator + Vehicle + Rollen) |
| [ADR-005](adr/005-session-recording.md) | Session Recording | Abstraktes Interface (Storage offen) |
| [ADR-006](adr/006-testing-strategy.md) | Testing Strategy | testing+testify / Docker / Safety Suite / Jest+Playwright / CI Latenz |
| [ADR-007](adr/007-system-runtime-topology.md) | System Topologie | Hub-and-Spoke (Control Hub ≠ Data Hub; SFU = orthogonaler Video Hub) |
| [ADR-008](adr/008-message-protocol.md) | Message Protocol | Protobuf (Application Bus); WebRTC Signaling außerhalb |
| [ADR-009](adr/009-failure-model.md) | Failure Model | CRITICAL/DEGRADED/OBSERVATION; Video=DEGRADED; Command ACK Timeout=CRITICAL |
| [ADR-010](adr/010-control-loop.md) | Control Loop & Safety Override | Event-driven Stream / ACK-Roundtrip / Channel Close |
| [ADR-011](adr/011-system-state-machine.md) | System State Machine | 4-Layer: SYSTEM / CONTROL / MEDIA / OPERATOR STATE |
| [ADR-012](adr/012-message-flow-runtime.md) | Message Flow Runtime | Field-based Protobuf Versioning, CI Schema-Gate |
| [ADR-012b](adr/012b-message-flow-runtime-sync-codegen.md) | Sync/Async & Code Generation | Safety/MQTT async; Auth async+lokale JWT; Frontend sync ACK; Build-time Code-Gen |
| [ADR-013](adr/013-frontend-tech-stack.md) | Frontend Technology Stack | React 18+ + TypeScript + Vite + Tailwind + Shadcn/ui + protoc-gen-es |
| [ADR-014](adr/014-video-streaming.md) | Video Streaming | WebRTC SFU (Pion/Go) + coturn; 1 Primary + 1-2 Secondary; Recording; Handover |
| [ADR-015](adr/015-session-coordinator.md) | Session Coordinator | Control Session als primäre Einheit; Control Server = GSA; Ephemeral + Checkpoint; SFU Event-Push |
| [ADR-016](adr/016-session-correlation-id.md) | Session Correlation ID | ULID; Control Server generiert bei CONNECTING→CONNECTED; Vehicle-ID→Session-ID→Event-ID; JWT = Identity only |
