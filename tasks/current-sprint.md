# Sprint 10 — Browser WebRTC ICE Migration ✅

Ziel: WHEP-basierter Browser-Videoempfang funktioniert zuverlässig auch auf 5G/LTE (CGNAT).
Drei Root Causes aus der Sprint-9-Debugging-Phase werden behoben.

Datum: 2026-06-10 | **Status: Deployed ✅** (12/12 Container Up auf EC2 `18.196.24.10`)
Vorgänger: Sprint 9 ✅ (MediaMTX WHIP/WHEP Video Stream — ADR-020)

Vollständige Dokumentation: [`docs/sprints/sprint-10-webrtc-ice-migration.md`](../docs/sprints/sprint-10-webrtc-ice-migration.md)

---

## Root Cause Analyse

| # | Problem | Ursache | Fix |
|---|---------|---------|-----|
| 1 | Candidate Explosion | `webrtcIPsFromInterfaces` fehlte → alle Docker-IPs annonciert | `webrtcIPsFromInterfaces: false` + `webrtcAdditionalHosts: [EC2-EIP]` |
| 2 | Srflx auf gesperrten Ports | `webrtcICEServers2` → MediaMTX gathert eigene Candidates auf ephemeren Ports | `webrtcICEServers2` entfernt — nur Browser gathert |
| 3 | Pion DTLS-Client-Bug | Browser sendet `actpass`, Pion verarbeitet ServerHello nicht | `a=setup:actpass → active` in `useWebRTC.ts` |
| 4 | Kein TURN-Fallback | `useWebRTC.ts` nur STUN — kein TURN bei symmetrischem NAT (5G) | `/api/ice-config` Endpoint + TURN UDP + TURN TCP |
| 5 | coturn relay-ip leer | `deploy.sh` nutzte IMDSv1, Amazon Linux 2023 erfordert IMDSv2 | IMDSv2 Token-Header in deploy.sh |

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| WEBRTC-01 | CDK Security Group: 3478 (TCP+UDP), 8189 (UDP), 49152–65535 (UDP) | S | ✅ |
| WEBRTC-02 | `mediamtx.yml`: `webrtcIPsFromInterfaces: false`, ICEServers2 entfernen, Port 8189 | S | ✅ |
| WEBRTC-03 | `docker-compose.prod.yml`: coturn `network_mode: host`, relay-ip, external-ip | M | ✅ |
| WEBRTC-04 | `docker-compose.prod.yml`: mediamtx UDP-Port 8889 → 8189 | S | ✅ |
| WEBRTC-05 | `scripts/deploy.sh`: `TURN_PRIVATE_IP` aus EC2 IMDS (IMDSv2) | S | ✅ |
| WEBRTC-06 | control-server: `GET /ice-config` — ICE-Server-Liste mit TURN-Credentials | M | ✅ |
| WEBRTC-07 | `docker-compose.prod.yml`: control-server bekommt `TURN_USER` + `TURN_PASSWORD` | S | ✅ |
| WEBRTC-08 | `useWebRTC.ts`: DTLS-Fix, TURN-ICE-Server, `/api/ice-config` fetch | M | ✅ |
| WEBRTC-09 | Deploy auf EC2; E2E Smoke Test | M | ✅ deployed, E2E offen |

---

## Definition of Done

- [x] CDK/SG: Port 3478 (TCP+UDP), 8189 (UDP), 49152–65535 (UDP) offen
- [x] mediamtx: `webrtcIPsFromInterfaces: false`, `webrtcAdditionalHosts: ['18.196.24.10']`, Port 8189
- [x] coturn: `network_mode: host`, `relay-ip=10.0.33.191`, `external-ip=18.196.24.10/10.0.33.191`
- [x] `GET /api/ice-config` (nginx) liefert STUN + TURN UDP + TURN TCP — extern verifiziert
- [x] Alle 12 Container Up auf EC2 `18.196.24.10`
- [x] WHEP Auth-Hook: 401 ohne Session — verifiziert
- [x] Frontend `http://18.196.24.10:3000/` → HTTP 200 — extern verifiziert
- [x] 31/31 TypeScript Unit-Tests, Go Unit-Tests grün
- [ ] Browser WiFi: `srflx`-ICE-Pair — ausstehend (WHIP-Quelle nötig)
- [ ] Browser 5G: `relay`-ICE-Pair via TURN — ausstehend
- [ ] `MEDIA_CONNECTED` im Operator-UI — ausstehend
