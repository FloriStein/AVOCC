# Sprint 10 — Browser WebRTC ICE Migration

Stand: 2026-06-10
Status: Deployed — E2E Smoke Test ausstehend (WHIP-Quelle benötigt)

---

## Ziel

WHEP-basierter Browser-Videoempfang funktioniert zuverlässig — auch auf 5G / LTE (CGNAT).
Drei Root Causes aus der Sprint-9-Debugging-Phase werden behoben, ohne die bestehende
AVOC-Architektur (Auth-Hook, SAFE_MODE, Docker Compose) zu verändern.

Referenz: [`docs/deployment/network_migration.md`](../deployment/network_migration.md)

---

## Root Cause Analyse

| # | Problem | Symptom | Ursache |
|---|---------|---------|---------|
| 1 | **Candidate Explosion** | ICE-Timeout 30–60 s | `webrtcIPsFromInterfaces` fehlt → MediaMTX annonciert alle Docker-Interfaces (`172.x`, `10.x`, `127.x`) als Host-Candidates; Browser testet hunderte nutzlose ICE-Pairs |
| 2 | **Srflx auf gesperrten Ports** | ICE fail nach Default-Timeout | `webrtcICEServers2` → MediaMTX gathert eigene srflx-Candidates auf ephemeren Ports; diese Ports sind nicht in der Security Group → alle Pairs scheitern |
| 3 | **Pion DTLS-Client-Bug** | Stream startet nie; Retransmit-Loop in Logs | Browser sendet `a=setup:actpass` → MediaMTX antwortet `active` → Pion v1.19.0 als DTLS-Client verarbeitet ServerHello nicht korrekt → retransmits ClientHello bis Timeout |
| 4 | **Fehlende TURN-Fallback-Candidates im Browser** | ICE fail bei symmetrischem NAT (5G) | `useWebRTC.ts` konfiguriert nur STUN (Port 3479, falsch) — kein TURN UDP, kein TURN TCP |
| 5 | **coturn relay-ip / external-ip Format** | Relay-Candidates nicht erreichbar von extern | `--external-ip=PUBLIC` ohne `PUBLIC/PRIVATE`-Mapping → coturn advertised falsche Relay-Adresse auf AWS (EC2 sieht nur private IP) |

---

## Scope

Ausschließlich Infrastructure-Fixes und ICE-Konfiguration. Keine neuen Features.

**Nicht in diesem Sprint:**
- Multi-Vehicle-Routing (ADR-020-Backlog)
- HTTPS / TLS-Terminierung (Backlog post-Sprint-8)
- WHIP-Publish aus Browser (Larix bleibt Referenz-Publisher)

---

## Tasks

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|----------------|
| WEBRTC-01 | CDK Security Group: 3479 → 3478 (coturn host), UDP 8189 (ICE mux), Relay 49152–65535 | S | — |
| WEBRTC-02 | `mediamtx.yml`: `webrtcIPsFromInterfaces: false`, `webrtcICEServers2` entfernen, `webrtcHandshakeTimeout: 60s`, UDP-Port → 8189 | S | — |
| WEBRTC-03 | `docker-compose.prod.yml`: coturn `network_mode: host`, `relay-ip`, `external-ip=PUBLIC/PRIVATE`, Relay-Range 49152–65535 | M | WEBRTC-01 |
| WEBRTC-04 | `docker-compose.prod.yml`: mediamtx UDP-Port 8889 → 8189 | S | WEBRTC-02 |
| WEBRTC-05 | `scripts/deploy.sh`: `TURN_PRIVATE_IP` aus EC2-Instance-Metadata | S | WEBRTC-03 |
| WEBRTC-06 | control-server: `GET /api/ice-config` — liefert ICE-Server-Liste mit TURN-Credentials | M | — |
| WEBRTC-07 | `docker-compose.prod.yml`: control-server bekommt `TURN_USER` + `TURN_PASSWORD` aus SSM | S | WEBRTC-06 |
| WEBRTC-08 | `useWebRTC.ts`: DTLS-Fix (`actpass → active`), TURN-ICE-Server (UDP + TCP, Port 3478), `/api/ice-config` fetch, 5s-Gathering-Timeout | M | WEBRTC-06 |
| WEBRTC-09 | `cdk deploy` + `deploy.sh` auf EC2; E2E Smoke Test (WiFi + 5G) | M | WEBRTC-01–08 |

---

## Abhängigkeitspfad

