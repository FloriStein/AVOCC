# Sprint 9 — WebRTC Videostream: Larix WHIP → MediaMTX → Browser

Ziel: Ende-zu-Ende-Video vom Smartphone (Larix Broadcaster via WHIP über 5G) durch MediaMTX
an den Operator-Browser (WHEP). Sprint 8 hat das System auf EC2 gebracht — Sprint 9
schließt den letzten fehlenden Kanal: echten Live-Videostream.

Datum: 2026-06-05
Vorgänger: Sprint 8 ✅ (EC2 Deployment via Docker Hub — ADR-019)

---

## Architektur-Entscheidung (ADR-020)

MediaMTX übernimmt WHIP-Ingestion (Larix) und WHEP-Distribution (Browser).
Der Pion SFU bleibt **passiver Session-Event-Subscriber** — er ruft keine externen Services auf.
Der **Control Server** kontrolliert MediaMTX direkt bei SAFE_MODE.

```
Larix (5G, Smartphone)
  ↓ WHIP POST /vehicle-001/whip  (Authorization: Bearer <WHIP_STREAM_KEY>)
MediaMTX (neuer Docker Service, Port 8889)
  ↓ Auth-Hook (alle Requests — publish UND read)
Control Server POST /internal/media/auth
  │  publish: Bearer == WHIP_STREAM_KEY (Env) → 200 / 401
  │  read:    JWT valide + aktive Operator-Session → 200 / 401
  └──────────────────────────────────────────────────────────

Operator Browser
  → GET /whep/vehicle-001/whep (via nginx)
  → WHEP SDP exchange → MediaMTX → Video

Control Server (bei SAFE_MODE):
  → DELETE http://mediamtx:9997/v3/webrtcsessions/...
  (direkt, kein Umweg über Pion SFU)

Pion SFU:
  empfängt SESSION_* Events (wie bisher) — keine ausgehenden Calls zu MediaMTX
```

**Design-Prinzipien:**
- Ein einziger Auth-Mechanismus: `externalAuthenticationURL` → Control Server
- SAFE_MODE-Kontrolle über MediaMTX liegt beim Control Server (der es auslöst)
- Pion SFU = reiner Session-State-Mirror, keine Seiteneffekte nach außen

---

## Ausgangslage (aus Sprint 8)

| Was existiert | Stand |
|---------------|-------|
| `webrtc-sfu` Service | Pion SFU mit Session-Event-Bus — bleibt, verliert nur Media-Routing |
| `useWebRTC.ts` | Custom `/sfu/subscribe` → wird auf WHEP-Standard umgestellt |
| `VideoPanel.tsx` | Fertig, kein Änderungsbedarf |
| nginx `/sfu/` Proxy | Wird um `/whep/` ergänzt |
| coturn STUN/TURN | Bleibt, MediaMTX nutzt TURN-Credentials für ICE |
| ADR-019 Deployment | docker-compose.prod.yml + deploy.sh + SSM |

---

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| STREAM-01 | ADR-020 — MediaMTX als WHIP/WHEP Router | L | 🔲 | — |
| STREAM-02 | `infrastructure/mediamtx/mediamtx.yml` + Docker Service | M | 🔲 | STREAM-01 |
| STREAM-03 | nginx: `/whep/` Proxy | S | 🔲 | STREAM-02 |
| STREAM-04 | `useWebRTC.ts` → WHEP-Protokoll + vehicleId-Prop | M | 🔲 | STREAM-02 |
| STREAM-05 | Control Server: `POST /internal/media/auth` + SAFE_MODE → MediaMTX API | M | 🔲 | STREAM-02 |
| STREAM-06 | TURN in MediaMTX ICE-Config + Compose env | S | 🔲 | STREAM-02 |
| STREAM-07 | CDK Port 8889 + SSM `/avoc/prod/whip-stream-key` + setup-ssm.sh | S | 🔲 | — |
| STREAM-08 | `docker-compose.prod.yml`: mediamtx + Control Server env + deploy.sh | S | 🔲 | STREAM-02 |
| STREAM-09 | Larix Setup Guide + E2E Smoke Test Protokoll | S | 🔲 | STREAM-07, STREAM-08 |

