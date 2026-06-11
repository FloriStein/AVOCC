# Sprint 11 — Verification Report

Stand: 2026-06-11
Verdict: **PASS**

---

## Scope

Commit `736d974` — Sprint 11: Vehicle Connectivity & Feedback (ADR-021)
23 Files, 1273 Insertions — 1 Commit.

---

## Method

Cold-start des Dev-Stacks via Docker Compose (`make up` / `--env-file .env`),
dann Go-Verification-Client (Docker `--network host`) für den vollständigen
Operator→Command→ACK→Telemetry-Flow gegen den laufenden Stack.
Abschließend Bundle-Analyse des gebauten Frontend (String-Verifikation nach Minification).

---

## Steps

### Backend: vehicle-mock Connect

| # | Action | Result |
|---|--------|--------|
| 1 | Stack gestartet mit `--env-file .env` | vehicle-mock: `"vehicle connected to control server"` + `"MQTT connected"` |
| 2 | Control-Server-Log geprüft | `"vehicle connected" vehicle_id=vehicle-001` — Registry-Eintrag angelegt |

**Log-Beweise:**
```
vehicle-mock: {"msg":"vehicle connected to control server","vehicle_id":"vehicle-001","url":"ws://control-server:8080/vehicle/ws"}
vehicle-mock: {"msg":"MQTT connected","broker":"mosquitto:1883"}
control-server: {"msg":"vehicle connected","vehicle_id":"vehicle-001"}
```

### Backend: Operator → Command → ACK Flow

| # | Action | Result |
|---|--------|--------|
| 3 | `POST /auth/operator/login` → Operator JWT | ✅ JWT erhalten |
| 4 | `ws://localhost:8080/ws` mit Bearer-Token | ✅ WebSocket connected |
| 5 | `POST /session/start` | ✅ `{"session_id":"01KTTSYMYWP2AMQBZRQC1MG6C5"}` (ULID) |
| 6 | ControlCommand STEER=0.75 (79 Bytes Protobuf) gesendet | ✅ Gesendet, Control-Server-Log: `"vehicle ACK received" command_event_id=verify-event-001` |
| 7 | `GET /vehicle/ack/latest/vehicle-001` nach 500ms | ✅ ACK auf Versuch 1 |

**ACK-Response:**
```json
{
  "command_event_id": "verify-event-001",
  "received": true,
  "received_at_ms": 1781163578333,
  "vehicle_connected": true,
  "vehicle_id": "vehicle-001"
}
```

### Backend: Telemetrie mit Actuation-Feldern

| # | Action | Result |
|---|--------|--------|
| 8 | `GET /telemetry/latest/vehicle-001` nach 2s | ✅ Alle 5 Actuation-Felder vorhanden |

**Telemetry-Response (Actuation-Felder):**
```json
{
  "steer_commanded": 0.75,
  "steer_actual": 0.6375,
  "throttle_commanded": 0,
  "throttle_actual": 0,
  "brake_commanded": 0
}
```

`steer_actual = 0.6375 = 0.75 × 0.85` — Lerp mit 15% Aktuator-Lag, mathematisch korrekt.

### Safety-Invariante

| # | Action | Result |
|---|--------|--------|
| 9 | Go-Client beendet (Operator-WS disconnect) | ✅ SAFE_MODE ausgelöst: `"system":"SAFE_MODE","control":"CONTROL_BLOCKED"` |

### Probes

| # | Probe | Result |
|---|-------|--------|
| P1 | `GET /vehicle/ack/latest/no-such-vehicle` | ✅ 404 "no ack for vehicle" — kein Panic |
| P2 | `GET /vehicle/ack/latest/` (kein vehicle_id) | ✅ 400 "vehicle_id required" |

### Frontend: InputIndicatorPanel im Bundle

| # | Check | Result |
|---|-------|--------|
| 10 | Frontend-HTTP-Response | ✅ HTML korrekt, Bundle `/assets/index-D-GH87ob.js` gefunden |
| 11 | Bundle: `"ACK vor"` | ✅ |
| 11 | Bundle: `"Nicht verbunden"` / `"Verbunden"` | ✅ |
| 11 | Bundle: `"Ist-Werte"` | ✅ |
| 11 | Bundle: `"/vehicle/ack/latest"` | ✅ |
| 11 | Bundle: `"steer_actual"`, `"vehicle_connected"` | ✅ |
| 11 | Bundle: `"Steer"`, `"Throt"`, `"Brake"` (ActuationBars) | ✅ |
| 11 | Bundle: `"500"` (Poll-Interval useVehicleAck) | ✅ |

---

## Findings

### ⚠️ `make up` startet Frontend nicht (Sprint-10-Regression, kein Sprint-11-Bug)

`nginx.conf` konfiguriert `listen 443 ssl` mit Zertifikat-Pfad `/etc/nginx/ssl/cert.pem`.
Dieses Zertifikat wird nur von `deploy.sh` für den EC2-Deploy via `openssl` erzeugt.
Im lokalen Dev-Stack existiert kein SSL-Volume → `avoc-frontend-1` crasht mit Exit 1.

**Workaround für Verifikation:** `openssl req -x509 ... -out cert.pem -keyout key.pem` + Docker `-v /tmp/ssl:/etc/nginx/ssl:ro` beim manuellen Start.

**Empfehlung:** Dev-Stack (`docker-compose.yml`) mit Fallback auf selbst-generiertes Cert oder HTTP-only (Port 80) für lokale Entwicklung. Sprint 10 hat HTTPS für Prod korrekt umgesetzt, aber den Dev-Stack nicht angepasst.

### Telemetrie `session_id` leer

vehicle-mock setzt kein `session_id` in den `TelemetryEvent`-Payload — es hat keinen Zugriff auf die Operator-Session des Control Servers. Kein Funktionsbug, aber ein Log-Korrelations-Gap: Telemetrie-Events können nicht direkt einer Session zugeordnet werden.

### Latency: ACK <500ms

ACK kam im ersten Poll-Intervall (500ms) zurück. Tatsächliche Round-Trip-Zeit wahrscheinlich <50ms (Control Server → vehicle-mock via Docker-Netzwerk). Kein Latenz-Problem erkennbar.

---

## Summary

Alle 3 Kern-Features von Sprint 11 funktionieren end-to-end:

1. **Command Forwarding** ✅ — ControlCommand erreicht vehicle-mock (79 Bytes Protobuf, <500ms ACK)
2. **VehicleCommandAck** ✅ — ACK mit korrektem event_id-Echo, vehicle_connected=true
3. **Telemetrie Actuation** ✅ — steer_commanded=0.75, steer_actual=0.6375 (15% Lag, mathematisch korrekt)
4. **Frontend Bundle** ✅ — Alle InputIndicatorPanel-Strings im gebauten Bundle bestätigt
5. **Safety-Invarianten** ✅ — WS-Disconnect → SAFE_MODE korrekt ausgelöst, Sprint-11-Integration verletzt keine Safety-Regeln
