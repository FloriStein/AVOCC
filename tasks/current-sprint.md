# Sprint 8 — EC2 Deployment via Docker Hub

Ziel: AVOC vollständig auf AWS EC2 deploybar — keine Quellcode-Kopie auf der Instanz, alle Images aus Docker Hub (linux/amd64), Secrets ausschließlich aus AWS SSM Parameter Store.

Datum: 2026-06-04
Vorgänger: Sprint 7 ✅ (Logging & Audit Trail — slog, SQLite Audit Store, Loki/Grafana)

---

## Ausgangslage (aus Sprint 7)

| Was existiert | Stand |
|---------------|-------|
| `docker-compose.yml` | Entwicklungs-Compose — baut alle Services aus Quellcode |
| `.env` / `.env.example` | Lokale Secrets-Datei — auf EC2 nicht zulässig |
| `infrastructure/coturn/turnserver.conf` | Fehlt `external-ip` — für EC2 hinter NAT nötig |
| `grafana` in Compose | `GF_AUTH_ANONYMOUS_ENABLED: true` + Admin-Rolle — auf EC2 Sicherheitsproblem |
| CDK Stack (`cdk_server-stack.ts`) | Bereits aktualisiert: Port 3000, 8080, 1883, 3479, 10000–10050/udp, SSM IAM Policy |
| ADR-019 | Deployment-Strategie entschieden ✅ |

---

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| DEPLOY-01 | ADR-019 — Deployment-Strategie dokumentieren | L | ✅ Done | — |
| DEPLOY-02 | Makefile `build-prod` + `push` — linux/amd64, Docker Hub Tags | M | ✅ Done | DEPLOY-01 |
| DEPLOY-03 | `infrastructure/compose/docker-compose.prod.yml` — `image:` statt `build:` | M | ✅ Done | DEPLOY-02 |
| DEPLOY-04 | `scripts/setup-ssm.sh` + `scripts/deploy.sh` — SSM-Integration | M | ✅ Done | DEPLOY-01 |
| DEPLOY-05 | coturn EC2-Konfiguration — `external-ip` via `TURN_EXTERNAL_IP` | M | ✅ Done | DEPLOY-01 |
| DEPLOY-06 | Grafana Security — Login-Form + Admin-Credentials aus SSM | S | ✅ Done | DEPLOY-03 |
| DEPLOY-07 | EC2 Bootstrap Guide — Checkliste für ersten Deploy ab null | M | ✅ Done | DEPLOY-03, DEPLOY-04, DEPLOY-05 |

---

## Abhängigkeitspfad

```
DEPLOY-01 (✅) → DEPLOY-02 → DEPLOY-03 ──────────────────────────┐
               ↘ DEPLOY-04 ────────────────────────────────────────▶ DEPLOY-07
               ↘ DEPLOY-05 ────────────────────────────────────────▶
               ↘ DEPLOY-06 (in DEPLOY-03 eingebettet) ────────────▶
```

---

## Implementierungsdetails je Task

### DEPLOY-02 — Makefile `build-prod` + `push`

**Neue Targets in `Makefile`:**

```makefile
# Docker Hub Registry — überschreibbar: DOCKER_USERNAME=foo make push
DOCKER_USERNAME ?= $(shell cat .docker-username 2>/dev/null || echo "DOCKER_USERNAME_NOT_SET")
REGISTRY        := docker.io/$(DOCKER_USERNAME)
VERSION         ?= latest
PLATFORM        := linux/amd64
GO_SERVICES     := control-server auth-service safety-service telemetry-service webrtc-sfu

build-prod:
	@echo "Building all images for $(PLATFORM)..."
	@for svc in $(GO_SERVICES); do \
		echo "  → avoc-$$svc"; \
		docker buildx build --platform $(PLATFORM) \
			--build-arg SERVICE_NAME=$$svc \
			-t $(REGISTRY)/avoc-$$svc:$(VERSION) \
			-f infrastructure/docker/go-service.Dockerfile . --load; \
	done
	@echo "  → avoc-frontend"
	docker buildx build --platform $(PLATFORM) \
		-t $(REGISTRY)/avoc-frontend:$(VERSION) \
		-f infrastructure/docker/frontend.Dockerfile . --load

push: build-prod
	@echo "Pushing all images to $(REGISTRY)..."
	@for svc in $(GO_SERVICES); do docker push $(REGISTRY)/avoc-$$svc:$(VERSION); done
	docker push $(REGISTRY)/avoc-frontend:$(VERSION)
	@echo "Push complete. Deploy on EC2: bash scripts/deploy.sh"
```

