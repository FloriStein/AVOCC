# ADR-019: Deployment-Strategie — Docker Hub + AWS EC2 + SSM Parameter Store

Status: Accepted

## Kontext

Nach 7 abgeschlossenen Sprints ist das AVOC-System vollständig implementiert und lokal lauffähig
(`docker compose up`). Der nächste Schritt ist die Validierung auf realer Cloud-Infrastruktur:
eine AWS EC2-Instanz mit statischer Elastic IP soll das System produktionsnahe testen.

Folgende Rahmenbedingungen sind gegeben:

- EC2-Instanz existiert bereits (t3.micro, Amazon Linux 2023, CDK-verwaltet)
- Elastic IP ist statisch und bekannt
- Quellcode soll **nicht** auf der EC2-Instanz liegen — nur Docker-Images aus einer Registry
- Secrets (JWT_SECRET, TURN-Credentials, Grafana-Passwort) dürfen **nicht** als `.env`-Datei
  auf der Instanz existieren — Anforderung aus dem Betriebskonzept
- Ziel: ein einzelner Deployment-Befehl genügt für einen Rollout

---

## Optionen

### Option A: Docker Hub (private Repositories)

**Vorteile:**
- Etablierter Standard, einfache `docker login` + `docker push/pull` Workflows
- Private Repositories verbergen Image-Inhalte vor Dritten
- Kein AWS-spezifisches CLI für Image-Pull nötig (nur `docker login`)
- Unabhängig vom Cloud-Provider — Images wären auch in anderen Umgebungen nutzbar

**Nachteile:**
- Separate Credentials für Docker Hub (Registry-Passwort muss auf EC2 vorliegen)
- Rate Limits bei Pull (mit Login: 200 Pulls/6h auf Free Plan)
- Nicht nativ in AWS IAM integriert

### Option B: AWS Elastic Container Registry (ECR)

**Vorteile:**
- Native IAM-Integration — EC2 Instance Profile reicht für Pull (kein separates Passwort)
- Kein Rate Limit
- VPC Endpoint möglich (kein Internet-Traffic für Pull)

**Nachteile:**
- AWS-Lock-in — Images nicht ohne AWS CLI aus anderen Umgebungen nutzbar
- ECR-Token läuft nach 12h ab — Login-Refresh in deploy.sh erforderlich
- Höhere initiale Komplexität

### Option C: Quellcode auf EC2 klonen + lokal bauen

**Nachteile:**
- Build-Dauer auf t3.micro sehr lang (10+ Go-Services + Frontend)
- Entwicklungsabhängigkeiten (Go, Node, protoc) auf der Instanz erforderlich
- Bruch des "kein Quellcode auf Produktion"-Prinzips
- Nicht reproduzierbar (Build-Ergebnis hängt vom State der Instanz ab)

---

## Entscheidung

Wir wählen **Option A: Docker Hub mit privaten Repositories**.

Begründung: Für die Testphase ist Docker Hub der einfachste Weg mit dem geringsten
Einrichtungsaufwand. Provider-Unabhängigkeit ist ein Vorteil (gleiche Images lokal und in
der Cloud). Bei Eskalation in Richtung Produktion kann zu ECR migriert werden (ADR-020 möglich).

---

## Secrets-Strategie

**Entschieden:** AWS Systems Manager (SSM) Parameter Store.

Ausgeschlossen:
- `.env`-Datei auf der Instanz — zu leicht vergessen, in Backups oder Logs landend
- Docker Secrets — bei Compose ohne Swarm aufwändig und nicht transparenter als SSM
- Hardcodiert — inakzeptabel

**SSM-Parameterstruktur:**

