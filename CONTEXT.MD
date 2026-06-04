# CONTEXT.MD — Domänenwissen & Architekturprinzipien

---

## Glossar

| Begriff | Definition |
|---------|-----------|
| **Operator** | Menschlicher Nutzer, der ein Fahrzeug über das System steuert |
| **Vehicle** | Ferngesteuertes Fahrzeug, das Steuerbefehle empfängt |
| **Control Session** | Primäre operative Einheit: 1 Vehicle + 1 Active Operator + 1 Control Server. Einzige Safety-relevante Einheit (ADR-015) |
| **Control Input** | Steuerbefehl (Joystick, Keyboard, Gamepad) vom Operator |
| **Dead-man Switch** | Muss aktiv gehalten werden — Loslassen löst Auto-Stop aus |
| **Emergency Stop** | Sofortige Systemdeaktivierung, bypasses alle anderen Ebenen |
| **Auto-Stop** | Automatischer Stopp bei CRITICAL Failure (Channel Close) |
| **SAFE MODE** | Systemzustand: Fahrzeug steht, keine Commands, Channel geschlossen, UI read-only, Operator-Ack für Recovery |
| **Operator Ack** | Explizite Bestätigung des Operators — vor Aktivierung UND nach Recovery aus SAFE MODE |
| **Recovery Checkpoint** | Zustandssnapshot bei SAFE_MODE-Eintritt (Vehicle-ID, Operator-ID, letzter State, Safety Reason, Session-ID) |
| **Safety Event Bus** | Go-Service, In-Memory; ersetzt DDS in dieser Phase; DDS-Interface-kompatibel (ADR-002) |
| **Global Session Authority (GSA)** | Control Server als einziger Session-Erzeuger, -Verwalter und -Zerstörer (ADR-015) |
| **Control Hub** | Control Server als Rang-1-Hub für Safety Truth und Command Routing (ADR-007) |
| **Video Hub** | WebRTC SFU als Rang-2-Hub für Media Relay, Recording, Multi-Operator (ADR-007/014) |
| **WebRTC SFU** | Selective Forwarding Unit (Pion/Go) — Dumb Media Router with State Subscription (ADR-014) |
| **Protobuf** | Protocol Buffers — Application Bus für Control/Safety/Telemetry (ADR-008) |
| **CorrelationHeader** | Protobuf-Nachrichtenfeld in allen Schemas: session_id, event_id, vehicle_id, operator_id, timestamp (ADR-016) |
| **ULID** | Universally Unique Lexicographically Sortable Identifier — Format der Session-IDs (zeitlich sortierbar) |
| **CRITICAL Failure** | Fehler der sofortigen Channel Close + SAFE MODE auslöst |
| **DEGRADED State** | Teilausfall (Video/Telemetrie) — Control möglich, Awareness reduziert |
| **OBSERVATION** | Nicht-kritischer Ausfall (Auth Service down) — laufende Session aktiv, neue Sessions blockiert |

---

## Domänenmodell

### Entities

- Vehicle
- Control Session (primäre Einheit — ADR-015)
- Operator (Active / Observer / Standby)
- Control Input
- Video Stream (Primary / Secondary)
- Safety State (inkl. SAFE MODE + Recovery Checkpoint)
- Connection State

### Session-Hierarchie (ADR-015)

```
1. Vehicle Runtime Context   (Transport: WebRTC/MQTT/WS/5G)
2. Control Session           (Safety Layer — primäre Einheit)
   └── 1 Vehicle + 1 Active Operator + 1 Control Server
3. Operator Session          (Identity Layer — JWT/Login)
```

### 4-Layer State Machine (ADR-011)

**SYSTEM STATE (Safety Truth — Master):**
```
IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED
CONNECTED/DEGRADED → SAFE_MODE (CRITICAL) → RECOVERING → AUTHENTICATED
```
CRITICAL Trigger: WS Disconnect, Safety Bus Failure, Dead-man Timeout, Command ACK Timeout, Auth Invalidation, No Active Operator

**CONTROL STATE** (abhängig von SYSTEM STATE):
`CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED (bei SAFE_MODE) → CONTROL_LOST → CONTROL_RECOVERING`

**MEDIA STATE** (WebRTC, unabhängig — beeinflusst nur DEGRADED):
`MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED ⇄ MEDIA_DEGRADED → MEDIA_FAILED`
MEDIA_FAILED → SYSTEM DEGRADED. **Niemals SAFE_MODE.**

**OPERATOR STATE** (Human Governance):
`NO_OPERATOR → OPERATOR_ASSIGNED → ACTIVE_OPERATOR ⇄ HANDOVER_PENDING → RECOVERING_OPERATOR`
Regel: Max. 1 ACTIVE_OPERATOR. NO_OPERATOR → SYSTEM SAFE_MODE.

---

## Safety Concepts