```
WEBRTC-01 (CDK SG) ──────────────────────────────────┐
WEBRTC-02 (mediamtx.yml) ────────────────────────────┤
WEBRTC-03 (coturn host mode) ──▶ WEBRTC-05 (deploy.sh)┤
WEBRTC-04 (mediamtx UDP port) ───────────────────────┤
WEBRTC-06 (ice-config endpoint) ─▶ WEBRTC-07 (env)   ┤
                                 ─▶ WEBRTC-08 (hook)  ┤
                                                       ▼
                                               WEBRTC-09 (deploy + test)
```

---

## Technische Details pro Task

### WEBRTC-01 — CDK Security Group

Datei: `infrastructure/AWS/cdk_server-stack.ts`

```typescript
// Entfernen:
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3479), "coturn TCP");
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(3479), "coturn UDP");
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(8889), "MediaMTX WebRTC ICE UDP");
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udpRange(49160, 49200), "TURN relay");

// Hinzufügen:
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3478), "coturn TCP");
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(3478), "coturn UDP");
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(8189), "MediaMTX ICE mux UDP");
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udpRange(49152, 65535), "TURN relay");
```

→ `npx cdk deploy` — nur Security Group Rules, kein EC2 Replacement.

---

### WEBRTC-02 — mediamtx.yml

Datei: `infrastructure/mediamtx/mediamtx.yml`

```yaml
# Hinzufügen:
webrtcIPsFromInterfaces: false      # nur webrtcAdditionalHosts wird annonciert
webrtcHandshakeTimeout: 60s         # default 10s zu kurz bei TURN-Relay-Pfad

# Entfernen:
webrtcLocalUDPAddress: :8889        # → Default :8189 (ICE mux auf eigeständigem Port)
webrtcICEServers2: [...]            # komplett entfernen — kein server-side STUN/TURN gathering

# Behalten:
webrtcAddress: :8889                # HTTP Signaling (WHIP/WHEP) — unverändert
webrtcAdditionalHosts: ["${TURN_EXTERNAL_IP}"]  # einziger Host-Candidate = EC2 Elastic IP
webrtcAllowOrigins: ["*"]           # CORS — in prod via nginx eingeschränkt
authMethod: http
authHTTPAddress: http://control-server:8080/internal/media/auth
```

**Warum kein `webrtcICEServers2`:**
MediaMTX soll keine eigenen STUN/TURN-Candidates gathern. Der einzige ICE-Candidate von
MediaMTX ist der Host-Candidate `18.196.24.10:8189`. Der Browser gathert seinen eigenen
Candidate-Satz (host + srflx + relay) über die ICE-Server aus `/api/ice-config`.
Die Kombination `browser-srflx ↔ mediamtx-host` oder `browser-relay ↔ mediamtx-host` ergibt
die funktionierende ICE-Pair.

---

### WEBRTC-03 — docker-compose.prod.yml: coturn host mode

Datei: `infrastructure/compose/docker-compose.prod.yml`

```yaml
stun-turn:
  image: coturn/coturn:latest
  network_mode: host          # kein bridge — relay-Sockets binden direkt an EC2-Private-IP
  restart: unless-stopped
  command: >
    --listening-port=3478
    --relay-ip=${TURN_PRIVATE_IP}
    --external-ip=${TURN_EXTERNAL_IP}/${TURN_PRIVATE_IP}
    --realm=${TURN_REALM}
    --user=${TURN_USER}:${TURN_PASSWORD}
    --min-port=49152
    --max-port=65535
    --fingerprint
    --lt-cred-mech
    --no-cli
    --log-file=stdout
    --verbose
  # Kein ports: — host mode bindet direkt an alle Ports auf dem EC2-Host
```

**Warum `network_mode: host` nur für coturn:**
Bridge-Networking würde `49152-65535:49152-65535/udp` (16.384 Port-Mappings) erfordern.
MediaMTX bleibt in Bridge-Networking damit Docker-DNS (`control-server:8080`, `mediamtx:9997`)
weiterhin funktioniert.

**Warum `relay-ip=PRIVATE` + `external-ip=PUBLIC/PRIVATE`:**
AWS-EC2 kennt die eigene Elastic IP nicht auf einem Netzwerk-Interface (kein Hairpin-NAT).
coturn muss Relay-Sockets an die private IP binden (`relay-ip=10.x.x.x`) und dabei
die öffentliche IP advertisen (`external-ip=18.196.24.10/10.x.x.x`).

---