| SSM-Pfad | Inhalt | Typ |
|---|---|---|
| `/avoc/prod/jwt-secret` | JWT Signing Secret (≥32 Zeichen) | SecureString |
| `/avoc/prod/turn-external-ip` | Elastic IP der EC2-Instanz | String |
| `/avoc/prod/turn-realm` | TURN Realm (z. B. `avoc.example.com`) | String |
| `/avoc/prod/turn-user` | TURN Benutzername | String |
| `/avoc/prod/turn-password` | TURN Passwort | SecureString |
| `/avoc/prod/grafana-admin-user` | Grafana Admin-Benutzername | String |
| `/avoc/prod/grafana-admin-password` | Grafana Admin-Passwort | SecureString |
| `/avoc/prod/docker-username` | Docker Hub Benutzername | String |
| `/avoc/prod/docker-password` | Docker Hub Passwort / Access Token | SecureString |

`deploy.sh` holt diese Werte via `aws ssm get-parameter --with-decryption` und exportiert sie
als Shell-Env-Variablen vor dem `docker compose up`. Die EC2-Instanz trägt ein IAM Instance
Profile mit Leseberechtigung auf `/avoc/*`.

---

## Image-Naming-Schema

```
docker.io/<DOCKER_USERNAME>/avoc-<service>:<version>

Beispiele:
  avoc-control-server:latest
  avoc-auth-service:1.0.0
  avoc-safety-service:latest
  avoc-telemetry-service:latest
  avoc-webrtc-sfu:latest
  avoc-frontend:latest
```

Plattform: **linux/amd64** für alle Custom-Images (EC2 ist x86_64).
Drittanbieter-Images (mosquitto, loki, promtail, grafana, coturn) werden direkt aus
ihren öffentlichen Registries gepullt — kein Re-Push nötig.

---

## Build-Pipeline (Makefile)

```
make build-prod   → docker buildx --platform linux/amd64 für alle 6 Images
make push         → build-prod + docker push für alle 6 Images
```

Deployment auf EC2:
```
scripts/setup-ssm.sh   → einmalig vom Dev-Rechner (SSM Parameter anlegen)
scripts/deploy.sh      → auf EC2 (SSM lesen → docker compose pull + up -d)
```

---

## Produktionskonfiguration (docker-compose.prod.yml)

Unterschiede zu `docker-compose.yml` (Entwicklung):

| Aspekt | Entwicklung | Produktion |
|--------|-------------|-----------|
| Services bauen | `build:` (Quellcode) | `image:` (aus Registry) |
| Secrets | `.env`-Datei | Shell-Env aus SSM via deploy.sh |
| coturn | Kein `external-ip` | `--external-ip=${TURN_EXTERNAL_IP}` als Command-Flag |
| Grafana Auth | Anonymous Admin (offen) | Login-Formular + Admin-Credentials aus SSM |

---

## Konsequenzen

### Positiv

- EC2 braucht nur Docker + AWS CLI — kein Build-Toolchain, kein Quellcode
- Reproducible Deployments — gleiche Image-Digests, unabhängig von der Instanz
- Rollback: `VERSION=1.0.0 make push` + `deploy.sh` auf EC2 mit altem Tag
- SSM Parameter Store ist auditierbar (CloudTrail) — welcher Prozess hat wann welchen Secret gelesen
- Provider-unabhängige Images (kein ECR-Lock-in für Testphase)

### Negativ

- Docker Hub Credentials müssen in SSM hinterlegt werden (ein weiterer Secret)
- Rate Limits Docker Hub Pull: mit Login 200 Pulls/6h (unkritisch für Testbetrieb)
- `linux/amd64` Cross-Compilation auf ARM-Dev-Rechnern via QEMU — langsamer als native
- t3.micro (1 GB RAM) ist knapp für 10+ Container — Monitoring der Speicherauslastung empfohlen

### Folge-Entscheidungen

| Thema | Referenz |
|-------|----------|
| Migration zu ECR für Produktivbetrieb | ADR-020 möglich |
| Audit Store Backup-Strategie (SQLite Volume auf S3) | ADR-018 Folge — S3-Bucket vorhanden (CDK) |
| HTTPS / TLS-Terminierung (Let's Encrypt / ACM) | Offen — für Testphase HTTP akzeptabel |
| MQTT-Authentifizierung (Mosquitto mit Passwort-File) | Offen — für Testphase ohne Auth |
