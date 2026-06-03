# Sprint 4 — Core Backend Services

Ziel: Vollständige Steuerkette. Protobuf-Parsing serverseitig. MQTT-Telemetrie aktiv. Session Recording. WebRTC SFU empfängt Session-Events und bereitet Signaling vor.

Datum: 2026-06-03
Vorgänger: Sprint 3 ✅ (Frontend Core — State-Polling, SAFE MODE, E-Stop, Dead-man live)

---

## Ausgangslage (aus Sprint 3)

| Was existiert | Stand |
|---------------|-------|
| `transport/websocket.go` `readLoop` | Zwei `TODO BE-04`-Kommentare: Protobuf-Parsing + Command-Routing fehlt noch; Deadman resettet auf jede Nachricht (Workaround); ACK ist `{"ack":true}` (JSON statt Protobuf) |
| `cmd/telemetry-service/main.go` | Stub — nur `/health`; `internal/telemetryservice/` leer; Mosquitto läuft auf Port 1883 |
| `cmd/webrtc-sfu/main.go` | Stub — nur `/health`; `internal/webrtcsfu/` leer; SFU-Session-Events werden schon vom Control Server gepusht (`HTTPSFUPublisher`), kommen aber nirgends an |
| `go.mod` | `paho.mqtt.golang` und `google.golang.org/protobuf` durch `go mod tidy` entfernt — müssen wieder hinzugefügt werden |
| `go-service.Dockerfile` | Proto-Gen nutzt `--go_opt=paths=source_relative` → Dateien landen in `gen/go/control.pb.go`, aber Import-Pfad lautet `avoc/gen/go/control/v1` — **Mismatch, bisher unbemerkt weil Gen-Code noch nicht importiert wird** |
| `docker-compose.yml` | Kommentar bei coturn: "Relay ports activated in Sprint 4"; webrtc-sfu hat keine UDP-Port-Range exposed |
| Frontend | Dead-man sendet Protobuf `DEADMAN_HOLD`, ACK-Latenzanzeige misst JSON-Roundtrip (Sprint-3-Vereinfachung) |

---

## Tasks

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| INFRA-02 | Proto-Gen Fix — Go Output-Pfade korrigieren | S | 🔲 Todo | `paths=source_relative` → Mismatch mit go_package Import-Pfaden; Fix: `--go_opt=module=avoc --go_out=.`; Prerequisite für BE-04 |
| BE-04 | Command Engine — Protobuf-Parsing + Routing + ControlAck | M | 🔲 Todo | ADR-007/010/012b/016; neues Paket `internal/controlserver/command/`; DEADMAN_HOLD/RELEASE, EMERGENCY_STOP, STEER/THROTTLE/BRAKE/SPEED; Rate Limiting; Protobuf `ControlAck` zurücksenden; Frontend-Latenzanzeige wird exakt |
| BE-05 | MQTT Telemetry Service — Mosquitto Client + Pub/Sub | M | 🔲 Todo | ADR-003/008/016; `internal/telemetryservice/` implementieren; Protobuf TelemetryEvent; Subscribe vehicle/+/telemetry; GET /telemetry/latest/{vehicleId}; paho.mqtt.golang in go.mod |
| BE-07 | Session Recording — Interface + In-Memory Adapter | M | 🔲 Todo | ADR-005/016; `internal/recording/`; SessionRecorder Interface + MockRecorder; in Control Server integrieren; session_id als Root Key |
| BE-08 | WebRTC SFU — Pion/Go + Session Event Consumer | M | 🔲 Todo | ADR-014/015; `internal/webrtcsfu/` implementieren; Session Event HTTP-Endpunkt; SDP-Signaling-Infrastruktur (SDP/ICE über bestehenden WS-Kanal); Primary Stream Routing; MEDIA STATE Notifications; pion/webrtc/v4 in go.mod; coturn Relay-Ports in docker-compose.yml aktivieren |

---

## Abhängigkeitspfad

```
INFRA-02 (S — sofort, Prerequisite für BE-04)
  │
  ▼
BE-04 ───────────────────────────────────────────────────────────┐
BE-05 (parallel, kein BE-04 nötig) ──────────────────────────── │→ Sprint 4 DoD ✓
BE-07 (parallel, kein BE-04 nötig) ──────────────────────────── │
BE-08 (parallel, längste Task) ──────────────────────────────── ┘
```

---

## Implementierungsdetails je Task

