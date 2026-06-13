# Sprint 14 — Security & Observability

Ziel: REST-Endpoints JWT-geschützt. Control- und Video-Kanal werden separat mit Status + Latenz angezeigt. Frontend signalisiert wenn das Backend nicht erreichbar ist.

Datum: 2026-06-13 | **Status: In Bearbeitung 🔄**
Vorgänger: Sprint 13 ✅ (Dev-Stack Stabilisierung & Log-Korrelation)

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| AUTH-01 | JWT-Pflicht auf REST-Endpoints im control-server | M | 🔲 |
| UI-01 | Dual-Channel Status: Control + Video separat mit Latenz in ConnectionPanel | M | 🔲 |
| ROB-01 | Backend-nicht-erreichbar-Zustand im Frontend (Banner + Zustandsschutz) | S | 🔲 |
| OBS-01 | Vehicle "zuletzt gesehen" Heartbeat-Timestamp (Bonus, wenn Zeit bleibt) | S | 🔲 |

---

## Scope-Details

### AUTH-01 — JWT-Pflicht REST-Endpoints
- Control-server hat bereits JWT-Middleware für WebSocket (`/ws`) — dieselbe auf REST ausdehnen
- Betrifft: `POST /session/start`, `POST /session/end`, `POST /emergency-stop`, `GET/POST/DELETE /vehicles`, `GET /audit/events`
- Ausnahmen: `GET /api/state`, `GET /api/health` (polling ohne Session-Kontext, bewusst offen)
- Frontend: `Authorization: Bearer <token>` Header in `api-client.ts` für alle geschützten Calls
- Schließt ⚠️-Finding aus Sprint 12/13

### UI-01 — Dual-Channel Status
- `useWebRTC.ts`: `RTCPeerConnection.getStats()` alle 1s → `onStats`-Callback
- `ConnectionPanel`: zwei Zeilen statt einer — „Control 🟢 23 ms" / „Video 🟢 45 ms"
- Bei `DEGRADED`: betroffener Kanal rot markiert; ersetzt globales gelbes Badge

### ROB-01 — Backend nicht erreichbar
- `useSystemState.ts`: bei HTTP-Fehler / Network-Error → neuer Zustand `UNREACHABLE`
- `App.tsx`: rotes Banner „Backend nicht erreichbar — Steuerung blockiert" (wie DEGRADED-Banner)
- Verhindert dass Operator glaubt zu steuern während Backend tot ist

### OBS-01 — Vehicle Heartbeat (Bonus)
- MQTT-Telemetry kommt schon alle ~100ms — letzter Timestamp reicht
- `AckBadge` → "Fahrzeug aktiv vor 2s" auch ohne aktive Steuerbefehle
