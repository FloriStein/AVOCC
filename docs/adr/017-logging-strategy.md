# ADR-017: Logging Strategy — Strukturiertes Logging mit Loki + Grafana

Status: Accepted

## Kontext

Das System hat vier Kommunikationskanäle (WebSocket/Control, MQTT/Telemetry, Safety Bus, WebRTC/Video),
sechs Backend-Services und ein React-Frontend. Alle Komponenten loggen aktuell mit `log.Printf` als
unstrukturierten Klartext. Eine Session-Rekonstruktion über mehrere Services hinweg ist damit nicht möglich.

### Primäre Anforderungen

- **Auditierung:** Wer hat wann was gesteuert? (Operator-Aktionen, Safety-Events, Session-Lifecycle)
- **Incident-Analyse:** Nach einem SAFE_MODE-Event muss der vollständige Kausalkettennachweis erbracht werden
- **Session-Korrelation:** Alle Log-Einträge müssen über `session_id` (ULID, ADR-016) verknüpfbar sein
- **Vollständigkeit:** Frontend-, Backend-, Safety-, Telemetry- und Video-Ereignisse müssen gemeinsam abfragbar sein

### Sekundäre Anforderungen

- Observability / Monitoring (Grundlage für spätere Dashboards)
- Debugging-Unterstützung

---

## Analysierte Optionen

### Option A: stdout only (plain text / JSON)

**Vorteile:**
- Kein neuer Service
- `docker logs` + `grep` immer verfügbar

**Nachteile:**
- Session-Korrelation über mehrere Services erfordert manuelle `grep`-Ketten
- Keine zeitliche Visualisierung
- Skaliert nicht für Auditierung und Incident-Analyse

### Option B: Grafana Loki + Grafana + Promtail (entschieden)

**Vorteile:**
- LogQL-Queries: `{session_id="01JTXY..."} |= "SAFE_MODE"` über alle Services gleichzeitig
- Timeline-Visualisierung einer Session in Grafana
- Etablierte Container, lokal betreibbar
- Geringe Komplexität: +2 Docker-Compose-Services (Loki + Grafana), Promtail läuft als Sidecar
- Docker-Label-Discovery: Promtail erkennt neue Services automatisch
- Keine externe Cloud-Abhängigkeit

**Nachteile:**
- +2 Docker-Compose-Services (Loki, Grafana)
- Loki ist kein revisionssicherer Audit-Store (Folge-ADR-018 erforderlich)
- Promtail-Konfiguration initial aufwendig

### Option C: Elasticsearch + Kibana

**Nachteile:**
- Ressourcenintensiv (Elasticsearch: min. 1 GB RAM)
- Unnötig komplex für den aktuellen Scope
- Kein Business Value gegenüber Option B

### Option D: Eigenentwicklung

**Nachteile:**
- Kein Business Value
- Logging-Infrastruktur neu erfinden lohnt nicht

---

## Entscheidung

Wir wählen **Option B: JSON-strukturiertes Logging → stdout → Docker → Promtail → Loki → Grafana**,
erweitert um einen **separaten Audit Store** (ADR-018) für Safety-Events mit garantierter Persistenz.

---

## Log-Klassen-Taxonomie

Das System unterscheidet drei Klassen von Log-Einträgen:

| Klasse | Zweck | Verlust erlaubt | Ziel |
|--------|-------|-----------------|------|
| **Technical Log** | Debugging, Betrieb, Verbose | Ja | Loki (TTL 30 Tage) |
| **Audit Log** | Nachvollziehbarkeit, Operator-Aktionen | Nein | Loki + Audit Store (ADR-018) |
| **Safety Event** | Safety-Nachweis, CRITICAL-Trigger | Nein, niemals | Audit Store (synchron) + Loki |

**Safety Events** (`EMERGENCY_STOP`, `DEADMAN_TIMEOUT`, `SAFE_MODE_ENTERED`, `SAFE_MODE_EXITED`,
`COMMAND_ACK_TIMEOUT`, `SAFETY_BUS_FAILURE`, `OPERATOR_HANDOVER`, `SESSION_START`, `SESSION_END`)
müssen **vor** dem Abschluss der Safety-Transition in den Audit Store geschrieben werden (synchron + fsync).
Loki erhält sie zusätzlich für Analyse und Visualisierung.

---

## Architektur

### Sync/Async-Modell (Hybrid — ADR-010-konform)

Das `<100ms` Control-Loop-Budget (ADR-010) darf durch Logging nicht gefährdet werden.
Gleichzeitig dürfen Safety Events niemals verloren gehen. Deshalb: Hybrid.

