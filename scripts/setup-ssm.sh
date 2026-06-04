#!/bin/bash
# setup-ssm.sh — Legt alle AVOC SSM Parameter unter /avoc/prod/ an.
# Einmalig vom Dev-Rechner ausführen. Voraussetzung: aws cli konfiguriert + IAM-Rechte.
#
# Verwendung:
#   AWS_REGION=eu-central-1 bash scripts/setup-ssm.sh
#
set -euo pipefail

REGION=${AWS_REGION:-eu-central-1}

put_secure() {
  aws ssm put-parameter \
    --region "$REGION" \
    --name "$1" \
    --value "$2" \
    --type SecureString \
    --overwrite \
    --query "Version" \
    --output text > /dev/null
  echo "  [SecureString] $1"
}

put_string() {
  aws ssm put-parameter \
    --region "$REGION" \
    --name "$1" \
    --value "$2" \
    --type String \
    --overwrite \
    --query "Version" \
    --output text > /dev/null
  echo "  [String]       $1"
}

echo "=== AVOC SSM Parameter Store Setup ==="
echo "Region: $REGION"
echo ""
echo "Bitte Werte eingeben (Enter = Standard übernehmen wo angegeben):"
echo ""

# JWT Secret
read -rp "JWT_SECRET (min. 32 Zeichen, kein Default): " jwt_secret
if [ ${#jwt_secret} -lt 32 ]; then
  echo "ERROR: JWT_SECRET muss mindestens 32 Zeichen lang sein." && exit 1
fi

# Elastic IP
read -rp "TURN_EXTERNAL_IP (Elastic IP der EC2-Instanz): " turn_external_ip
if [ -z "$turn_external_ip" ]; then
  echo "ERROR: TURN_EXTERNAL_IP darf nicht leer sein." && exit 1
fi

# TURN
read -rp "TURN_REALM [avoc.example.com]: " turn_realm
turn_realm=${turn_realm:-avoc.example.com}

read -rp "TURN_USER [avoc]: " turn_user
turn_user=${turn_user:-avoc}

read -rsp "TURN_PASSWORD (min. 16 Zeichen): " turn_password; echo ""
if [ ${#turn_password} -lt 16 ]; then
  echo "ERROR: TURN_PASSWORD muss mindestens 16 Zeichen lang sein." && exit 1
fi

# Grafana
read -rp "GRAFANA_ADMIN_USER [admin]: " grafana_user
grafana_user=${grafana_user:-admin}

read -rsp "GRAFANA_ADMIN_PASSWORD (min. 12 Zeichen): " grafana_password; echo ""
if [ ${#grafana_password} -lt 12 ]; then
  echo "ERROR: GRAFANA_ADMIN_PASSWORD muss mindestens 12 Zeichen lang sein." && exit 1
fi

# Docker Hub
read -rp "DOCKER_USERNAME (Docker Hub Benutzername): " docker_username
if [ -z "$docker_username" ]; then
  echo "ERROR: DOCKER_USERNAME darf nicht leer sein." && exit 1
fi

read -rsp "DOCKER_PASSWORD (Docker Hub Access Token): " docker_password; echo ""
if [ -z "$docker_password" ]; then
  echo "ERROR: DOCKER_PASSWORD darf nicht leer sein." && exit 1
fi

echo ""
echo "Schreibe Parameter nach /avoc/prod/ ..."
echo ""

put_secure  /avoc/prod/jwt-secret              "$jwt_secret"
put_string  /avoc/prod/turn-external-ip        "$turn_external_ip"
put_string  /avoc/prod/turn-realm              "$turn_realm"
put_string  /avoc/prod/turn-user               "$turn_user"
put_secure  /avoc/prod/turn-password           "$turn_password"
put_string  /avoc/prod/grafana-admin-user      "$grafana_user"
put_secure  /avoc/prod/grafana-admin-password  "$grafana_password"
put_string  /avoc/prod/docker-username         "$docker_username"
put_secure  /avoc/prod/docker-password         "$docker_password"

echo ""
echo "=== Alle Parameter angelegt. ==="
echo ""
echo "Nächster Schritt: deploy.sh auf die EC2-Instanz kopieren und ausführen:"
echo "  scp scripts/deploy.sh ec2-user@<IP>:~/app/"
echo "  bash ~/app/deploy.sh"