**Image-Naming:**
```
docker.io/<USERNAME>/avoc-control-server:latest
docker.io/<USERNAME>/avoc-auth-service:latest
docker.io/<USERNAME>/avoc-safety-service:latest
docker.io/<USERNAME>/avoc-telemetry-service:latest
docker.io/<USERNAME>/avoc-webrtc-sfu:latest
docker.io/<USERNAME>/avoc-frontend:latest
```

---

### DEPLOY-03 — `docker-compose.prod.yml`

Alle Custom-Services: `image: ${REGISTRY}/avoc-<service>:${VERSION}` statt `build:`.

Drittanbieter-Images (mosquitto, loki, promtail, grafana, coturn) bleiben unverändert.

Env-Variablen werden nicht aus `.env` geladen — sondern vom aufrufenden `deploy.sh` als
Shell-Exports bereitgestellt.

**Abweichungen von `docker-compose.yml`:**
- Kein `build:` in irgendeinem Service
- coturn: `command`-Override mit `--external-ip=${TURN_EXTERNAL_IP}`
- Grafana: kein Anonymous-Admin (DEPLOY-06)
- Volumes identisch (audit-data, loki-data, grafana-data)

---

### DEPLOY-04 — SSM-Scripts

**`scripts/setup-ssm.sh`** — einmalig vom Dev-Rechner:
```bash
#!/bin/bash
# Legt alle AVOC SSM-Parameter unter /avoc/prod/ an.
# Voraussetzung: aws cli konfiguriert, ausreichende IAM-Rechte.
REGION=${AWS_REGION:-eu-central-1}

put_secure() { aws ssm put-parameter --region "$REGION" --name "$1" --value "$2" \
  --type SecureString --overwrite; }
put_string() { aws ssm put-parameter --region "$REGION" --name "$1" --value "$2" \
  --type String --overwrite; }

put_secure  /avoc/prod/jwt-secret          "<STARKES_SECRET_MIN_32_ZEICHEN>"
put_string  /avoc/prod/turn-external-ip    "<ELASTIC_IP>"
put_string  /avoc/prod/turn-realm          "avoc.example.com"
put_string  /avoc/prod/turn-user           "avoc"
put_secure  /avoc/prod/turn-password       "<TURN_PASSWORT>"
put_string  /avoc/prod/grafana-admin-user  "admin"
put_secure  /avoc/prod/grafana-admin-password "<GRAFANA_PASSWORT>"
put_string  /avoc/prod/docker-username     "<DOCKER_HUB_USERNAME>"
put_secure  /avoc/prod/docker-password     "<DOCKER_HUB_ACCESS_TOKEN>"
```

