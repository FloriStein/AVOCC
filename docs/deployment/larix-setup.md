# Larix Broadcaster Setup Guide — AVOC WHIP Stream

Dieses Dokument beschreibt die Einrichtung von Larix Broadcaster auf einem Smartphone
als Fahrzeug-Kamera-Client. Larix streamt via WHIP direkt zu MediaMTX auf EC2.

Voraussetzung: EC2-Stack läuft (`bash ~/app/deploy.sh`), SSM-Parameter gesetzt (inkl. `whip-stream-key`).

---

## Übersicht

```
Smartphone (Larix Broadcaster, 5G)
  ↓ WHIP — HTTP POST mit SDP Offer
  ↓ Authorization: Bearer <WHIP_STREAM_KEY>
MediaMTX auf EC2 (Port 8889)
  ↓ Auth-Hook → Control Server
  ↓ WHEP für Operator-Browser
Operator UI (VideoPanel)
```

---

## Schritt 1 — WHIP_STREAM_KEY aus SSM auslesen

```bash
aws ssm get-parameter \
  --name /avoc/prod/whip-stream-key \
  --with-decryption \
  --region eu-central-1 \
  --query Parameter.Value \
  --output text
```

Diesen Wert notieren — er wird als Bearer Token in Larix eingetragen.

---

## Schritt 2 — Larix Broadcaster installieren

- **iOS**: App Store → „Larix Broadcaster"
- **Android**: Google Play → „Larix Broadcaster"

Larix ist kostenlos, WHIP-Support ist ohne Premium verfügbar.

---

## Schritt 3 — WHIP-Connection in Larix einrichten

1. Larix öffnen → **Settings** (Zahnrad-Icon)
2. **Connections** → **+** (neue Verbindung)
3. Protokoll: **WHIP**
4. **URL**: `http://<ELASTIC_IP>:8889/vehicle-001/whip`
   - `<ELASTIC_IP>` = Elastic IP der EC2-Instanz (CloudFormation-Output: `StreamingStack.PublicIP`)
   - `vehicle-001` = vehicleId (muss mit dem Pfad in `mediamtx.yml` übereinstimmen)
5. **Authorization**: `Bearer <WHIP_STREAM_KEY>` (aus Schritt 1)
6. **Save** → Connection speichern

---

## Schritt 4 — Stream-Parameter (empfohlen)

| Parameter | Empfehlung | Begründung |
|-----------|-----------|------------|
| Video Codec | H.264 Baseline | Maximale Kompatibilität mit MediaMTX |
| Auflösung | 720p (1280×720) | Balance Latenz/Qualität auf 5G |
| Bitrate | 2000 kbps | Stabile 5G-Übertragung |
| Keyframe Interval | 1–2 s | Niedrigere Latenz bei Keyframe-Request |
| Audio | Optional (deaktivieren für reine Video-Tests) | |

Einstellungen in Larix: **Settings → Video** und **Settings → Audio**

---

## Schritt 5 — Stream starten

1. Larix Hauptbildschirm → **REC**-Button (roter Kreis)
2. Larix zeigt Verbindungsstatus: `Connecting...` → `Live`
3. MediaMTX-Log auf EC2 prüfen (optional):
   ```bash
   docker compose -f ~/app/docker-compose.prod.yml logs mediamtx --tail=20
   ```
   Erwartete Ausgabe:
   ```
   INF [WebRTC/WHIP] session opened: path=vehicle-001
   ```

---

## Schritt 6 — Operator-Browser prüfen

1. Browser: `http://<ELASTIC_IP>:3000`
2. Login + Session starten
3. **VideoPanel** wechselt: `MEDIA_INIT` → `MEDIA_NEGOTIATING` → `MEDIA_CONNECTED`
4. Live-Video aus Larix erscheint im Dashboard

---

## Troubleshooting

**Larix zeigt „Connection failed":**
- EC2 Security Group: Port 8889 TCP offen? (`aws ec2 describe-security-groups`)
- MediaMTX gestartet? `docker compose -f ~/app/docker-compose.prod.yml ps mediamtx`
- WHIP-URL korrekt? Format: `http://<IP>:8889/vehicle-001/whip` (kein https, kein Slash am Ende)

**Auth-Fehler (401):**
- WHIP_STREAM_KEY in SSM prüfen
- Bearer-Token in Larix korrekt eingetragen? Kein Leerzeichen, kein Anführungszeichen
- Control Server Log: `docker compose logs control-server | grep "media auth"`

**Video stockt / hohe Latenz:**
- Bitrate reduzieren (1000–1500 kbps)
- 5G-Signal prüfen (Larix zeigt Bitrate-Statistiken)
- TURN-Verbindung aktiv? `docker compose logs mediamtx | grep "turn"`

**VideoPanel bleibt auf MEDIA_NEGOTIATING:**
- Browser-Console öffnen (F12): ICE-Fehler?
- TURN-Credentials in SSM korrekt? TURN_USER + TURN_PASSWORD
- MediaMTX ICE-Config prüfen: coturn erreichbar von MediaMTX-Container?

---

## Rollout neuer Vehicle-IDs (Sprint 10)

Aktuell ist `vehicle-001` der einzige konfigurierte Pfad in MediaMTX (`mediamtx.yml`).
Für mehrere Fahrzeuge: `mediamtx.yml` Pfad-Regex anpassen + vehicleId dynamisch aus
Session-Assign laden (ADR-020-Folge).

---

## E2E Smoke Test Protokoll

| # | Schritt | Erwartetes Ergebnis | Status |
|---|---------|---------------------|--------|
| 1 | Larix startet WHIP-Stream | MediaMTX-Log: `session opened: path=vehicle-001` | 🔲 |
| 2 | Auth-Hook wird aufgerufen | Control Server-Log: `media auth: WHIP publish allowed` | 🔲 |
| 3 | Operator-Browser öffnet Session | VideoPanel: `MEDIA_NEGOTIATING` | 🔲 |
| 4 | WHEP-Verbindung aufgebaut | VideoPanel: `MEDIA_CONNECTED`, Video sichtbar | 🔲 |
| 5 | Emergency Stop drücken | System: `SAFE_MODE`; Control Server-Log: `mediamtx: subscriber kicked` | 🔲 |
| 6 | Larix-Stream noch aktiv | MediaMTX-Pfad `vehicle-001` weiterhin published | 🔲 |
| 7 | Nach Recovery: Retry-Button | VideoPanel: `MEDIA_CONNECTED` (reconnect erfolgreich) | 🔲 |
