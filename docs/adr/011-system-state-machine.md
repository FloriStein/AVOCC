# ADR-011: System State Machine (4-Layer Model)

Status: Accepted (erweitert durch ADR-014)

## Kontext

Die ursprüngliche State Machine definierte 7 Zustände in einer einzigen Maschine. Mit der Einführung von WebRTC (ADR-014), Multi-Operator-Handover und dem Klarheitsbedarf zwischen Safety-State, Control-State, Video-State und Operator-State ist eine Erweiterung auf 4 orthogonale State Machines notwendig. Das Antipattern "eine State Machine für alles" wird explizit vermieden.

## Architekturprinzip

```
┌──────────────────────────────┐
│ 1. SYSTEM STATE (Safety Core)│  ← Safety Truth, Master State
├──────────────────────────────┤
│ 2. CONTROL STATE             │  ← Command Flow, abhängig von System State
├──────────────────────────────┤
│ 3. MEDIA STATE               │  ← WebRTC/Video Health, unabhängig
└──────────────────────────────┘
         ↑ alle beeinflusst von ↑
┌──────────────────────────────┐
│ 4. OPERATOR STATE            │  ← Human Governance
└──────────────────────────────┘
```

**Abhängigkeitsregel:** SYSTEM STATE ist die Safety Truth. Alle anderen State Machines sind davon abhängig — niemals umgekehrt.

---

## 1. SYSTEM STATE (Safety Core — Master)

### Zustände

| Zustand | Beschreibung |
|---------|-------------|
| `IDLE` | System inaktiv, keine Session aktiv |
| `CONNECTING` | Verbindungsaufbau (WebSocket + Initial Handshake) |
| `AUTHENTICATED` | JWT gültig, Identität verifiziert — noch keine Control-Freigabe |
| `CONNECTED` | Volle Control-Berechtigung aktiv |
| `DEGRADED` | Teilausfall (Video/Telemetrie) — Steuerung möglich, Awareness reduziert |
| `SAFE_MODE` | CRITICAL Failure — Fahrzeug steht, Control blockiert, Channel geschlossen |
| `RECOVERING` | Reconnect läuft, Systemzustand wird validiert |

### SAFE_MODE Trigger (CRITICAL)

- WebSocket Control Disconnect
- Safety Event Bus Failure
- Dead-man Switch Timeout
- Auth Invalidation (laufende Session)
- Command ACK Timeout (kritisch)

### DEGRADED Trigger (nicht kritisch)

- Video Lost (MEDIA_FAILED → DEGRADED)
- Partial Telemetry Loss
- Secondary Camera Failure

### Transition Rules

```
Normal Flow:
  IDLE → CONNECTING → AUTHENTICATED → CONNECTED

Failure Transitions:
  CONNECTED    → DEGRADED    (DEGRADED Trigger)
  CONNECTED    → SAFE_MODE   (CRITICAL Trigger)
  DEGRADED     → SAFE_MODE   (weiterer CRITICAL Trigger)

Recovery:
  SAFE_MODE    → RECOVERING
  RECOVERING   → AUTHENTICATED → CONNECTED (Validierung + Operator-Ack)

Hard Failures:
  CONNECTING   → SAFE_MODE   (Timeout / Auth Failure)
  RECOVERING   → SAFE_MODE   (Validierung fehlgeschlagen)
```

---

## 2. CONTROL STATE (WebSocket / Command Flow)

**Abhängig von SYSTEM STATE — nie umgekehrt.**

### Zustände

| Zustand | Beschreibung |
|---------|-------------|
| `CONTROL_INIT` | Control Channel initialisiert |
| `CONTROL_ACTIVE` | Commands werden akzeptiert und ausgeführt |
| `CONTROL_BLOCKED` | SAFE_MODE Overlay — keine Commands möglich |
| `CONTROL_LOST` | WebSocket unterbrochen |
| `CONTROL_RECOVERING` | Reconnect läuft |

### Abhängigkeitsregel

```
SYSTEM SAFE_MODE  ⇒  CONTROL_BLOCKED
VIDEO_LOSS        ⇏  kein CONTROL STATE CHANGE
```

---

## 3. MEDIA STATE (WebRTC / Video Layer)

**Unabhängig von CONTROL STATE. Beeinflusst nur SYSTEM DEGRADED.**

### Zustände

| Zustand | Beschreibung |
|---------|-------------|
| `MEDIA_INIT` | WebRTC Initialisierung |
| `MEDIA_NEGOTIATING` | SDP/ICE Exchange läuft |
| `MEDIA_CONNECTED` | Video Stream aktiv |
| `MEDIA_DEGRADED` | Qualitätsverlust, Stream instabil |
| `MEDIA_FAILED` | Video Stream verloren |

