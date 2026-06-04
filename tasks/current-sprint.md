# Sprint 7 — Logging & Audit Trail

Ziel: Vollständig strukturiertes Logging. Safety Events garantiert persistent. Grafana-Dashboard für Session-Rekonstruktion.

Datum: 2026-06-04
Vorgänger: Sprint 6 ✅ (Testing & Quality Gates — 31 Vitest Tests, 9 Integration Tests, k6 Latenz-CI)

---

## Ausgangslage (aus Sprint 6)

| Was existiert | Stand |
|---------------|-------|
| `make test-integration` | Test-Stack startet/stoppt automatisch — LOG-Tests können ihn nutzen |
| `make test-safety` | Safety Gate 19/19 — Regression-Basis für LOG-11 (AuditWriter Integration) |
| `pkg/ulid/` | ULID-Wrapper vorhanden — AuditWriter nutzt ihn für event_id |
| Alle Services nutzen `log.Printf` | Wird in LOG-02..06 auf strukturierten Logger migriert |
| ADR-017/018 | Architekturentscheidungen vollständig — kein offener Klärungsbedarf |
| `modernc.org/sqlite` | Noch nicht in go.mod — wird in LOG-10 ergänzt |

---

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| LOG-01 | `pkg/logger/` — strukturierter slog-Wrapper | M | 🔲 Todo | — |
| LOG-02 | Control Server Migration | M | 🔲 Todo | LOG-01 |
| LOG-03 | Auth Service Migration | S | 🔲 Todo | LOG-01 |
| LOG-04 | Safety Service Migration | S | 🔲 Todo | LOG-01 |
| LOG-05 | Telemetry Service Migration | S | 🔲 Todo | LOG-01 |
| LOG-06 | WebRTC SFU Migration | S | 🔲 Todo | LOG-01 |
| LOG-07 | `POST /log` Endpoint — Frontend Log-Ingestion | M | 🔲 Todo | LOG-02 |
| LOG-08 | Frontend `logger.ts` + Integration | M | 🔲 Todo | LOG-07 |
| LOG-09 | Loki + Grafana + Promtail Docker Compose | M | 🔲 Todo | LOG-01 |
| LOG-10 | `pkg/audit/` — AuditWriter Interface + SQLiteAuditWriter | M | 🔲 Todo | LOG-01 |
| LOG-11 | Control Server Safety-Event-Integration | M | 🔲 Todo | LOG-10, LOG-02 |

---

## Abhängigkeitspfad

```
LOG-01 → LOG-02..06 (parallel) → LOG-07 → LOG-08
       ↘
        LOG-09 (parallel zu allem)
       ↘
        LOG-10 → LOG-11 (nach LOG-02)
```

---

## Implementierungsdetails je Task

### LOG-01 — `pkg/logger/` slog-Wrapper

**Neue Dateien:** `pkg/logger/logger.go`, `pkg/logger/event_types.go`

**API:**
```go
// Erstellt Logger für einen Service
log := logger.New("control-server")

// Technical Log (async → stdout → Loki)
log.Info("WebSocket connected", "subject", "op-1", "role", "ACTIVE_OPERATOR")

// Structured Safety/Audit Event (mit session-Kontext)
log.Event(ctx, logger.EventEmergencyStop,
    "Emergency Stop received",
    "session_id", sessionID,
    "vehicle_id", vehicleID,
    "operator_id", operatorID,
    "event_id", ulid.Generate(),
)
```

**`event_types.go`:** alle Event-Types aus ADR-017 als typisierte Konstanten
```go
const (
    EventSessionStarted     = "SESSION_STARTED"
    EventSessionEnded       = "SESSION_ENDED"
    EventSafeModeEntered    = "SAFE_MODE_ENTERED"
    EventEmergencyStop      = "EMERGENCY_STOP"
    EventDeadmanTimeout     = "DEADMAN_TIMEOUT"
    EventDeadmanArmed       = "DEADMAN_ARMED"
    EventAckTimeout         = "COMMAND_ACK_TIMEOUT"
    EventWsDisconnect       = "WS_DISCONNECT_CRITICAL"
    EventOperatorHandover   = "OPERATOR_HANDOVER_COMPLETED"
    EventStateTransition    = "STATE_TRANSITION_SYSTEM"
    EventCommandReceived    = "COMMAND_RECEIVED"
    EventMediaStateChange   = "MEDIA_STATE_CHANGE"
    // Frontend events
    EventFEEmergencyStop    = "FE_EMERGENCY_STOP_CLICKED"
    EventFEDeadmanHold      = "FE_DEADMAN_HOLD"
    EventFEWebRTCState      = "FE_WEBRTC_STATE_CHANGE"
    EventFEWSReconnect      = "FE_WS_RECONNECT"
    EventFEOperatorAck      = "FE_OPERATOR_ACK_CLICKED"
)
```

**Level per ENV:** `LOG_LEVEL=debug|info|warn|error` (default: `info`)

---

### LOG-02..06 — Service-Migrationen

Alle `log.Printf("[TAG] message")` Calls → `log.Event()` oder `log.Info/Warn/Error()`.

| Service | Haupt-Events |
|---------|-------------|
| **control-server** | SESSION_STARTED/ENDED, STATE_TRANSITION_*, SAFE_MODE_ENTERED, EMERGENCY_STOP, DEADMAN_ARMED/TIMEOUT, ACK_TIMEOUT, WS_DISCONNECT, COMMAND_RECEIVED |
| **auth-service** | Service-Start, Login, Token-Ausstellung, Fehler |
| **safety-service** | EmergencyStop-Event, Bus-Events |
| **telemetry-service** | MQTT connect/disconnect, MESSAGE_RECEIVED |
| **webrtc-sfu** | Session-Events (SESSION_CREATED..ENDED), ICE-State-Changes |