- **Dead-man Switch:** muss aktiv gehalten werden — Timeout = CRITICAL
- **Emergency Stop:** sofortige Systemdeaktivierung, bypasses alle Ebenen → CRITICAL
- **Auto-Stop:** Channel Close bei CRITICAL Failure (ADR-010)
- **Command ACK Timeout:** CRITICAL Trigger → SAFE_MODE
- **Operator Ack:** vor Aktivierung UND nach Recovery — niemals automatisches Resume
- **SAFE MODE:** Channel geschlossen, CONTROL_BLOCKED, UI read-only, Recovery Checkpoint gespeichert

---

## Failure Classification (ADR-009)

| Klasse | Trigger | Reaktion |
|--------|---------|----------|
| **CRITICAL** | WS Disconnect, Safety Bus Failure, Dead-man Timeout, Command ACK Timeout, Auth Invalidation, No Active Operator | Channel Close → SAFE_MODE |
| **DEGRADED** | Video Lost (MEDIA_FAILED), Video Degraded, Partial Telemetry Loss | DEGRADED State, Warnung im UI, Control möglich |
| **OBSERVATION** | Auth Service down (bestehende Session) | Neue Sessions blockiert, JWT-Validierung lokal weiter |

**Formale Invarianten:**
```
INVARIANT 1: Media Layer SHALL NOT influence SAFE_MODE transitions
             except via DEGRADED annotation evaluated by the Control Hub.
INVARIANT 2: SAFE_MODE is exclusively triggered by Control, Safety Bus,
             or Operator-level failures. Never by Media events.
INVARIANT 3: Control Hub is Single Source of Truth for Session State.
```

---

## Communication Layers (entschieden)

| Kanal | Protokoll | Zweck | ADR |
|-------|-----------|-------|-----|
| Control | WebSocket (WSS + JWT), sync ACK | Bidirektionale Steuerung, <100ms | ADR-004/010/012b |
| Telemetry | MQTT (Mosquitto), async | Fahrzeugstatus pub/sub | ADR-003/012b |
| Safety | Safety Event Bus (Go, In-Memory), async | Safety-Events; DDS-Interface-kompatibel | ADR-002/012b |
| Video | WebRTC SFU (Pion/Go) + coturn | Multi-Kamera, Adaptive Bitrate, NAT Traversal | ADR-014 |
| Message Format | Protobuf (Application Bus) | Control/Safety/Telemetry + CorrelationHeader | ADR-008/016 |
| WebRTC Signaling | SDP/ICE (WebRTC Standard) | Media Layer — bewusst außerhalb Protobuf | ADR-014 |
| Session Correlation | ULID (Vehicle-ID→Session-ID→Event-ID) | Systemweite Korrelation aller Kanäle | ADR-016 |

> DDS wird in dieser Phase NICHT implementiert. Der Safety Event Bus ist als DDS-Drop-in-Replacement designed.

---

## Systemtopologie (ADR-007)

**Zwei orthogonale Hubs — CONTROL HUB > VIDEO HUB (strikte Hierarchie):**

```
CONTROL HUB (Rang 1 — Safety Truth)
┌─────────────────────────────────┐
│        Control Server           │
│  · State Machine (4-Layer)      │
│  · Safety Decision Engine       │
│  · Session Manager (GSA)        │
│  · SFU Event Publisher          │
└───────┬──────────┬──────────────┘
        │          │
 Auth Service   Safety Bus   MQTT Telemetry
                              (Mosquitto)

VIDEO HUB (Rang 2 — Awareness only)
┌─────────────────────────────────┐
│        WebRTC SFU (Pion)        │
│  · Media Relay (Primary/Sec.)   │
│  · Server-seitiges Recording    │
│  · Multi-Operator Forwarding    │
│  · Session Event Consumer       │
└───────────────┬─────────────────┘
                │ NAT Traversal
           coturn (STUN/TURN)
```

---

## Session Correlation ID (ADR-016)

```
Format:    ULID (zeitlich sortierbar, URL-safe, distributed-safe)
Erzeuger:  Control Server (GSA), Zeitpunkt: CONNECTING → CONNECTED

Hierarchie:
  Vehicle-ID
    └── Session-ID (ULID — Root Anchor, überlebt SAFE_MODE)
          └── Event-ID (ULID — pro Message/Command/Frame)

JWT = Identity (Wer bist du?) ≠ Session-ID (In welchem Kontext?)
```

---

## Backend & Tech Decisions (entschieden)

| Komponente | Technologie | ADR |
|------------|-------------|-----|
| Backend-Services | Go | ADR-001 |
| Test-Framework (Go) | testing + testify | ADR-006 |
| Frontend | React 18+ + TypeScript + Vite + Tailwind CSS + Shadcn/ui | ADR-013 |
| Message Format | Protobuf (Application Bus) | ADR-008 |
| Protobuf Code-Gen | protoc-gen-es (TS) + protoc-gen-go, build-time | ADR-012b/013 |
| MQTT Broker | Eclipse Mosquitto | ADR-003 |
| Video | WebRTC SFU (Pion/Go) + coturn | ADR-014 |
| Authentifizierung | Separater Auth Service (JWT, Operator + Vehicle) | ADR-004 |
| Session-ID | ULID, generiert vom Control Server (GSA) | ADR-016 |
| Session Recording | Abstraktes Interface (Storage noch offen) | ADR-005 |
| Deployment | Docker Compose (kein Kubernetes) | CONTEXT |