```
                    ┌───────────────────┐
                    │  Control Server   │
                    └─────────┬─────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼

      Technical Logger                 Safety Logger
    (async — fire-and-forget)          (sync — blocking)

              │                               │
              ▼                               ▼

     JSON → stdout → Docker          AuditWriter.WriteSync()
                                      (fsync vor SAFE_MODE)
              │                               │
              ▼                               ▼

         Loki/Grafana                Audit Store (ADR-018)
                                      + Loki (zusätzlich)
```

| Log-Klasse | Pfad | Latenz-Impact | Beispiele |
|------------|------|---------------|-----------|
| Technical Log | Async stdout | ~0ms | WS_CONNECTED, MQTT_MESSAGE, VIDEO_STATE |
| Audit Log | Async stdout | ~0ms | STATE_TRANSITION, COMMAND_RECEIVED |
| **Safety Event** | **Sync** (WriteSync + fsync) | ~1–5ms | EMERGENCY_STOP, SAFE_MODE_ENTERED, DEADMAN_TIMEOUT |

**Invariante:** SAFE_MODE-Transition wird erst nach erfolgreichem `AuditWriter.WriteSync()` abgeschlossen.

### AuditWriter Interface (Interface-first, konsistent mit ADR-005)

Analog zum `SessionRecorder`-Interface (ADR-005) wird ein `AuditWriter`-Interface definiert.
Die konkrete Persistenzstrategie entscheidet ADR-018 — der Control Server kennt nur das Interface.

```go
// pkg/audit/writer.go
type SafetyAuditEvent struct {
    EventID     string
    SessionID   string
    VehicleID   string
    OperatorID  string
    EventType   string         // typisiert — aus Event-Type-Katalog
    Reason      string
    SystemState string
    CtrlState   string
    Timestamp   time.Time
    Data        map[string]any // optionaler Kontext
}

type AuditWriter interface {
    WriteSync(event SafetyAuditEvent) error  // blockiert bis Durability garantiert
    Close() error
}

// NoopWriter für Tests und Entwicklung ohne persistenten Store
type NoopWriter struct{}
func (n *NoopWriter) WriteSync(_ SafetyAuditEvent) error { return nil }
func (n *NoopWriter) Close() error                       { return nil }
```

### Dual-Path Log-Pipeline

```
Go Services        React Frontend
    │                   │
slog (JSON/stdout)  POST /log API
    │                   │
    └───────── Docker ──┘
                   │
               Promtail
            (Docker Discovery)
                   │
                 Loki
                   │
               Grafana
          (LogQL, Dashboards)
```

### Frontend Log-Ingestion

Das Frontend sendet Logs **nicht direkt** an Loki, sondern über den Control Server:

```
Browser
  └─→ POST /api/log  (Control Server)
            └─→ slog JSON → stdout → Docker → Loki
```

**Begründung:**
- Authentifizierung bleibt zentral (JWT + Session-Kontext)
- `session_id` wird server-seitig validiert — kein Spoofing möglich
- Audit-Trail manipulationssicherer

### Pflichtfelder pro Log-Eintrag

```json
{
  "time":        "2026-06-03T19:00:00.000Z",
  "level":       "INFO",
  "service":     "control-server",
  "session_id":  "01JTXYZ...",
  "event_id":    "01JTXYZ...",
  "vehicle_id":  "vehicle-1",
  "operator_id": "operator-1",
  "event_type":  "SAFE_MODE_ENTERED",
  "msg":         "Dead-man timeout — CRITICAL → SAFE_MODE"
}
```

- `session_id` und `event_id` sind ULIDs (ADR-016)
- `event_type` ist ein typisiertes Enum (Liste unten) — kein Freitext
- Optionale Felder: `data` (strukturiertes Objekt für zusätzliche Kontextdaten)

### Event-Type-Katalog