---

## Abhängigkeitspfad

```
STREAM-01 (ADR) → STREAM-02 (MediaMTX Service) ──────────────────────┐
                                                 ├── STREAM-03 (nginx) │
                                                 ├── STREAM-04 (hook)  ├──▶ STREAM-09
                                                 ├── STREAM-05 (CS)    │
                                                 ├── STREAM-06 (TURN)  │
                                                 └── STREAM-08 (prod)  │
STREAM-07 (CDK + SSM) ────────────────────────────────────────────────┘
```

---

## Implementierungsdetails je Task

### STREAM-01 — ADR-020

Neue Datei: `docs/adr/020-mediamtx-whip-whep.md`

**Entscheidung:** MediaMTX als WHIP/WHEP Media Router; Control Server als einzige Auth-
und SAFE_MODE-Kontrollebene; Pion SFU als passiver Session-State-Subscriber.

**Begründung:**
- Larix Broadcaster spricht WHIP nativ — kein Bridging, kein Transcoding nötig
- WHIP/WHEP sind IETF-Standards (RFC-Prozess) — zukunftssicherer als Custom-Signaling
- Ein Auth-Hook → Control Server ersetzt alle verteilten Credentials
- Pion SFU ohne ausgehende Calls bleibt ADR-007 konform (Dumb Media Router with State Subscription)
- Control Server als SAFE_MODE-Auslöser kontrolliert auch MediaMTX: Single Point of Control

---

### STREAM-02 — MediaMTX Config + Docker Service

**Neue Datei: `infrastructure/mediamtx/mediamtx.yml`**

```yaml
# MediaMTX (ADR-020) — WHIP/WHEP Router
# Auth: externalAuthenticationURL → Control Server (einzige Validierungsinstanz)
# ICE: STUN + TURN via coturn (Env-Injection zur Laufzeit)

api: yes
apiAddress: :9997

webrtc: yes
webrtcAddress: :8889

# ICE-Server für Browser-Verbindungen (Operator hinter NAT)
# TURN_USER + TURN_PASSWORD werden als Env-Variablen injiziert (STREAM-06)
webrtcICEServers2:
  - url: stun:coturn:3478
  - url: turn:coturn:3478
    username: ${TURN_USER}
    password: ${TURN_PASSWORD}

# Einziger Auth-Mechanismus: alle Publish/Read-Requests → Control Server
externalAuthenticationURL: http://control-server:8080/internal/media/auth

paths:
  "~^vehicle-.*":
    # Larix publisht via WHIP (Bearer Stream Key → auth hook)
    # Operator Browser subscribed via WHEP (JWT → auth hook)
```

**In `docker-compose.yml` (dev):**
```yaml
mediamtx:
  image: bluenviron/mediamtx:latest
  ports:
    - "8889:8889"   # WHIP/WHEP
    - "9997:9997"   # Management API (nur intern)
  volumes:
    - ./infrastructure/mediamtx/mediamtx.yml:/mediamtx.yml:ro
  environment:
    TURN_USER: ${TURN_USER:-avoc}
    TURN_PASSWORD: ${TURN_PASSWORD:-changeme}
  networks:
    - avoc-net
  restart: unless-stopped
  depends_on:
    - control-server
    - stun-turn
```

---

### STREAM-03 — nginx `/whep/` Proxy

In `infrastructure/docker/nginx.conf` ergänzen:

```nginx
# WHEP — Operator Browser subscribed an MediaMTX Stream
# /whep/vehicle-001/whep → http://mediamtx:8889/vehicle-001/whep
location /whep/ {
    set $upstream_mtx http://mediamtx:8889;
    rewrite ^/whep/(.*) /$1 break;
    proxy_pass $upstream_mtx;
    proxy_set_header Host $host;
    proxy_set_header Authorization $http_authorization;
}
```

