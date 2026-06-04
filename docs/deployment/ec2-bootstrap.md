# EC2 Bootstrap Guide — AVOC auf AWS EC2

Dieses Dokument beschreibt den kompletten Deployment-Prozess ab null:
vom frischen CDK-Deploy bis zum laufenden AVOC-Stack auf EC2.

Voraussetzung: CDK Stack ist deployed (`cdk deploy`), Elastic IP ist bekannt.

---

## Übersicht

```
Dev-Rechner                          EC2-Instanz
───────────────────                  ────────────────────────
1. cdk deploy          ──────────▶   EC2 + IAM + Security Group
2. setup-ssm.sh        ──────────▶   SSM Parameter Store (/avoc/prod/*)
3. make push           ──────────▶   Docker Hub (private Repos)
4. deploy.sh kopieren  ──────────▶   /home/ec2-user/app/
5.                                   deploy.sh ausführen → Stack läuft
```

---

## Schritt 1 — CDK Deploy

```bash
cd <cdk-projekt>
cdk deploy
```

Nach dem Deploy: Elastic IP aus dem CloudFormation-Output notieren.

```
Outputs:
  StreamingStack.PublicIP     = 1.2.3.4    ← Elastic IP
  StreamingStack.InstanceId   = i-0abc123
  StreamingStack.BucketName   = streamingstack-appbucket-xyz
```

---

## Schritt 2 — SSM Parameter anlegen (einmalig vom Dev-Rechner)

```bash
AWS_REGION=eu-central-1 bash scripts/setup-ssm.sh
```

Das Script fragt interaktiv nach allen Secrets und schreibt sie nach `/avoc/prod/`:

| SSM-Pfad | Inhalt |
|---|---|
| `/avoc/prod/jwt-secret` | JWT Signing Secret (≥32 Zeichen) |
| `/avoc/prod/turn-external-ip` | Elastic IP (z. B. `1.2.3.4`) |
| `/avoc/prod/turn-realm` | TURN Realm (z. B. `avoc.example.com`) |
| `/avoc/prod/turn-user` | TURN Benutzername |
| `/avoc/prod/turn-password` | TURN Passwort (≥16 Zeichen) |
| `/avoc/prod/grafana-admin-user` | Grafana Login |
| `/avoc/prod/grafana-admin-password` | Grafana Passwort (≥12 Zeichen) |
| `/avoc/prod/docker-username` | Docker Hub Benutzername |
| `/avoc/prod/docker-password` | Docker Hub Access Token |

Parameter prüfen:
```bash
aws ssm get-parameters-by-path --path /avoc/prod/ --region eu-central-1 \
  --query "Parameters[].Name" --output table
```

---

## Schritt 3 — Images bauen und nach Docker Hub pushen (Dev-Rechner)

```bash
# Docker Hub Username setzen (einmalig)
echo "dein-dockerhub-username" > .docker-username

# Alle 6 Images für linux/amd64 bauen und pushen
make push

# Oder mit explizitem Versionstag:
VERSION=1.0.0 make push
```

Der Befehl baut `linux/amd64`-Images für alle 5 Go-Services + Frontend
und pushed sie nach `docker.io/<USERNAME>/avoc-*`.

---

## Schritt 4 — Docker Compose Plugin auf EC2 installieren

Das UserData-Script im CDK installiert das Plugin automatisch bei neuen Instanzen.
Bei bestehenden Instanzen einmalig manuell ausführen (via AWS SSM Session Manager):

```bash
# SSM Session Manager öffnen (kein SSH nötig)
aws ssm start-session --target i-0abc123 --region eu-central-1
```

Im Session-Terminal:
```bash
sudo mkdir -p /usr/local/lib/docker/cli-plugins
sudo curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

# Prüfen:
docker compose version
# → Docker Compose version v2.x.x
```

---

## Schritt 5 — Deployment-Dateien auf EC2 kopieren

