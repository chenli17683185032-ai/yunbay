#!/usr/bin/env bash
set -euo pipefail

# New API VPS base preparation script.
# Run on a fresh Ubuntu/Debian VPS as root after SSH access is available.
# It does NOT deploy New API application code yet; it prepares the host for a later deployment.

DOMAIN="${DOMAIN:-yunbay.xyz}"
APP_USER="${APP_USER:-deploy}"
TZ_VALUE="${TZ_VALUE:-Asia/Shanghai}"
SSH_PORT="${SSH_PORT:-22}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root" >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y \
  ca-certificates \
  curl \
  gnupg \
  lsb-release \
  ufw \
  fail2ban \
  git \
  jq \
  vim \
  htop \
  unzip

# Timezone
if command -v timedatectl >/dev/null 2>&1; then
  timedatectl set-timezone "${TZ_VALUE}" || true
fi

# Docker official repository
install -m 0755 -d /etc/apt/keyrings
if [[ ! -f /etc/apt/keyrings/docker.asc ]]; then
  curl -fsSL https://download.docker.com/linux/$(. /etc/os-release && echo "$ID")/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
fi
. /etc/os-release
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${ID} \
  $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" \
  > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker

# Deployment user (kept passwordless-login disabled unless SSH key is added later)
if ! id "${APP_USER}" >/dev/null 2>&1; then
  adduser --disabled-password --gecos "" "${APP_USER}"
fi
usermod -aG docker "${APP_USER}"

# Directories reserved for later New API deployment.
install -d -m 0755 -o "${APP_USER}" -g "${APP_USER}" /opt/new-api
install -d -m 0750 -o "${APP_USER}" -g "${APP_USER}" /opt/new-api/{app,data,logs,postgres,redis,backups,secrets}

# Host firewall. Keep SSH open before enabling UFW.
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow "${SSH_PORT}"/tcp comment 'SSH'
ufw allow 80/tcp comment 'HTTP for reverse proxy / certificate issuance'
ufw allow 443/tcp comment 'HTTPS for reverse proxy'
ufw --force enable

# Basic fail2ban for sshd if present.
cat >/etc/fail2ban/jail.d/sshd.local <<JAIL
[sshd]
enabled = true
port = ${SSH_PORT}
maxretry = 5
findtime = 10m
bantime = 1h
JAIL
systemctl enable --now fail2ban || true
systemctl restart fail2ban || true

# Keep a machine-readable state file for future deploy steps.
cat >/opt/new-api/server-prep-state.json <<STATE
{
  "domain": "${DOMAIN}",
  "app_user": "${APP_USER}",
  "timezone": "${TZ_VALUE}",
  "prepared_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "docker": "$(docker --version 2>/dev/null || true)",
  "compose": "$(docker compose version 2>/dev/null || true)"
}
STATE
chown "${APP_USER}:${APP_USER}" /opt/new-api/server-prep-state.json
chmod 0640 /opt/new-api/server-prep-state.json

echo "Base preparation complete for ${DOMAIN}."
echo "Next step: deploy customized New API into /opt/new-api/app and attach reverse proxy."
