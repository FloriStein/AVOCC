# ADR-020: Video Ingestion & Distribution — MediaMTX WHIP/WHEP

Status: Accepted

## Kontext

Sprint 9 schließt den letzten fehlenden Kanal: echten Live-Videostream vom Fahrzeug
zum Operator-Browser. Larix Broadcaster (iOS/Android) wird als Fahrzeug-Kamera-Client
eingesetzt und spricht nativ **WHIP** (WebRTC-HTTP Ingestion Protocol). Der Operator-Browser
empfängt via **WHEP** (WebRTC-HTTP Egress Protocol).

Der bestehende Pion-SFU (ADR-014) wurde für Custom-Signaling (POST /offer, POST /subscribe)
gebaut und beherrscht WHIP/WHEP nicht nativ. Ein Umbau würde den gesamten Media-Layer
neu implementieren.

## Anforderungen

| Anforderung | Ausprägung |
|-------------|-----------|
| Ingestion | Larix Broadcaster → WHIP (IETF-Standard) |
| Distribution | Operator-Browser → WHEP (IETF-Standard) |
| Auth (Publish) | Bearer Stream Key (SSM) |
| Auth (Read) | Operator JWT (Control Server validiert) |
| NAT-Traversal | TURN via coturn (Fahrzeug im 5G-Netz) |
| SAFE_MODE | Alle Subscriber sofort trennen |
| Safety Invariante | Video-Fehler = DEGRADED, nie SAFE_MODE-Trigger (ADR-009) |

## Analysierende Optionen

### Option A: Pion SFU für WHIP/WHEP erweitern

**Vorteile:**
- Bestehender Code bleibt zentraler Media-Layer

**Nachteile:**
- WHIP/WHEP sind komplexe Standards — vollständige Reimplementierung in Pion
- ICE Trickle, Bearer-Token-Auth, WHEP Resource URLs, PATCH für Renegotiation
- Hoher Aufwand für Standards, die MediaMTX out-of-the-box bietet
- Pion-SFU müsste ausgehende HTTP-Calls für Auth übernehmen

### Option B: MediaMTX als WHIP/WHEP Router (gewählt)

**Vorteile:**
- WHIP/WHEP nativ unterstützt (MediaMTX v1.x)
- `externalAuthenticationURL` delegiert alle Auth-Entscheidungen an Control Server
- Management API (`/v3/`) erlaubt SAFE_MODE-Kick ohne eigene Peer-Connection-Logik
- ICE/STUN/TURN nativ konfigurierbar
- Docker-Image verfügbar (`bluenviron/mediamtx:latest`)
- IETF-Standard — Larix, OBS, ffmpeg, GStreamer kompatibel

**Nachteile:**
- Zusätzlicher Docker-Service
- Pion-SFU verliert Media-Routing-Rolle (bleibt aber für Session-Event-Bus)

## Entscheidung

Wir wählen **Option B: MediaMTX als WHIP/WHEP Router**.

## Begründung

WHIP/WHEP sind konsolidierte IETF-Standards. Larix implementiert WHIP nativ — kein
RTMP-Bridging, kein Transcoding, keine Custom-Signaling-Erweiterung des Pion-SFU nötig.
MediaMTX bietet eine vollständige WHIP/WHEP-Implementierung mit Auth-Hook und Management-API.

Der Pion-SFU bleibt als **passiver Session-Event-Subscriber** (ADR-007 Invariant:
Dumb Media Router with State Subscription) — er ruft keine externen Services auf.
Der **Control Server** übernimmt die SAFE_MODE-Kontrolle über MediaMTX direkt, da er
der Auslöser von SAFE_MODE ist (Single Point of Control).

## Architektur

```
Larix (Smartphone, 5G)
  │
  │ WHIP POST /vehicle-001/whip
  │ Authorization: Bearer <WHIP_STREAM_KEY>
  ▼
MediaMTX (Docker, Port 8889)
  │
  │ externalAuthenticationURL → POST /internal/media/auth
  ▼
Control Server
  │ publish: Bearer == WHIP_STREAM_KEY (env) → 200/401
  │ read:    JWT valide + aktive Session → 200/401
  └───────────────────────────────────────────────────
  │ bei SAFE_MODE:
  │ DELETE http://mediamtx:9997/v3/webrtcsessions/{id}
  ▼
MediaMTX trennt alle aktiven Subscriber

Operator Browser
  │ WHEP POST /whep/vehicle-001/whep (via nginx)
  │ Authorization: Bearer <JWT>
  ▼
MediaMTX → video srcObject → VideoPanel.tsx
```