| Kategorie | Event Types |
|-----------|------------|
| **Session** | `SESSION_STARTED`, `SESSION_ENDED`, `SESSION_CHECKPOINT_SAVED`, `SESSION_RECOVERED` |
| **System State** | `STATE_TRANSITION_SYSTEM`, `STATE_TRANSITION_CONTROL`, `STATE_TRANSITION_MEDIA`, `STATE_TRANSITION_OPERATOR` |
| **Safety** | `EMERGENCY_STOP`, `DEADMAN_TIMEOUT`, `DEADMAN_ARMED`, `ACK_TIMEOUT`, `WS_DISCONNECT_CRITICAL`, `SAFETY_BUS_FAILURE`, `NO_OPERATOR_CRITICAL` |
| **Operator** | `OPERATOR_ACK`, `OPERATOR_HANDOVER_REQUESTED`, `OPERATOR_HANDOVER_CONFIRMED`, `OPERATOR_HANDOVER_CANCELLED` |
| **Commands** | `COMMAND_STEER`, `COMMAND_THROTTLE`, `COMMAND_BRAKE`, `COMMAND_SPEED`, `COMMAND_RATE_LIMITED` |
| **Media** | `MEDIA_STATE_CHANGE`, `MEDIA_FAILED`, `MEDIA_DEGRADED`, `MEDIA_CONNECTED` |
| **Vehicle** | `VEHICLE_CONNECTED`, `VEHICLE_DISCONNECTED` |
| **Recording** | `RECORDING_STARTED`, `RECORDING_ENDED` |
| **Frontend** | `FE_EMERGENCY_STOP_CLICKED`, `FE_DEADMAN_HOLD`, `FE_DEADMAN_RELEASE`, `FE_WEBRTC_STATE_CHANGE`, `FE_WS_RECONNECT`, `FE_OPERATOR_ACK_CLICKED`, `FE_SESSION_VISIBLE` |
| **System** | `SERVICE_STARTED`, `SERVICE_HEALTH_CHECK` |

### Log-Level-Semantik

| Level | Verwendung |
|-------|------------|
| `ERROR` | CRITICAL Failures, Service-Crashes |
| `WARN` | DEGRADED State, Verbindungsprobleme, Rate-Limiting |
| `INFO` | Normale Operationen: Session-Lifecycle, State-Transitions, Operator-Aktionen |
| `DEBUG` | Verbose: einzelne Commands, Heartbeats (default: deaktiviert) |

### Go-Implementierung: `pkg/logger`

Gemeinsames Logger-Package für alle Go-Services basierend auf `log/slog` (Go Standardbibliothek seit 1.21 — keine externe Abhängigkeit):

```go
// pkg/logger/logger.go
log := logger.New("control-server")
log.Event(sessionID, vehicleID, operatorID, eventID, "SAFE_MODE_ENTERED",
    "Dead-man timeout — CRITICAL → SAFE_MODE",
    "reason", "DEADMAN_TIMEOUT",
    "timeout_ms", 10000,
)
```

### Latenz-Impact auf Control Loop

`slog` schreibt gepuffert nach `stdout` (Betriebssystem-Puffer). Docker und Promtail laufen asynchron.
Gemessener Overhead: `<1ms` — weit unterhalb des `<100ms` ACK-Roundtrip-Budgets (ADR-010).

---

## Infra-Konfiguration

### Loki
- Retention: 30 Tage (konfigurierbar)
- Storage: lokales Filesystem (Docker Volume)
- Schema: TSDB (Loki v3 default)

### Promtail
- Discovery: Docker-Label-basiert (`__path__` aus Container-Metadata)
- Parsing: JSON Pipeline — `json` Stage extrahiert `session_id`, `event_type`, `level` als Loki-Labels
- Labels (für LogQL): `{service="control-server", session_id="01J..."}` 

### Grafana
- Port: 3001 (3000 ist Frontend)
- Provisioning: Loki Datasource + AVOC Dashboard automatisch beim Start
- Anmeldung: `admin/avoc` (lokal, kein Produktiv-Einsatz)

---

## Folge-ADR

**ADR-018: Audit Trail Strategy** — definiert den Audit Store für Safety Events und Audit Logs.
Loki bleibt für Technical Logs und Analyse; der Audit Store garantiert revisionssichere Persistenz.
→ Beide ADRs bilden gemeinsam die vollständige Logging-Strategie.

---

## Konsequenzen

### Positiv

- Vollständige Session-Rekonstruktion über alle Kanäle via `session_id`
- LogQL-Abfragen: `{session_id="01J..."} | json | event_type="EMERGENCY_STOP"`
- Timeline-Visualisierung in Grafana (wer hat wann was getan)
- Nullstellige externe Abhängigkeit (lokal betreibbar)
- Go `slog` ohne externe Dependency — konsistent mit ADR-001 (Abhängigkeiten minimieren)
- Latenz-neutral: `<1ms` overhead, async via stdout

### Negativ

- +2 Docker-Compose-Services (Loki + Grafana) — Stack wächst auf 10 Services
- Loki ist kein Audit-Store — ADR-018 erforderlich für Safety-Garantie
- Migration aller bestehenden `log.Printf` Calls in 6 Go-Services (einmalig)
- Frontend-Logs über Control Server: kleiner Overhead pro Log-Event (`POST /api/log`)