---

## Control Loop Definition (ADR-010)

- **Modell:** Event-driven Stream — Operator-Inputs als kontinuierlicher WebSocket-Event-Stream
- **Latenz-Messpunkt:** Client → Server → ACK zurück (Roundtrip, <100ms = CI Build-Fail)
- **Safety Override:** Channel Close — WebSocket bei CRITICAL Event geschlossen
- **Konsequenz:** Frontend muss Reconnect-Logik nach Channel Close implementieren

---

## Protobuf Versioning & Code Generation (ADR-012/012b)

- **Versioning:** Field-based — keine Field IDs ändern, keine Felder entfernen, neue Features als optionale Felder
- **CI:** Schema-Kompatibilitätsprüfung als Pflicht-Gate
- **Code-Gen:** Build-time via `protoc`, gitignored — `.proto` ist Single Source of Truth
- **Frontend:** protoc-gen-es (TypeScript-Klassen, ADR-013)

---

## Sync/Async-Modell (ADR-012b)

| Kanal | Modell |
|-------|--------|
| Control Server → Safety Event Bus | Async (fire-and-forget) |
| Control Server → Auth Service | Async + lokale JWT-Validierung |
| Frontend → Control Server | Synchron ACK-based (non-blocking UI) |
| Control Server → MQTT Telemetry | Async (one-way) |
| Control Server → SFU | Async Event-Push (SESSION_* Events) |

---

## Assumptions

- Netzwerk ist nicht stabil — Vehicle ↔ Internet ↔ OCC (uncontrolled routing)
- Latenz schwankt stark, kann jederzeit >100ms steigen
- NAT, CGNAT, Firewalls können UDP blockieren — WebRTC mit STUN/TURN löst das
- Kein automatisches Resume nach CRITICAL Failure — Operator-Ack immer erforderlich
- DDS wird in dieser Phase nicht eingesetzt

---

## Risks

| Risiko | Mitigierung |
|--------|-------------|
| Netzwerk-Latenz >100ms | CI-Latenz-Tests, <100ms Build-Fail (ADR-006) |
| NAT/CGNAT blockiert Video | WebRTC + coturn (STUN/TURN) — ADR-014 |
| Safety Event Bus kein echtes DDS | DDS-kompatibles Interface, Austausch per ADR-002 |
| Control Server als Single Point of Failure | Deployment-Strategie als zukünftiges ADR |
| Stale Command Execution nach Reconnect | SAFE MODE + Operator-Ack + Channel Close (ADR-009/010) |
| Inkonsistente Message-Strukturen | Protobuf + CI Schema-Gate (ADR-008/012) |
| Control Hub / Video Hub Zustandsdrift | GSA als Single Source of Truth, SFU Event-Push (ADR-015) |
| WebRTC CI-Flakiness | Non-blocking E2E Tests, Retry 3×, threshold-based (ADR-006) |
| Session Korrelationsverlust bei SAFE_MODE | Session-ID überlebt SAFE_MODE als Root-Anchor (ADR-016) |

---

## Offene Fragen

| Frage | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (Channels vs. Header-Flag) | TEST-05 / Sprint 6 | ADR-008 Folge — BE-04 nutzt Publisher-Pattern; explizite Channel-Trennung noch offen |
| Session Recording Storage (DB / Files / Object Storage) | nach Sprint 6 | ADR-005 Folge — MemoryRecorder seit Sprint 4 als Platzhalter aktiv; Storage-ADR ausstehend |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store (SQLite Volume) | nach LOG-10 | ADR-019 möglich — SQLite-Volume-Sicherung für Produktivbetrieb |

---

## Architekturprinzipien

1. **Safety First** — SAFE MODE ist nicht überbrückbar; Media darf Safety nie triggern
2. **Explicit Failure Model** — jeder Fehlerfall klassifiziert und deterministisch behandelt (ADR-009)
3. **Schema-First** — Protobuf vor Implementierung, kein implizites Nachrichtenformat (ADR-008)
4. **Control-plane-driven** — Control Server = Brain; SFU = Reflex-System; Vehicle = Execution Layer
5. **Hub-Hierarchie** — CONTROL HUB > VIDEO HUB; Video nie in Safety-Entscheidungen (ADR-007)
6. **Interface over Implementation** — Safety Bus, Session Recording, SFU gegen Interfaces (ADR-002/005/015)
7. **No Auto-Resume** — nach CRITICAL Failure immer Operator-Ack (ADR-009/010)
8. **Global Session Authority** — Control Server ist einziger Session-Erzeuger und Single Source of Truth (ADR-015)
9. **Correlation-First** — alle Kanäle tragen Session-ID (ULID); vollständige Rekonstruierbarkeit (ADR-016)
10. **Hybrid Logging** — Technical Logs async (slog → Loki); Safety Events sync (AuditWriter.WriteSync + fsync vor SAFE_MODE); drei Log-Klassen (ADR-017/018)
11. **Safety-Log-Garantie** — Safety Events dürfen niemals verloren gehen; separater SQLite Audit Store orthogonal zu Loki (ADR-018)