## Auth-Design (ein Mechanismus)

Alle WHIP- und WHEP-Requests gehen durch einen einzigen Auth-Hook:

```
MediaMTX externalAuthenticationURL → POST /internal/media/auth (Control Server)

Request-Body (von MediaMTX):
  action:   "publish" | "read"
  path:     "vehicle-001"
  password: "<bearer-token>"  (Bearer aus Authorization-Header)

Response:
  200 OK      → erlaubt
  401 Unauth  → abgelehnt
```

- **publish**: Control Server vergleicht `password` gegen `WHIP_STREAM_KEY` Env-Variable
- **read**: Control Server validiert `password` als Operator-JWT (bestehende JWT-Logik)
- MediaMTX hat **keine eigenen Credentials** — kein doppelter Auth-Mechanismus

## ICE / NAT-Traversal

```yaml
webrtcICEServers2:
  - url: stun:coturn:3478
  - url: turn:coturn:3478
    username: ${TURN_USER}
    password: ${TURN_PASSWORD}
```

TURN_USER und TURN_PASSWORD stammen aus SSM (bereits in deploy.sh vorhanden).
Kein neuer SSM-Parameter für TURN.

## SAFE_MODE-Integration

Beim Auslösen von SAFE_MODE im Control Server:

```go
// internal/mediamtx/client.go
func (c *Client) KickVehicle(vehicleID string) {
    // GET /v3/paths/get/{vehicleID} → WebRTC Session IDs
    // DELETE /v3/webrtcsessions/{id} für alle aktiven Subscriber
}
```

Der Pion-SFU empfängt weiterhin SESSION_SAFE_MODE-Events (für seine interne
State-Buchhaltung), ruft aber **nicht** die MediaMTX API auf.

## vehicleId-Routing (Sprint 9: Single Vehicle)

MediaMTX-Pfad = vehicleId (statisch, in Larix konfiguriert).

- Larix WHIP-URL: `http://<EC2-IP>:8889/vehicle-001/whip`
- Browser WHEP-URL: `http://<EC2-IP>:3000/whep/vehicle-001/whep` (via nginx)

Multi-Vehicle-Routing (dynamische vehicleId-to-path-Zuordnung) ist eine Folge-Entscheidung
für Sprint 10 (ADR-020-Folge, `tasks/backlog.md`).

## Neue Ports / Services

| Service | Port | Zweck |
|---------|------|-------|
| MediaMTX | 8889 | WHIP/WHEP (extern für Larix + intern für Browser via nginx) |
| MediaMTX | 9997 | Management API (nur Docker-intern) |

CDK Security Group: Port 8889 TCP muss geöffnet werden.

## ADR-014-Abgrenzung

ADR-014 entschied: WebRTC SFU mit Pion/Go. Diese Entscheidung bleibt gültig für die
**Architekturidee** (SFU als zweiter Hub, orthogonal zum Control Hub). ADR-020 konkretisiert
die **Implementierung** des Media-Layers: MediaMTX als WHIP/WHEP-konforme Implementierung
des SFU-Konzepts. Pion bleibt als Session-Event-Consumer aktiv.

## Konsequenzen

### Positiv
- Larix-Kompatibilität out-of-the-box (WHIP nativ)
- IETF-Standards (WHIP/WHEP) — langfristig wartbar
- Ein Auth-Mechanismus für Publish und Read
- Control Server = Single Point of Control für SAFE_MODE-Effekte
- Pion-SFU ohne ausgehende Calls = ADR-007-konform

### Negativ
- Zusätzlicher Docker-Service (Ressourcen auf t3.micro knapp)
- Pion-SFU `/offer`+`/subscribe`-Endpunkte werden für Media nicht mehr genutzt
- Multi-Vehicle-Routing noch nicht implementiert (Sprint 10)
