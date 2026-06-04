# ADR-018: Audit Trail Strategy — Garantierte Persistenz von Safety Events

Status: Accepted

## Kontext

ADR-017 entschied für Loki als Log-Aggregationssystem. Loki ist jedoch kein revisionssicherer
Audit-Store — Logs können durch TTL-Ablauf, Log-Rotation oder erhöhte Last verloren gehen.

Für ein Teleoperationssystem müssen folgende Fragen immer beantwortbar sein:

- Warum hat das Fahrzeug angehalten?
- Wer hat den Emergency Stop ausgelöst — Operator oder automatischer Safety-Trigger?
- Welche Session und welches Fahrzeug war betroffen?
- Welche Systemzustände lagen unmittelbar vor dem Event vor?
- Vollständiger Zeitstrahl einer Session für spätere Untersuchung

Diese Informationen dürfen unter **keinen Umständen** verloren gehen — unabhängig von Last,
Log-Rotation, Container-Neustart oder Loki-Ausfall.

---

## Log-Klassen (ADR-017)

| Klasse | Verlust | Ziel |
|--------|---------|------|
| Technical Log | Erlaubt | Loki |
| Audit Log | Nicht erlaubt | Loki + Audit Store |
| **Safety Event** | **Niemals** | **Audit Store (synchron) + Loki** |

### Safety Events (garantiert persistiert)

| Event Type | Auslöser |
|------------|---------|
| `EMERGENCY_STOP` | Operator-Klick oder automatischer Trigger |
| `DEADMAN_TIMEOUT` | Dead-man Switch nicht gehalten |
| `SAFE_MODE_ENTERED` | Jede CRITICAL-Transition → SAFE_MODE |
| `SAFE_MODE_EXITED` | Recovery → AUTHENTICATED |
| `COMMAND_ACK_TIMEOUT` | Control Loop Budget überschritten |
| `SAFETY_BUS_FAILURE` | Safety Event Bus nicht erreichbar |
| `WS_DISCONNECT_CRITICAL` | WebSocket-Disconnect → CRITICAL |
| `NO_OPERATOR_CRITICAL` | Kein Active Operator |
| `AUTH_INVALIDATION` | JWT-Invalidierung |
| `OPERATOR_HANDOVER_COMPLETED` | Aktiver Operator gewechselt |
| `SESSION_STARTED` | Control Session erstellt (CONNECTING → CONNECTED) |
| `SESSION_ENDED` | Control Session beendet |

---

## Optionen

### Option A: Append-only NDJSON-Datei

**Vorteile:**
- Zero dependencies, Go stdlib
- `jq`-Queries möglich

**Nachteile:**
- Keine strukturierten Queries über mehrere Sessions
- Kein Index — langsame Suche bei großen Logs
- Keine atomaren Writes (File-Rotation-Problematik)

### Option B: SQLite (embedded, kein zusätzlicher Service) ← Entschieden

**Vorteile:**
- Kein zusätzlicher Docker-Compose-Service
- SQL-Queries: `SELECT * FROM audit_events WHERE session_id=? ORDER BY timestamp`
- WAL-Modus: crash-sicher, Writes überstehen Container-Crash
- Append-Only per Tabellendesign (kein UPDATE/DELETE)
- `modernc.org/sqlite` — Pure Go, kein CGO, funktioniert in Alpine/Docker

**Nachteile:**
- Einzige externe Go-Dependency für diesen Task
- Schreibperformance durch fsync begrenzt (Safety Events: akzeptabel, niedrige Frequenz)

### Option C: PostgreSQL

**Nachteile:**
- Zusätzlicher Docker-Compose-Service
- Unnötige Komplexität für lokale Entwicklung
- Stack wächst von 10 auf 11 Services

### Option D: In-Memory (MemoryRecorder-Erweiterung)

**Nachteile:**
- Kein Crash-Schutz — Safety Events bei Container-Neustart verloren
- Erfüllt die Grundanforderung nicht

---

## Entscheidung

Wir wählen **Option B: SQLite mit WAL-Modus** als konkrete Implementierung des `AuditWriter`-Interfaces
(definiert in ADR-017). SQLite ist embedded — kein zusätzlicher Docker-Compose-Service erforderlich.