### Mapping auf SYSTEM STATE

| Media State | System Impact |
|-------------|--------------|
| `MEDIA_CONNECTED` | Normal — kein Impact |
| `MEDIA_DEGRADED` | SYSTEM → DEGRADED |
| `MEDIA_FAILED` | SYSTEM → DEGRADED |

**Regel: Kein SAFE_MODE durch Media State. Video = Awareness only.**

---

## 4. OPERATOR STATE (Human Governance)

### Zustände

| Zustand | Beschreibung |
|---------|-------------|
| `NO_OPERATOR` | Keine Operator-Session aktiv |
| `OPERATOR_ASSIGNED` | Operator authentifiziert, noch kein Active Status |
| `ACTIVE_OPERATOR` | Volle Steuerungsberechtigung, exklusiv |
| `HANDOVER_PENDING` | Steuerungsübergabe läuft |
| `RECOVERING_OPERATOR` | Operator reconnectet nach Verbindungsverlust |

### Governance-Regeln

- Maximal **1 ACTIVE_OPERATOR** pro Session
- Weitere Operatoren sind OBSERVER oder STANDBY
- `NO_OPERATOR` → SYSTEM SAFE_MODE (kein Operator = kein Control)
- `ACTIVE_OPERATOR_LOST` → SYSTEM RECOVERING

---

## Gesamtintegration (Master View)

```
                ┌─────────────────────┐
                │   OPERATOR STATE    │
                │  (Human Governance) │
                └─────────┬───────────┘
                          │ beeinflusst
                          ▼
┌────────────────────────────────────────────┐
│         SYSTEM STATE (Safety Truth)        │
│                                            │
│  IDLE → CONNECTING → AUTHENTICATED         │
│              ↓                             │
│          CONNECTED ──────────── DEGRADED   │
│              ↓                      ↓      │
│           SAFE_MODE ←───────────────┘      │
│              ↓                             │
│          RECOVERING → AUTHENTICATED        │
└───────────────┬────────────────────────────┘
                │ bestimmt
     ┌──────────┴──────────┐
     ▼                     ▼
CONTROL STATE         MEDIA STATE
(Command Flow)        (WebRTC Health)
CONTROL_INIT          MEDIA_INIT
CONTROL_ACTIVE        MEDIA_NEGOTIATING
CONTROL_BLOCKED ←     MEDIA_CONNECTED
(bei SAFE_MODE)       MEDIA_DEGRADED → DEGRADED
CONTROL_LOST          MEDIA_FAILED  → DEGRADED
CONTROL_RECOVERING
```

---

## Formale System-Invarianten

```
INVARIANT 1:
  Media Layer SHALL NOT influence SAFE_MODE transitions
  except via DEGRADED annotation evaluated by the Control Hub.

INVARIANT 2:
  SAFE_MODE transitions are exclusively triggered by Control,
  Safety Bus, or Operator-level failures. Never by Media events.

INVARIANT 3 (Hub-Hierarchie — ADR-007):
  Control Hub (Rang 1) > Video Hub (Rang 2).
  Control Hub ist Single Source of Truth für Session State.
  Bei widersprüchlichen Zuständen gilt Control Hub als autoritativ.
```

## Safety Rules

**Regel 1 — Safety Override**
`SAFE_MODE` überschreibt alles. CONTROL_BLOCKED. Media läuft weiter (optional). Operator UI read-only.

**Regel 2 — Media darf niemals Safety triggern**
`MEDIA_FAILED` → SYSTEM DEGRADED, niemals SAFE_MODE. (Invariante 1 + 2)

**Regel 3 — Control ist einzig sicherheitskritischer Kanal**
Control + Safety Bus = System Safety. Video = Awareness only.

**Regel 4 — Recovery Sequence (ADR-009)**
```
RECOVERING:
  1. reconnect control channel
  2. validate safety bus
  3. operator ACK
  4. restore control (CONNECTED)
```

## Konsequenzen

### Positiv:
- Klare Trennung: Safety Truth, Command Flow, Video Health, Human Governance
- Safety Test Suite kann SYSTEM STATE isoliert testen
- WebRTC-Fehler beeinflussen nie Safety-kritische Pfade
- Multi-Operator-Handover ist explizit modelliert

### Negativ:
- 4 State Machines erhöhen Implementierungskomplexität im Control Server
- Zustandskombinationen müssen in Tests durchgespielt werden
