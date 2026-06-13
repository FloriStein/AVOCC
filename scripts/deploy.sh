#!/bin/bash
# deploy.sh — AVOC Deployment auf EC2.
# Holt Secrets aus AWS SSM Parameter Store, loggt sich in Docker Hub ein,
# pulled alle Images und startet den Stack.
#
# Voraussetzung auf EC2:
#   - docker + docker compose plugin installiert
#   - aws cli installiert
#   - IAM Instance Profile mit SSM-Leseberechtigung auf /avoc/*
#   - docker-compose.prod.yml liegt in APP_DIR
#   - mediamtx/mediamtx.yml liegt in APP_DIR (aws s3 cp ... ~/app/mediamtx/mediamtx.yml)
#
# Verwendung:
#   AWS_REGION=eu-central-1 VERSION=latest bash ~/app/deploy.sh
#
set -euo pipefail

REGION=${AWS_REGION:-eu-central-1}
APP_DIR=${APP_DIR:-$(dirname "$(realpath "$0")")}
VERSION=${VERSION:-latest}

echo "=== AVOC Deploy === Region: $REGION  Version: $VERSION"
echo ""

# ─── Voraussetzungen prüfen ───────────────────────────────────────────────────

REQUIRED_FILES=(
  "$APP_DIR/docker-compose.prod.yml"
  "$APP_DIR/mediamtx/mediamtx.yml"
  "$APP_DIR/mosquitto/mosquitto.conf"
)
for f in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$f" ]; then
    echo "ERROR: Pflichtdatei fehlt: $f" && exit 1
  fi
done

# ─── SSM Parameter lesen ──────────────────────────────────────────────────────

get() {
  aws ssm get-parameter \
    --region "$REGION" \
    --name "$1" \
    --query Parameter.Value \
    --output text
}

get_secure() {
  aws ssm get-parameter \
    --region "$REGION" \
    --name "$1" \
    --with-decryption \
    --query Parameter.Value \
    --output text
}

echo "[1/4] Lade Secrets..."

# DB_PASSWORD + ADMIN_PASSWORD kommen aus $APP_DIR/.env (docker-compose liest sie automatisch).
# Alle anderen Secrets kommen aus AWS SSM Parameter Store.
ENV_FILE="$APP_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE fehlt — bitte DB_PASSWORD und ADMIN_PASSWORD dort eintragen." && exit 1
fi

export JWT_SECRET=$(get_secure          /avoc/prod/jwt-secret)
export WHIP_STREAM_KEY=$(get_secure     /avoc/prod/whip-stream-key)
export TURN_EXTERNAL_IP=$(get           /avoc/prod/turn-external-ip)
export TURN_REALM=$(get                 /avoc/prod/turn-realm)
export TURN_USER=$(get                  /avoc/prod/turn-user)
export TURN_PASSWORD=$(get_secure       /avoc/prod/turn-password)
export GRAFANA_ADMIN_USER=$(get         /avoc/prod/grafana-admin-user)
export GRAFANA_ADMIN_PASSWORD=$(get_secure /avoc/prod/grafana-admin-password)
DOCKER_USERNAME=$(get                   /avoc/prod/docker-username)
DOCKER_PASSWORD=$(get_secure            /avoc/prod/docker-password)

# EC2 Private IP aus Instance Metadata Service (IMDS v2) — für coturn relay-ip + external-ip=PUBLIC/PRIVATE.
# IMDSv2 erfordert Token-Header (Standard auf Amazon Linux 2023).
_IMDS_TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
export TURN_PRIVATE_IP=$(curl -sf -H "X-aws-ec2-metadata-token: ${_IMDS_TOKEN}" \
  http://169.254.169.254/latest/meta-data/local-ipv4)

export REGISTRY="docker.io/${DOCKER_USERNAME}"
export VERSION

echo "  JWT_SECRET          loaded (SSM)"
echo "  DB_PASSWORD         from .env"
echo "  ADMIN_PASSWORD      from .env"
echo "  WHIP_STREAM_KEY     loaded (SSM)"
echo "  TURN_EXTERNAL_IP    ${TURN_EXTERNAL_IP}"
echo "  TURN_PRIVATE_IP     ${TURN_PRIVATE_IP}"
echo "  TURN_REALM          ${TURN_REALM}"
echo "  REGISTRY            ${REGISTRY}"
echo "  VERSION             ${VERSION}"

# ─── Self-Signed SSL-Zertifikat (für getUserMedia auf HTTPS) ──────────────────
# Browser blockiert getUserMedia auf HTTP — HTTPS mit self-signed cert umgeht das.
# Einmalig generieren; bei erneutem Deploy wird es wiederverwendet.

SSL_DIR="$APP_DIR/ssl"
mkdir -p "$SSL_DIR"
if [ ! -f "$SSL_DIR/cert.pem" ]; then
  echo "Generiere Self-Signed SSL-Zertifikat für $TURN_EXTERNAL_IP..."
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$SSL_DIR/key.pem" \
    -out    "$SSL_DIR/cert.pem" \
    -days 365 -nodes \
    -subj "/CN=${TURN_EXTERNAL_IP}" \
    -addext "subjectAltName=IP:${TURN_EXTERNAL_IP}" 2>/dev/null
  echo "  Zertifikat erstellt: $SSL_DIR/cert.pem"
else
  echo "  SSL-Zertifikat vorhanden: $SSL_DIR/cert.pem"
fi

# ─── Docker Hub Login ─────────────────────────────────────────────────────────

echo ""
echo "[2/4] Docker Hub Login..."
echo "$DOCKER_PASSWORD" | docker login \
  --username "$DOCKER_USERNAME" \
  --password-stdin

# ─── Images pullen ────────────────────────────────────────────────────────────

echo ""
echo "[3/4] Pull Images from ${REGISTRY}..."
cd "$APP_DIR"
docker compose -f docker-compose.prod.yml pull

# ─── Stack starten ────────────────────────────────────────────────────────────

echo ""
echo "[4/4] Start Stack..."
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "=== Deploy abgeschlossen — $(date) ==="
echo ""
echo "Status:"
docker compose -f docker-compose.prod.yml ps