---

## Architektur

### Einordnung in ADR-017

ADR-017 definiert das `AuditWriter`-Interface und das Hybrid-Logging-Modell.
ADR-018 entscheidet ausschließlich, wie `AuditWriter` implementiert wird:

```
Safety Event
      │
      ├──→ AuditWriter.WriteSync()     ← ADR-017: Interface
      │         └──→ SQLiteAuditWriter ← ADR-018: Implementierung
      │               └──→ SQLite (WAL + fsync)
      │
      └──→ slog → stdout → Loki        ← async (ADR-017)
```

**Invariante:** SAFE_MODE-Transition erst nach erfolgreichem `WriteSync()`.
Schreibfehler = CRITICAL → ebenfalls SAFE_MODE.

### SQLite-Schema

```sql
CREATE TABLE IF NOT EXISTS audit_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id    TEXT NOT NULL UNIQUE,
    session_id  TEXT NOT NULL,
    vehicle_id  TEXT NOT NULL,
    operator_id TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    reason      TEXT,
    system_state TEXT,
    ctrl_state   TEXT,
    data        TEXT,          -- JSON
    timestamp   TEXT NOT NULL, -- ISO 8601
    written_at  TEXT NOT NULL  -- Server-seitiger Schreibzeitpunkt
);

CREATE INDEX idx_session ON audit_events(session_id);
CREATE INDEX idx_event_type ON audit_events(event_type);
CREATE INDEX idx_timestamp ON audit_events(timestamp);
```

### Storage-Pfad

```
/data/audit/avoc_audit.db    ← Docker Volume: audit-data
```

Der Control Server ist der einzige Schreiber (GSA-Prinzip, ADR-015).
SFU und andere Services schreiben Safety Events via Control Server-API (nicht direkt).

### Query-Beispiele (SQLite CLI / zukünftiges Dashboard)

```sql
-- Alle Safety Events einer Session
SELECT timestamp, event_type, reason, system_state
FROM audit_events
WHERE session_id = '01JTXYZ...'
ORDER BY timestamp;

-- Alle Emergency Stops der letzten 30 Tage
SELECT session_id, operator_id, vehicle_id, timestamp, reason
FROM audit_events
WHERE event_type = 'EMERGENCY_STOP'
  AND timestamp > datetime('now', '-30 days')
ORDER BY timestamp DESC;

-- Session-Rekonstruktion: vollständiger Zeitstrahl
SELECT * FROM audit_events
WHERE session_id = '01JTXYZ...'
ORDER BY timestamp;
```

---

## Integration mit bestehendem Session Recording (ADR-005)

Der bestehende `SessionRecorder` (MemoryRecorder, ADR-005) bleibt für Command-History
und State-Snapshots verantwortlich. Der Audit Store ist orthogonal dazu:

| Komponente | Verantwortung |
|------------|--------------|
| `SessionRecorder` (ADR-005) | Command-History, State-Snapshots, Session-Timeline |
| `AuditWriter` (ADR-018) | Safety Events, garantierte Persistenz, Compliance |

Beide können parallel existieren. Ein Audit-Event löst keinen Recording-Entry aus und umgekehrt.

---

## Konsequenzen

### Positiv

- Safety Events sind garantiert persistent — auch bei Loki-Ausfall oder Container-Crash
- SQL-Queries über session_id, event_type, Zeitraum
- Kein zusätzlicher Docker-Compose-Service (SQLite embedded)
- `modernc.org/sqlite` = Pure Go, kein CGO, Alpine-kompatibel
- WAL-Modus = Crash-safe, keine Datenverluste bei ungeplantem Shutdown
- Append-Only-Design verhindert nachträgliche Manipulation

### Negativ

- `modernc.org/sqlite` als neue Go-Dependency
- fsync bei jedem Safety Event = leichte Latenz (~1–5ms) — akzeptabel da Safety Events selten sind
- Kein automatisches Backup (ADR-019 möglich für Backup-Strategie)
- SQLite skaliert nicht horizontal — für Single-Instance-Betrieb (Docker Compose) ausreichend