`/sfu/` Proxy bleibt bestehen (interne Service-Kommunikation unberührt).

---

### STREAM-04 — `useWebRTC.ts` → WHEP-Protokoll

**vehicleId als neues Argument:**
```typescript
export function useWebRTC(
  sessionId: string | null,
  vehicleId: string | null,   // für WHEP URL
  enabled: boolean
)
```

**WHEP-Request (ersetzt custom /sfu/subscribe):**
```typescript
// ICE-Gathering vollständig abwarten (non-trickle WHEP)
await new Promise<void>(resolve => {
  if (pc.iceGatheringState === 'complete') { resolve(); return }
  pc.onicegatheringstatechange = () => {
    if (pc.iceGatheringState === 'complete') resolve()
  }
})

const token = getStoredJwt()  // bestehende JWT aus Auth-Store
const res = await fetch(`/whep/${vehicleId}/whep`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/sdp',
    'Authorization': `Bearer ${token}`,
  },
  body: pc.localDescription!.sdp,
})
if (!res.ok) { updateState('MEDIA_FAILED'); return }

const answerSdp = await res.text()  // raw SDP — kein JSON
await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
```

`VideoPanel.tsx` erhält `vehicleId: string | null` als Prop.
`App.tsx` leitet vehicleId aus Session-State weiter (Session-Assign enthält vehicleId).

---

### STREAM-05 — Control Server: Auth-Hook + SAFE_MODE → MediaMTX

**Zwei neue Verantwortlichkeiten im Control Server:**

#### A) `POST /internal/media/auth` — MediaMTX Auth-Hook

MediaMTX ruft diesen Endpoint für jeden WHIP-Publish und WHEP-Read-Request auf:

```json
{
  "action": "publish",        // oder "read"
  "path": "vehicle-001",
  "ip": "...",
  "protocol": "webrtc",
  "query": "",
  "user": "",
  "password": "<bearer-token>"
}
```

Control Server Logik:
```go
switch req.Action {
case "publish":
    // Bearer-Token gegen WHIP_STREAM_KEY Env-Variable prüfen
    if req.Password != s.whipStreamKey { → 401 }
case "read":
    // JWT aus req.Password (oder Query-Param) validieren
    // Prüfen ob Operator eine aktive Session hat → 200 / 401
}
```

`WHIP_STREAM_KEY` kommt als Env-Variable (deploy.sh lädt aus SSM).
**Kein zweiter Auth-Mechanismus** — MediaMTX hat keine eigenen Credentials.

#### B) SAFE_MODE → MediaMTX API (direkte Kopplung im Control Server)

Beim Auslösen von SAFE_MODE in der State Machine (bestehender Pfad):

```go
// Nach s.triggerSafeMode(session):
go m.mediamtxClient.KickVehicle(session.VehicleID)
```

```go
// internal/mediamtx/client.go — neues Package
type Client struct { apiURL string }

func (c *Client) KickVehicle(vehicleID string) {
    // GET /v3/paths/get/{vehicleID} → WebRTC Session IDs
    // DELETE /v3/webrtcsessions/{id} für jede Session
}
```

Neue Env-Variable im Control Server: `MEDIAMTX_API_URL=http://mediamtx:9997`

**Pion SFU erhält keinen MediaMTX-Auftrag** — er bleibt reiner State-Subscriber.

---

### STREAM-06 — TURN in MediaMTX ICE-Config + Compose

MediaMTX nutzt dieselben TURN-Credentials wie coturn.
`TURN_USER` und `TURN_PASSWORD` kommen aus SSM (bereits in deploy.sh geladen).

**In `docker-compose.yml` und `docker-compose.prod.yml`:**
```yaml
mediamtx:
  environment:
    TURN_USER: ${TURN_USER:-avoc}
    TURN_PASSWORD: ${TURN_PASSWORD}
```

Kein neuer SSM-Eintrag. Bestehende Parameter werden wiederverwendet.

---