### WEBRTC-04 — docker-compose.prod.yml: mediamtx UDP Port

Datei: `infrastructure/compose/docker-compose.prod.yml`

```yaml
mediamtx:
  ports:
    - "8889:8889/tcp"   # HTTP Signaling — unverändert
    - "8189:8189/udp"   # ICE UDP mux — war 8889/udp
    - "9997:9997"       # Management API — unverändert
```

---

### WEBRTC-05 — deploy.sh: TURN_PRIVATE_IP

Datei: `scripts/deploy.sh`

```bash
# IMDSv2 (Amazon Linux 2023 erfordert Token-Header):
_IMDS_TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
export TURN_PRIVATE_IP=$(curl -sf -H "X-aws-ec2-metadata-token: ${_IMDS_TOKEN}" \
  http://169.254.169.254/latest/meta-data/local-ipv4)
echo "  TURN_PRIVATE_IP     ${TURN_PRIVATE_IP}"
```

Amazon Linux 2023 erzwingt IMDSv2 (Token-Header Pflicht). Das initiale `curl -sf http://169.254.169.254/...`
ohne Token gibt leere Response zurück → `TURN_PRIVATE_IP` wäre leer → coturn startet mit
`--relay-ip=` und `--external-ip=18.196.24.10/` (broken).

deploy.sh läuft auf der EC2-Instanz → IMDS direkt erreichbar, kein SSM-Parameter nötig.

---

### WEBRTC-06 — control-server: GET /ice-config

Neuer HTTP-Endpunkt im Control Server (kein Auth erforderlich — TURN-Credentials sind
per Design öffentlich sobald jemand die Seite lädt).

**Route-Präfix-Konvention:** nginx strippt `/api/` vor dem Forwarding
(`rewrite ^/api/(.*) /$1 break`). Intern in Go als `GET /ice-config` registriert —
der Browser ruft `/api/ice-config` auf, nginx leitet als `/ice-config` weiter.

```
GET /ice-config  (nginx-Proxy: Browser → /api/ice-config → control-server /ice-config)

Response 200:
{
  "iceServers": [
    { "urls": "stun:HOST:3478" },
    { "urls": "turn:HOST:3478",
      "username": "TURN_USER",
      "credential": "TURN_PASSWORD" },
    { "urls": "turn:HOST:3478?transport=tcp",
      "username": "TURN_USER",
      "credential": "TURN_PASSWORD" }
  ]
}
```

`HOST` = `TURN_EXTERNAL_IP` Env-Var (bereits vorhanden in deploy.sh).
`TURN_USER`, `TURN_PASSWORD` = neue Env-Vars für control-server (WEBRTC-07).

Begründung für API-Endpunkt (statt Build-Time Vite-Env):
- TURN-Credentials werden nicht in das Frontend-Bundle eingebettet
- Credentials können rotiert werden ohne Frontend-Rebuild
- Frontend-Image bleibt wiederverwendbar über Deployments

---

### WEBRTC-07 — docker-compose.prod.yml: control-server env

Datei: `infrastructure/compose/docker-compose.prod.yml`

```yaml
control-server:
  environment:
    # ... bestehende Envs ...
    TURN_EXTERNAL_IP: ${TURN_EXTERNAL_IP}   # bereits in deploy.sh
    TURN_USER: ${TURN_USER}                  # bereits in SSM + deploy.sh
    TURN_PASSWORD: ${TURN_PASSWORD}          # bereits in SSM + deploy.sh
```

Kein neuer SSM-Parameter — alle drei Werte werden bereits in deploy.sh exportiert.

---

### WEBRTC-08 — useWebRTC.ts: drei Korrekturen

Datei: `frontend/src/hooks/useWebRTC.ts`

**1. ICE-Config aus API fetchen:**
```typescript
const iceRes = await fetch('/api/ice-config')
const { iceServers } = await iceRes.json()
const pc = new RTCPeerConnection({ iceServers })
```

**2. DTLS-Fix (Pion-Bug):**
```typescript
const offer = await pc.createOffer()
// Pion v1.19.0 DTLS-Client-Bug: Browser muss DTLS-Client sein, MediaMTX DTLS-Server.
// actpass → active erzwingt: Browser sendet ClientHello, MediaMTX antwortet ServerHello.
const fixedSdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active')
await pc.setLocalDescription({ type: 'offer', sdp: fixedSdp })
```