**`scripts/deploy.sh`** — auf EC2 ausführen:
```bash
#!/bin/bash
set -euo pipefail
REGION=${AWS_REGION:-eu-central-1}
APP_DIR=/home/ec2-user/app

get()        { aws ssm get-parameter --region "$REGION" --name "$1" \
               --query Parameter.Value --output text; }
get_secure() { aws ssm get-parameter --region "$REGION" --name "$1" \
               --with-decryption --query Parameter.Value --output text; }

export JWT_SECRET=$(get_secure /avoc/prod/jwt-secret)
export TURN_EXTERNAL_IP=$(get      /avoc/prod/turn-external-ip)
export TURN_REALM=$(get            /avoc/prod/turn-realm)
export TURN_USER=$(get             /avoc/prod/turn-user)
export TURN_PASSWORD=$(get_secure  /avoc/prod/turn-password)
export GRAFANA_ADMIN_USER=$(get    /avoc/prod/grafana-admin-user)
export GRAFANA_ADMIN_PASSWORD=$(get_secure /avoc/prod/grafana-admin-password)
DOCKER_USERNAME=$(get              /avoc/prod/docker-username)
DOCKER_PASSWORD=$(get_secure       /avoc/prod/docker-password)
export REGISTRY="docker.io/${DOCKER_USERNAME}"
export VERSION=${VERSION:-latest}

echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin

cd "$APP_DIR"
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
echo "Deploy complete — $(date)"
```

---

### DEPLOY-05 — coturn EC2-Konfiguration

**Problem:** `turnserver.conf` hat kein `external-ip` — ohne diesen Parameter weiß coturn
nicht, welche öffentliche IP es gegenüber Clients ankündigen soll. TURN relay funktioniert
hinter NAT nicht korrekt.

**Lösung:** In `docker-compose.prod.yml` den coturn-Command überschreiben:
```yaml
stun-turn:
  image: coturn/coturn:latest
  command: >
    --external-ip=${TURN_EXTERNAL_IP}
    --realm=${TURN_REALM}
    --user=${TURN_USER}:${TURN_PASSWORD}
    --fingerprint
    --lt-cred-mech
    --no-cli
    --log-file=stdout
    --min-port=49160
    --max-port=49200
  ports:
    - "3479:3478/udp"
    - "3479:3478/tcp"
    - "49160-49200:49160-49200/udp"
```

Der `command`-Override ersetzt die Config-Datei vollständig — alle nötigen Flags werden
inline gesetzt. Keine Dependency auf `turnserver.conf` für EC2-Betrieb.

---

### DEPLOY-06 — Grafana Security

In `docker-compose.prod.yml`:
```yaml
grafana:
  environment:
    GF_SECURITY_ADMIN_USER: ${GRAFANA_ADMIN_USER}
    GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_ADMIN_PASSWORD}
    GF_AUTH_ANONYMOUS_ENABLED: "false"
    GF_AUTH_DISABLE_LOGIN_FORM: "false"
    # GF_AUTH_ANONYMOUS_ORG_ROLE entfernt
```

---

### DEPLOY-07 — EC2 Bootstrap Guide

Neue Datei: `docs/deployment/ec2-bootstrap.md`

Inhalt: IAM Instance Profile (SSM Read Policy bereits im CDK), Security Group Ports
(bereits im CDK), Docker Compose Plugin installieren (manuell, da UserData nicht erneut
läuft), `setup-ssm.sh` ausführen, deploy.sh auf EC2 kopieren, erster Deploy.

---

## Sprint-Ziel / Definition of Done

- [x] ADR-019 dokumentiert (`docs/adr/019-deployment-strategy.md`)
- [x] `make push` baut alle 6 Images für `linux/amd64` und pushed nach Docker Hub
- [x] `docker-compose.prod.yml` — `image:` statt `build:`, YAML validiert
- [x] `scripts/deploy.sh` holt Secrets aus SSM — kein `.env` auf der Instanz
- [x] coturn `--external-ip=${TURN_EXTERNAL_IP}` als Command-Flag, Port-Range 49160-49200
- [x] Grafana Login-Formular aktiv, `GF_AUTH_ANONYMOUS_ENABLED: false`
- [x] EC2 Bootstrap Guide vollständig (`docs/deployment/ec2-bootstrap.md`)
- [x] Safety Regression: 19/19 ✅ (kein Go-Code geändert in Sprint 8)