### STREAM-07 — CDK Port 8889 + SSM whip-stream-key

**`infrastructure/AWS/cdk_server-stack.ts`:**
```typescript
// MediaMTX WHIP — Larix Broadcaster direkt (kein nginx Proxy für Ingress)
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8889), "MediaMTX WHIP/WHEP");
```

**Neuer SSM Parameter:**
```
/avoc/prod/whip-stream-key   SecureString, min 32 Zeichen
```

**`scripts/setup-ssm.sh`** neuer Eintrag:
```bash
put_secure /avoc/prod/whip-stream-key "<STREAM_KEY_MIN_32_ZEICHEN>"
```

---

### STREAM-08 — `docker-compose.prod.yml` + `deploy.sh`

**Neuer Service in `docker-compose.prod.yml`:**
```yaml
mediamtx:
  image: bluenviron/mediamtx:latest
  ports:
    - "8889:8889"
    - "9997:9997"
  volumes:
    - ./mediamtx/mediamtx.yml:/mediamtx.yml:ro
  environment:
    TURN_USER: ${TURN_USER}
    TURN_PASSWORD: ${TURN_PASSWORD}
  networks:
    - avoc-net
  restart: unless-stopped
  depends_on:
    - control-server
    - stun-turn
```

**`control-server` Service bekommt neue Env-Variablen:**
```yaml
control-server:
  environment:
    WHIP_STREAM_KEY: ${WHIP_STREAM_KEY}    # NEU
    MEDIAMTX_API_URL: http://mediamtx:9997  # NEU
```

**`scripts/deploy.sh`** — ein neues SSM-Param:
```bash
export WHIP_STREAM_KEY=$(get_secure /avoc/prod/whip-stream-key)
```

mediamtx.yml auf EC2 kopieren (via deploy.sh S3-Sync oder separates scp).

---

### STREAM-09 — Larix Setup Guide + E2E Test

**Neue Datei: `docs/deployment/larix-setup.md`**

Inhalt:
- WHIP-URL: `http://<ELASTIC_IP>:8889/vehicle-001/whip`
- Authorization Header: `Bearer <WHIP_STREAM_KEY>`
- Codec: H.264 Baseline, 720p, 2 Mbit/s empfohlen
- Schritt-für-Schritt in Larix: Connections → Add → WHIP → URL + Bearer Token

**E2E Smoke Test (5 Schritte):**
1. Larix startet → MediaMTX zeigt `vehicle-001` als aktiven Path (`GET /v3/paths/list`)
2. Auth-Hook wird für Publish aufgerufen → Control Server antwortet 200
3. Operator-Browser öffnet Session → VideoPanel: MEDIA_NEGOTIATING → MEDIA_CONNECTED
4. Emergency Stop → SAFE_MODE → Control Server ruft MediaMTX API → Video stoppt beim Operator
5. Nach Recovery: Video-Reconnect via Retry-Button möglich

---

## Sprint-Ziel / Definition of Done

- [ ] ADR-020 dokumentiert (`docs/adr/020-mediamtx-whip-whep.md`)
- [ ] MediaMTX startet in Docker Compose, WHIP-Endpunkt erreichbar auf Port 8889
- [ ] Larix (Smartphone, 5G) streamt erfolgreich zu MediaMTX
- [ ] Operator-Browser empfängt Video via WHEP — `MEDIA_CONNECTED` sichtbar
- [ ] SAFE_MODE stoppt Video (Control Server → MediaMTX API — kein SFU-Umweg)
- [ ] Ein Auth-Mechanismus: alle WHIP/WHEP-Requests via `externalAuthenticationURL` → Control Server
- [ ] TURN-Credentials konfiguriert (ICE für Operator hinter NAT)
- [ ] CDK Port 8889 offen, SSM `whip-stream-key` angelegt
- [ ] Larix Setup Guide vorhanden (`docs/deployment/larix-setup.md`)
- [ ] Safety Regression: 19/19 ✅ (kein Safety-Code geändert)
