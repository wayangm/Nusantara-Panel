#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo ./scripts/install_ubuntu_22.sh /path/to/nusantarad"
  exit 1
fi

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 /path/to/nusantarad"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

BINARY_SRC="$1"

if [[ ! -f "${BINARY_SRC}" ]]; then
  echo "Binary not found: ${BINARY_SRC}"
  exit 1
fi

if [[ ! -f /etc/os-release ]]; then
  echo "Cannot detect OS (missing /etc/os-release)"
  exit 1
fi

source /etc/os-release
if [[ "${ID:-}" != "ubuntu" ]]; then
  echo "Unsupported OS: ${ID:-unknown}. Required: Ubuntu 22.04+"
  exit 1
fi

MAJOR_VERSION="${VERSION_ID%%.*}"
if [[ -z "${MAJOR_VERSION}" ]] || (( MAJOR_VERSION < 22 )); then
  echo "Unsupported Ubuntu version: ${VERSION_ID:-unknown}. Required: 22.04+"
  exit 1
fi

if ! id -u nusantara >/dev/null 2>&1; then
  useradd --system --home /var/lib/nusantara-panel --shell /usr/sbin/nologin nusantara
fi

echo "Installing base packages..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y \
  nginx \
  php-fpm \
  mariadb-server \
  redis-server \
  certbot \
  python3-certbot-nginx \
  curl \
  openssl

install -d -m 0750 -o nusantara -g nusantara /var/lib/nusantara-panel
install -d -m 0750 -o nusantara -g nusantara /var/log/nusantara-panel
install -d -m 0750 -o root -g root /var/backups/nusantara-panel
install -d -m 0755 /etc/nusantara-panel

install -m 0755 "${BINARY_SRC}" /usr/local/bin/nusantarad
install -m 0644 "${ROOT_DIR}/deploy/systemd/nusantara-panel.service" /etc/systemd/system/nusantara-panel.service

BOOTSTRAP_PASSWORD="$(openssl rand -base64 18 | tr -d '\n' | tr '/+' 'AB')"
if [[ ! -f /etc/nusantara-panel/nusantara-panel.env ]]; then
  install -m 0640 "${ROOT_DIR}/configs/nusantara-panel.env.example" /etc/nusantara-panel/nusantara-panel.env
fi

if grep -q '^NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=' /etc/nusantara-panel/nusantara-panel.env; then
  sed -i "s|^NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=.*|NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=${BOOTSTRAP_PASSWORD}|" /etc/nusantara-panel/nusantara-panel.env
else
  echo "NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=${BOOTSTRAP_PASSWORD}" >> /etc/nusantara-panel/nusantara-panel.env
fi

if grep -q '^NUSANTARA_PROVISION_APPLY=' /etc/nusantara-panel/nusantara-panel.env; then
  sed -i "s|^NUSANTARA_PROVISION_APPLY=.*|NUSANTARA_PROVISION_APPLY=true|" /etc/nusantara-panel/nusantara-panel.env
else
  echo "NUSANTARA_PROVISION_APPLY=true" >> /etc/nusantara-panel/nusantara-panel.env
fi

if grep -q '^NUSANTARA_BACKUP_DIR=' /etc/nusantara-panel/nusantara-panel.env; then
  sed -i "s|^NUSANTARA_BACKUP_DIR=.*|NUSANTARA_BACKUP_DIR=/var/backups/nusantara-panel|" /etc/nusantara-panel/nusantara-panel.env
else
  echo "NUSANTARA_BACKUP_DIR=/var/backups/nusantara-panel" >> /etc/nusantara-panel/nusantara-panel.env
fi

chown root:nusantara /etc/nusantara-panel/nusantara-panel.env
chmod 0640 /etc/nusantara-panel/nusantara-panel.env

systemctl enable nginx mariadb redis-server
systemctl restart nginx mariadb redis-server

systemctl daemon-reload
systemctl enable nusantara-panel
systemctl restart nusantara-panel

echo "Nusantara Panel installed and started."
echo "Check status: systemctl status nusantara-panel"
echo "Bootstrap admin username: admin"
echo "Bootstrap admin password: ${BOOTSTRAP_PASSWORD}"