---

### LOG-07 — `POST /log` Endpoint

**Control Server** erhält Frontend-Logs:
```go
// cmd/control-server/main.go
mux.HandleFunc("POST /log", func(w http.ResponseWriter, r *http.Request) {
    var entry struct {
        Level     string         `json:"level"`
        EventType string         `json:"event_type"`
        SessionID string         `json:"session_id"`
        VehicleID string         `json:"vehicle_id"`
        OperatorID string        `json:"operator_id"`
        EventID   string         `json:"event_id"`
        Message   string         `json:"msg"`
        Data      map[string]any `json:"data,omitempty"`
    }
    // Validiert session_id wenn vorhanden, loggt mit service="frontend"
})
```

---

### LOG-08 — Frontend `logger.ts`

**Neue Datei:** `frontend/src/lib/logger.ts`

```typescript
// Sendet strukturierte Log-Events an Control Server (POST /api/log)
export function logEvent(
  eventType: string,
  msg: string,
  context: { sessionId?: string; vehicleId?: string; operatorId?: string; data?: Record<string, unknown> }
): void
```

**Integration in:** useDeadmanSwitch, SafetyPanel (E-Stop), useWebRTC, useSession (WS-Reconnect), SafeModeOverlay (Operator-Ack).

---

### LOG-09 — Loki + Grafana + Promtail

**Neue Infrastruktur:**
```
infrastructure/
├── loki/
│   └── loki.yml              # Loki v3 Config (filesystem storage, TTL 30d)
├── promtail/
│   └── promtail.yml          # Docker Service Discovery, JSON Pipeline
└── grafana/
    └── provisioning/
        ├── datasources/loki.yml   # Loki DataSource auto-provision
        └── dashboards/
            ├── dashboards.yml
            └── avoc.json          # AVOC Session Dashboard
```

**docker-compose.yml Erweiterung:**
```yaml
loki:      # Port 3100
grafana:   # Port 3001 (3000 ist Frontend)
promtail:  # kein Port (liest Docker-Logs)
```

**Promtail Pipeline** extrahiert `session_id`, `event_type`, `level` als Labels.

---

### LOG-10 — `pkg/audit/` AuditWriter + SQLiteAuditWriter

**Neue Dateien:** `pkg/audit/writer.go`, `pkg/audit/sqlite_writer.go`, `pkg/audit/noop_writer.go`

**Schema:**
```sql
CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    session_id TEXT NOT NULL,
    vehicle_id TEXT NOT NULL,
    operator_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    reason TEXT,
    system_state TEXT,
    ctrl_state TEXT,
    data TEXT,            -- JSON
    timestamp TEXT NOT NULL,
    written_at TEXT NOT NULL
);
CREATE INDEX idx_session ON audit_events(session_id);
CREATE INDEX idx_event_type ON audit_events(event_type);
```

**Storage:** `/data/audit/avoc_audit.db` — Docker Volume `audit-data`.

---

### LOG-11 — Control Server Safety-Event-Integration

Alle Safety-Trigger schreiben synchron via `AuditWriter.WriteSync()` **vor** SAFE_MODE-Transition:

```go
// In detector.go fire():
auditEvent := audit.SafetyAuditEvent{
    EventID:     ulid.Generate(),
    SessionID:   w.sessionID,
    EventType:   logger.EventDeadmanTimeout,
    Reason:      "dead-man switch timeout",
    SystemState: "CONNECTED",
    Timestamp:   time.Now(),
}
if err := w.auditWriter.WriteSync(auditEvent); err != nil {
    // Schreibfehler = CRITICAL → trotzdem SAFE_MODE
    log.Error("audit write failed", "error", err)
}
// DANN erst SAFE_MODE-Transition
w.sm.TransitionSystem(statemachine.StateSafeMode)
```

**Query-Endpoint:** `GET /audit/events?session_id=<ulid>` → JSON-Array der Safety-Events.

---

## Sprint-Ziel / Definition of Done

- [ ] `pkg/logger/logger.go` — `logger.New()`, `Event()`, Level-Konfiguration per `LOG_LEVEL` ENV
- [ ] Alle `log.Printf` in 6 Go-Services migriert → strukturiertes JSON auf stdout
- [ ] `POST /log` Endpunkt am Control Server — Frontend-Logs landen mit `service="frontend"`
- [ ] `frontend/src/lib/logger.ts` — Events für E-Stop, Deadman, WebRTC, WS-Reconnect, Operator-Ack
- [ ] Loki + Grafana + Promtail laufen in `docker-compose up` (Ports 3100, 3001)
- [ ] LogQL `{service="control-server"} | json | event_type="EMERGENCY_STOP"` liefert Treffer
- [ ] `pkg/audit/` — AuditWriter Interface + SQLiteAuditWriter + NoopWriter (Tests)
- [ ] Safety Events werden synchron in SQLite geschrieben bevor SAFE_MODE-Transition
- [ ] `GET /audit/events?session_id=<ulid>` liefert JSON-Array
- [ ] Safety Regression: weiterhin 19/19 grün
- [ ] `docker-compose up --build` — alle 11 Services (inkl. Loki, Grafana) healthy