```bash
ELASTIC_IP=1.2.3.4

# deploy.sh und prod Compose auf die Instanz kopieren
scp scripts/deploy.sh                               ec2-user@${ELASTIC_IP}:~/app/
scp infrastructure/compose/docker-compose.prod.yml  ec2-user@${ELASTIC_IP}:~/app/
scp infrastructure/mosquitto/mosquitto.conf         ec2-user@${ELASTIC_IP}:~/app/mosquitto/
scp -r infrastructure/loki/                         ec2-user@${ELASTIC_IP}:~/app/
scp -r infrastructure/promtail/                     ec2-user@${ELASTIC_IP}:~/app/
scp -r infrastructure/grafana/                      ec2-user@${ELASTIC_IP}:~/app/
```

> Alternativ via AWS SSM Session Manager + S3-Bucket (kein SSH nötig):
> ```bash
> BUCKET=streamingstack-appbucket-xyz
> aws s3 cp scripts/deploy.sh s3://${BUCKET}/deploy.sh
> aws s3 cp infrastructure/compose/docker-compose.prod.yml s3://${BUCKET}/
> # ... auf EC2: aws s3 sync s3://${BUCKET}/ ~/app/
> ```

Verzeichnisstruktur auf EC2 (erwartet von `deploy.sh`):
```
/home/ec2-user/app/
├── docker-compose.prod.yml
├── deploy.sh
├── mosquitto/
│   └── mosquitto.conf
├── loki/
│   └── loki.yml
├── promtail/
│   └── promtail.yml
└── grafana/
    └── provisioning/
        ├── datasources/loki.yml
        └── dashboards/
            ├── dashboards.yml
            └── avoc.json
```

---

## Schritt 6 — Erster Deploy auf EC2

```bash
# SSM Session öffnen
aws ssm start-session --target i-0abc123 --region eu-central-1

# Im Session-Terminal:
cd ~/app
AWS_REGION=eu-central-1 VERSION=latest bash deploy.sh
```

Der Stack ist bereit, wenn alle Container `healthy` / `running` sind:
```bash
docker compose -f docker-compose.prod.yml ps
```

---

## Erreichbare Services nach Deploy

| Service | URL |
|---|---|
| Operator UI (Frontend) | `http://<ELASTIC_IP>:3000` |
| Control Server API | `http://<ELASTIC_IP>:8080` |
| MQTT Broker (Vehicle) | `<ELASTIC_IP>:1883` |
| Grafana | `http://<ELASTIC_IP>:3001` |
| STUN/TURN | `<ELASTIC_IP>:3479` |

---

## Rollback

```bash
# Auf EC2:
cd ~/app
VERSION=<alter-tag> AWS_REGION=eu-central-1 bash deploy.sh
```

---

## Troubleshooting

**Container startet nicht:**
```bash
docker compose -f docker-compose.prod.yml logs <service-name>
```

**SSM Parameter nicht lesbar:**
```bash
# IAM Instance Profile prüfen:
aws sts get-caller-identity
# Muss die EC2-Instance-Rolle zeigen, nicht den User
```

**coturn TURN relay funktioniert nicht:**
- Security Group: UDP 49160-49200 muss offen sein (CDK: `ec2.Port.udpRange(49160, 49200)`)
- `TURN_EXTERNAL_IP` in SSM prüfen: muss die Elastic IP sein (nicht die private IP)

**Grafana Login schlägt fehl:**
```bash
# Credentials aus SSM prüfen:
aws ssm get-parameter --name /avoc/prod/grafana-admin-user --region eu-central-1 \
  --query Parameter.Value --output text
```

**Speicher auf t3.micro knapp:**
```bash
free -h
docker stats --no-stream
```
Bei dauerhaft >80% RAM-Auslastung: Upgrade auf t3.small (CDK `InstanceSize.SMALL`).
Achtung: Instance-Replacement durch CloudFormation.

---

## Updates einspielen

```bash
# Dev-Rechner: neue Images bauen und pushen
VERSION=1.1.0 make push

# EC2: neue Version deployen
VERSION=1.1.0 AWS_REGION=eu-central-1 bash ~/app/deploy.sh
```