### INFRA-02 — Proto-Gen Fix

**Problem:** `go-service.Dockerfile` nutzt:
```bash
protoc --proto_path=proto --go_out=gen/go --go_opt=paths=source_relative proto/*.proto
```
→ Erzeugt `gen/go/control.pb.go`, aber Import-Pfad lautet `avoc/gen/go/control/v1`.
Go kann das nicht auflösen.

**Fix:**
```bash
protoc --proto_path=proto \
  --go_out=. \
  --go_opt=module=avoc \
  proto/*.proto
```
→ Erzeugt `gen/go/control/v1/control.pb.go` — passt zu `import "avoc/gen/go/control/v1"`.

Gleiches Fix-Pattern für `Makefile` target `proto-gen`.

---

### BE-04 — Command Engine

**Neues Paket:** `internal/controlserver/command/engine.go`

```go
type Engine struct {
    sm          *statemachine.Machine
    safetyPub   safety.Publisher
    sessionMgr  *session.Manager
    deadman     *safety.DeadmanWatchdog
    rateLimiter *rate.Limiter   // golang.org/x/time/rate, 100 cmd/s
}

func (e *Engine) Handle(rawMsg []byte, sess session.Session) (ackBytes []byte, err error)
```

**Routing nach CommandType:**

| CommandType | Aktion |
|-------------|--------|
| `DEADMAN_HOLD` | `deadman.Reset()` (ersetzt den Workaround in readLoop) |
| `DEADMAN_RELEASE` | kein Reset — Watchdog läuft ab |
| `EMERGENCY_STOP` | `safetyPub.TriggerEmergencyStop(...)` + `sm.TransitionSystem(SAFE_MODE)` |
| `STEER / THROTTLE / BRAKE / SPEED` | Rate Limiting → Log → (Sprint 5: an Vehicle weiterleiten) |

**ControlAck zurücksenden:**
```go
ack := create(ControlAckSchema, {
    header: { sessionId, eventId: ulid.Generate(), ... },
    success: true,
})
conn.WriteMessage(websocket.BinaryMessage, toBinary(ControlAckSchema, ack))
```

→ Frontend `ws-client.ts` empfängt Binary-ACK → `onmessage` feuert → Latenz korrekt gemessen.

**Rate Limiting:** Token Bucket, 100 commands/s. Überschreitung → `ControlAck{success: false, error_msg: "rate limited"}`, kein SAFE_MODE.

**`readLoop`-Integration:** Die beiden `TODO BE-04`-Kommentare in `transport/websocket.go` werden durch `engine.Handle(msg, sess)` ersetzt.

---

### BE-05 — MQTT Telemetry Service

**Neue Dateien:**
- `internal/telemetryservice/client.go` — Paho MQTT-Client (connect, subscribe, reconnect)
- `internal/telemetryservice/handler.go` — TelemetryEvent-Parsing + Speicherung
- `cmd/telemetry-service/main.go` — vollständig implementieren

**Topics:**
- Subscribe: `vehicle/+/telemetry` → empfängt Protobuf `TelemetryEvent`
- Publish: `system/state` → publiziert SYSTEM STATE Änderungen (optional Sprint 4)

**HTTP-Endpunkte:**
- `GET /telemetry/latest/{vehicleId}` → letzte TelemetryEvent als JSON (für Frontend Connection Panel)
- `GET /health` → bereits vorhanden

**go.mod:** `github.com/eclipse/paho.mqtt.golang v1.4.3` wieder hinzufügen.

**docker-compose.yml:** `control-server` bekommt `TELEMETRY_SERVICE_URL` Env-Var.

---

### BE-07 — Session Recording

**Neues Paket:** `internal/recording/`

```go
// recorder.go — Interface (ADR-005)
type SessionRecorder interface {
    StartSession(sessionID, vehicleID, operatorID string)
    EndSession(sessionID string)
    RecordControlEvent(header CorrelationHeader, cmdType string, value float32)
    RecordStateSnapshot(header CorrelationHeader, sysState, ctrlState string)
    RecordSafetyEvent(header CorrelationHeader, eventType string, reason string)
}

// memory_recorder.go — In-Memory Mock (Sprint 4)
type MemoryRecorder struct { ... }

// noop_recorder.go — No-op (default wenn kein Recorder konfiguriert)
type NoopRecorder struct{}
```