**3. ICE-Gathering Timeout (5s Safety):**
```typescript
await new Promise<void>(resolve => {
  if (pc.iceGatheringState === 'complete') { resolve(); return }
  const tid = setTimeout(resolve, 5000)    // Safety: max 5s warten
  pc.addEventListener('icegatheringstatechange', function handler() {
    if (pc.iceGatheringState === 'complete') {
      clearTimeout(tid)
      pc.removeEventListener('icegatheringstatechange', handler)
      resolve()
    }
  })
})
```

Der bestehende `iceGatheringState`-Wait (bereits in Sprint 9 implementiert) wird mit dem
5s-Timeout erweitert. Der DTLS-Fix muss **vor** `setLocalDescription` stehen.

---

### WEBRTC-09 — Deploy & Smoke Test

```bash
# 1. CDK deploy (Security Group update):
cd infrastructure/cdk && npx cdk deploy

# 2. Config-Files auf EC2 deployen:
# mediamtx.yml via SSM (botocore send_command)
# docker-compose.prod.yml via SSM

# 3. Full Stack restart:
bash ~/app/deploy.sh

# 4. Smoke Test:
# a) Browser auf WiFi: WHEP verbindet → MEDIA_CONNECTED in UI
# b) Browser auf 5G:   WHEP verbindet via TURN-Relay → relay-Candidate in about:webrtc sichtbar
# c) coturn logs: keine 401-Fehler mehr
# d) MediaMTX logs: 1 ICE-Candidate (18.x.x.x:8189), kein srflx
```

---

## Sprint DoD

- [x] CDK: Port 3478 (TCP+UDP), 8189 (UDP), 49152–65535 (UDP) offen in Security Group — via `aws ec2 authorize-security-group-ingress` (CDK deploy übersprungen wegen Subnet-AZ-Drift)
- [x] MediaMTX: `webrtcIPsFromInterfaces: false`, `webrtcAdditionalHosts: ['18.196.24.10']`, `webrtcLocalUDPAddress: :8189` — verifiziert via `/v3/config/global/get`
- [x] coturn: läuft in `network_mode: host`; Port 3478 TCP erreichbar; `relay-ip=10.0.33.191`, `external-ip=18.196.24.10/10.0.33.191` — verifiziert via `docker inspect`
- [x] `GET /api/ice-config` (nginx) → STUN + TURN UDP + TURN TCP mit korrekter EC2-IP — verifiziert von extern (`curl http://18.196.24.10:3000/api/ice-config`)
- [x] Alle 12 Services UP: control-server, frontend, auth, safety, telemetry, webrtc-sfu, mediamtx, stun-turn, mosquitto, loki, promtail, grafana
- [x] WHEP Auth-Hook: 401 bei fehlendem Token/Session — verifiziert via `curl`
- [x] Frontend: HTTP 200 von `http://18.196.24.10:3000/` — extern erreichbar
- [x] 31/31 TypeScript Unit-Tests pass; Go Unit-Tests pass
- [ ] Browser (WiFi): WHEP verbindet in <5s; `srflx`-Pair in `about:webrtc` — **ausstehend (WHIP-Quelle nötig)**
- [ ] Browser (5G/LTE): WHEP verbindet via TURN-Relay; `relay`-Candidate in `about:webrtc` — **ausstehend**
- [ ] `MEDIA_CONNECTED` State sichtbar im Operator-UI — **ausstehend**
- [ ] DTLS kein Retransmit-Loop — **verifizierbar erst mit aktivem WHIP-Stream**
- [ ] coturn logs: keine 401 nach TURN-Allocate — **verifizierbar erst bei Browser-Verbindung**

---

## ADR-Auswirkungen

Kein neues ADR erforderlich — dies ist eine Konfigurationskorrektur, keine Architekturentscheidung.

**ADR-020** erhält einen Nachtrag: Die ursprüngliche ICE-Konfiguration mit
`webrtcICEServers2` war fehlerhaft (srflx auf gesperrten ephemeren Ports). Die korrekte
Konfiguration lässt MediaMTX ohne server-side STUN/TURN-Gathering arbeiten; der Browser
übernimmt vollständig das eigene ICE-Candidate-Gathering über `/api/ice-config`.

---

## Lokales Dev (unverändert)

`docker-compose.yml` (local dev) bleibt unberührt. Der DTLS-Fix in `useWebRTC.ts` wirkt
auch lokal; lokales coturn bleibt bridge-networking (kein AWS-NAT-Problem).
Für lokale TURN-Tests: `.env` `TURN_EXTERNAL_IP` auf LAN-IP setzen.

