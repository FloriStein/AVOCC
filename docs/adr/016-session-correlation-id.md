# ADR-016: Session Correlation ID Standard

Status: Accepted

## Kontext

Das System kommuniziert über mehrere parallele Protokollwelten (WebSocket/Protobuf, WebRTC/SDP, MQTT, Safety Event Bus, Session Recording). Ohne einen systemweiten Korrelationsstandard ist es unmöglich, Control-Events mit Video-Events, Safety-Events oder Telemetriedaten zu verknüpfen — sowohl in Produktion als auch bei Safety Audits, Debugging und Incident Reconstruction. ADR-016 definiert den einheitlichen Correlation ID Standard für alle Kanäle.

---

## Kernentscheidungen

### 1. Primäres Korrelationsmittel: Session-ID

Die **Session-ID** (Control Session — ADR-015) ist der einzige globale Korridor über alle Systeme hinweg. Alle Kanäle tragen dieselbe Session-ID als primären Anker.

**Kernregel:** *Eine Session-ID pro Control Session ist der einzige globale Korridor über alle Systeme hinweg. Alles andere ist lokal.*

### 2. Hierarchisches Identifier-Modell

```
Vehicle-ID          → physische Einheit (stabil, aus Vehicle Registry)
  └── Session-ID    → Control Session (ULID, generiert vom Control Server)
        └── Event-ID → einzelne Nachricht / Command / Frame (ULID, lokal generiert)
```

Optional im Event:
```
  └── Operator-ID   → Identity Layer (aus JWT, wechselbar innerhalb Session)
```

### 3. JWT = Identity, nicht Session Context

Das JWT enthält `sub` (Operator Identity), Rollen und Expiry — aber **keine Session-ID**. JWT beantwortet: *"Wer bist du?"* Die Session-ID beantwortet: *"In welchem aktiven Steuerkontext bist du gerade?"*

---

## Kanal-Mapping

| Kanal | Trägt Session-ID | Zusätzliche IDs | Notizen |
|-------|-----------------|-----------------|---------|
| WebSocket Control (Protobuf) | ✅ Session-ID | Event-ID | In jedem Protobuf-Message-Header |
| WebRTC Signaling (SDP/ICE) | ✅ Session-ID | Stream-ID | Im Signaling-Wrapper (WebSocket) |
| MQTT Telemetry | ✅ Session-ID | Vehicle-ID | Im Topic oder Message Header |
| Safety Event Bus | ✅ Session-ID | Safety-Event-ID | In jedem Safety Event |
| Session Recording | ✅ Session-ID | — | Root Key für alle Recording-Daten |
| SFU Session Events | ✅ Session-ID | — | Event Stream Control → SFU (ADR-015) |
| Auth JWT | ❌ | Operator-ID only | Identity Layer, kein Session Context |

---

## Event-Struktur (Standard)

Jeder Event in allen Kanälen enthält mindestens:

```json
{
  "session_id":   "01HZYT8K3J9QW3X6R8M2F1V9AB",
  "event_id":     "01HZYT8K3J9QW3X6R8M2F1V9AC",
  "vehicle_id":   "VHL-042",
  "operator_id":  "OPR-101",
  "timestamp":    "2026-06-03T14:32:00.123Z"
}
```

In Protobuf-Schemas (control.proto, safety.proto, telemetry.proto, session.proto) wird ein gemeinsamer `CorrelationHeader` definiert:

```protobuf
message CorrelationHeader {
  string session_id  = 1;
  string event_id    = 2;
  string vehicle_id  = 3;
  string operator_id = 4;
  int64  timestamp   = 5;
}
```

---

## Format: ULID

**Entscheidung:** ULID (Universally Unique Lexicographically Sortable Identifier)

### Begründung

| Eigenschaft | UUID v4 | ULID | Hierarchisch |
|-------------|---------|------|--------------|
| Zeitlich sortierbar | ❌ | ✅ | ✅ |
| URL-safe | ✅ | ✅ | ⚠️ |
| Distributed-safe | ✅ | ✅ | ❌ |
| Log-Korrelation | mittel | sehr gut | gut |
| Global eindeutig | ✅ | ✅ | ❌ |
| Debugging (Timeline) | ❌ | ✅ | ✅ |

ULID ist zeitstempel-sortierbar (Millisekunden-Präfix) — damit ist die chronologische Rekonstruktion einer Session aus Logs ohne expliziten Timestamp möglich.

Hierarchische IDs (`vehicle-42/session-7`) werden abgelehnt: nicht global eindeutig, schwer indexierbar, koppeln Identifier an Domain-Struktur.

---

## Erzeuger und Zeitpunkt

**Erzeuger:** Control Server (Global Session Authority — ADR-015)

**Zeitpunkt:** Exakt beim Übergang `CONNECTING → CONNECTED`
- Vehicle verbunden ✅
- Operator authentifiziert ✅
- Safety Bus OK ✅
- Session validiert ✅
→ **Erst dann existiert eine echte Control Session, erst dann wird die Session-ID generiert**

**Warum nicht Auth Service:** Auth ≠ Session. Ein Login kann mehrere Sessions enthalten, Operator kann wechseln. Falsche Granularität.

**Warum nicht Vehicle:** Vehicle hat keine globale System-Sicht. NAT/5G-Reconnects erzeugen mehrfaches Connect/Disconnect → Split-Brain-Risiko.

---

## SAFE_MODE / Recovery Kontinuität

Die Session-ID überlebt die SAFE_MODE-Grenze als Root-Referenz:

```
CONNECTED (Session-ID: S-001)
    ↓ CRITICAL Failure
SAFE_MODE (Checkpoint enthält S-001)
    ↓ Recovery
RECOVERING (lädt Checkpoint mit S-001)
    ↓ Operator-Ack
CONNECTED (neue Execution Branch, Root: S-001)
```

Recovery = neue Execution Branch unter derselben Root-Correlation. Die Session-ID ändert sich nicht — aber jede Post-Recovery-Event trägt eine neue Event-ID.

**Handover:** Operator-Wechsel erzeugt einen neuen Correlation Sub-Tree:
```
Session-ID: S-001
  ├── Operator: OPR-101 (vor Handover)
  └── Operator: OPR-202 (nach Handover, neue Event-ID-Reihe)
```

---

## Systemweite Vorteile

Mit diesem Modell wird möglich:
- Vollständige Session-Rekonstruktion (Control + Video + Safety + Telemetry)
- Safety Incident Analyse: Control Event → zeitgleicher Video Frame → Safety Event
- Handover historisch nachvollziehen
- Performance-Analyse über alle Kanäle
- CI Replay-Tests für Safety Szenarien

---

## Architektur-Pattern (Zusammenfassung)

```
Auth Service    → Identity (JWT: Wer bist du?)
Control Server  → Reality (Session-ID: In welchem Kontext?)
Session-ID      → Truth Anchor (verbindet alle Protokollwelten)
```

## Konsequenzen

### Positiv:
- Vollständig rekonstruierbares Teleoperation-System
- Safety Audits und Incident Reconstruction werden möglich
- Einheitliche Korrelation über alle Protokoll-Welten (Protobuf, WebRTC, MQTT)
- SAFE_MODE / Recovery ohne Korrelationsverlust

### Negativ:
- `CorrelationHeader` muss in alle `.proto`-Schemas aufgenommen werden (INFRA-01 Erweiterung)
- ULID-Bibliothek für Go und TypeScript als Dependency erforderlich
- Event-ID muss pro Nachricht generiert werden — minimaler Overhead, aber explizit