**Integration in Control Server:**
- `SessionRecorder` wird in `cmd/control-server/main.go` instanziiert (default: `MemoryRecorder`)
- `session/manager.go` ruft `RecordStateSnapshot` bei State-Transitionen auf
- `transport/websocket.go` ruft `RecordControlEvent` nach `engine.Handle()` auf
- Bei SAFE_MODE: `RecordSafetyEvent`

**Hinweis:** Storage-ADR (ADR-005 Folge) ist noch offen → `MemoryRecorder` als Platzhalter. Kein File/DB-Adapter in Sprint 4.

---

### BE-08 — WebRTC SFU (Pion/Go)

**Neues Paket:** `internal/webrtcsfu/`

**Sprint-4-Scope (minimal viable):**
1. Session Event Consumer (`POST /session/event`)
2. SDP Signaling-Infrastruktur (HTTP-Endpunkte für SDP/ICE)
3. Primary Stream: Vehicle verbindet sich, SFU empfängt RTP und routet an Operator
4. MEDIA STATE Notifications an Control Server

**Deferred auf Sprint 5:**
- Secondary Streams (on-demand)
- Server-seitiges Recording
- Multi-Operator Forwarding (>1 Observer)

**Neue Endpunkte im SFU:**
```
POST /session/event          ← Control Server pusht SESSION_* Events
POST /offer                  ← Vehicle sendet SDP Offer
POST /answer/{vehicleId}     ← Operator empfängt SDP Answer
POST /ice/{peerId}           ← ICE Candidate Exchange
GET  /health                 ← bereits vorhanden
```

**go.mod:** `github.com/pion/webrtc/v4` hinzufügen.

**docker-compose.yml:**
```yaml
webrtc-sfu:
  environment:
    SFU_PORT: "8084"
    CONTROL_SERVER_URL: "http://control-server:8080"
  ports:
    - "8084:8084"
    - "10000-10100:10000-10100/udp"   # WebRTC Media Ports

stun-turn:
  ports:
    - "49152-49200:49152-49200/udp"   # coturn Relay Ports (Sprint 4)
```

**nginx.conf:** `/sfu/` Proxy-Route ergänzen (für Frontend SDP Signaling in Sprint 5).

---

## Infrastruktur-Änderungen

| Datei | Änderung |
|-------|---------|
| `go-service.Dockerfile` | Proto-Gen Fix: `--go_opt=module=avoc --go_out=.` |
| `Makefile` target `proto-gen` | Gleicher Fix |
| `go.mod` | `paho.mqtt.golang`, `pion/webrtc/v4`, `google.golang.org/protobuf` wieder aufnehmen |
| `docker-compose.yml` | WebRTC UDP-Port-Range, coturn Relay-Ports, neue Env-Vars |
| `nginx.conf` | `/sfu/` Proxy-Route |

---

## Sprint-Ziel / Definition of Done

- [ ] INFRA-02: Proto-Gen erzeugt korrekte Go-Verzeichnisstruktur (`gen/go/control/v1/`, etc.)
- [ ] BE-04: Server parsed Protobuf `ControlCommand` — DEADMAN_HOLD resettet Watchdog korrekt (statt jede Nachricht)
- [ ] BE-04: `ControlAck` als Protobuf Binary zurückgesendet — Frontend-Latenzanzeige messen echten Protobuf-Roundtrip
- [ ] BE-04: EMERGENCY_STOP Command → direkt SAFE_MODE (ohne separaten HTTP-Call)
- [ ] BE-04: Rate Limiting aktiv (100 cmd/s, ACK mit error bei Überschreitung)
- [ ] BE-05: Telemetry Service verbindet sich mit Mosquitto, subscribt Vehicle-Topics
- [ ] BE-05: TelemetryEvent über Protobuf empfangbar, `GET /telemetry/latest/{vehicleId}` antwortet
- [ ] BE-07: `SessionRecorder`-Interface implementiert, Control Server nutzt `MemoryRecorder`
- [ ] BE-07: Control Events + Safety Events + State Snapshots werden aufgezeichnet
- [ ] BE-08: SFU empfängt SESSION_* Events vom Control Server (SESSION_SAFE_MODE → Streams droppen)
- [ ] BE-08: SDP Signaling-Endpunkte vorhanden, Vehicle kann WebRTC-Offer senden
- [ ] BE-08: Primary Stream von Vehicle an Operator routbar (lokal testbar)
- [ ] Sprint-2-Safety-Tests: weiterhin 19/19 grün