---

## Deployment-Protokoll — 2026-06-10

### Bugs gefunden & behoben während Deploy

| # | Bug | Ursache | Fix |
|---|-----|---------|-----|
| B1 | `TURN_PRIVATE_IP` leer | `deploy.sh` nutzte IMDSv1; Amazon Linux 2023 erfordert IMDSv2 | `curl -X PUT .../api/token` + Token-Header in IMDS-Aufruf |
| B2 | loki/promtail crash loop `is a directory` | `deploy.sh` erster Run bevor Config-Files hochgeladen → Docker erstellte Verzeichnis statt File-Bind | Container stoppen, falsche Verzeichnisse `rm -rf`, Config-Files hochladen, neu starten |
| B3 | `/api/ice-config` → 404 | Route als `GET /api/ice-config` registriert, aber nginx strippt `/api/`-Präfix vor Forwarding | Route umbenannt zu `GET /ice-config` in `cmd/control-server/main.go` |
| B4 | mediamtx startet nicht (erster Deploy) | Port 9997 durch `streaming-mediamtx-1` belegt | `streaming-platform` Container gestoppt, danach AVOC-Stack gestartet |
| B5 | Grafana Provisioning-Verzeichnisse fehlten | `docker-compose.prod.yml` referenziert `../grafana/provisioning` (relativ zu `~/app`), aber Pfad `~/grafana/provisioning` existierte nicht auf EC2 | Verzeichnisse angelegt, Config-Files hochgeladen |

### Deployment-Sequenz (tatsächlich durchgeführt)

1. Streaming-Platform gestoppt (`cd /opt/streaming && docker compose down`)
2. Config-Files per SCP hochgeladen: `mediamtx.yml`, `docker-compose.prod.yml`, `mosquitto.conf`, `deploy.sh`, `loki.yml`, `promtail.yml`, Grafana Provisioning
3. `deploy.sh` korrigiert (IMDSv2) und auf EC2 deployed
4. AVOC-Images neu gebaut und gepusht: `avoc-control-server:latest`, `avoc-frontend:latest`
5. Finaler `deploy.sh` run → alle 12 Container Up

### Verifizierte Endpoints (2026-06-10, 19:23 UTC)

| Endpoint | Ergebnis |
|----------|---------|
| `http://18.196.24.10:3000/` | HTTP 200 ✓ |
| `http://18.196.24.10:3000/api/ice-config` | JSON mit STUN+TURN UDP+TURN TCP ✓ |
| `POST http://18.196.24.10:3000/whep/vehicle-test/whep` (kein Token) | HTTP 401 ✓ |
| `http://localhost:8080/health` | `{"status":"ok","service":"control-server"}` ✓ |
| `http://localhost:8081/health` | `{"status":"ok","service":"auth-service"}` ✓ |
| `http://localhost:8082/health` | `{"status":"ok","service":"safety-service"}` ✓ |
| `http://localhost:8083/health` | `{"status":"ok","service":"telemetry-service"}` ✓ |
| `http://localhost:8084/health` | `{"status":"ok","service":"webrtc-sfu"}` ✓ |
| Port 3478 TCP (coturn) von extern | OPEN ✓ |
| Port 8889 TCP (mediamtx WHIP/WHEP) von extern | OPEN ✓ |
| mediamtx `webrtcIPsFromInterfaces` | `false` ✓ |
| mediamtx `webrtcAdditionalHosts` | `['18.196.24.10']` ✓ |
| mediamtx `webrtcLocalUDPAddress` | `:8189` ✓ |
| coturn `relay-ip` | `10.0.33.191` ✓ |
| coturn `external-ip` | `18.196.24.10/10.0.33.191` ✓ |
| Go Unit-Tests | alle pass ✓ |
| TypeScript Unit-Tests | 31/31 pass ✓ |

### Offen: E2E Smoke Test

Erfordert eine aktive WHIP-Quelle (z.B. Larix Broadcaster oder gstreamer WHIP-Client),
die auf `http://18.196.24.10:8889/vehicle-001` published. Danach:

```bash
# Browser öffnen:
http://18.196.24.10:3000/

# Login → Session starten → Video-Panel beobachten
# Erwarteter Status: MEDIA_NEGOTIATING → MEDIA_CONNECTED

# Chrome about:webrtc für ICE-Candidate-Typ prüfen:
# WiFi → srflx-Pair erwartet
# 5G   → relay-Pair erwartet (TURN)
```
