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

find_asset() {
  local path
  for path in "$@"; do
    if [[ -f "${path}" ]]; then
      echo "${path}"
      return 0
    fi
  done
  return 1
}

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

SERVICE_UNIT_SRC="$(find_asset \
  "${ROOT_DIR}/deploy/systemd/nusantara-panel.service" \
  "${SCRIPT_DIR}/deploy/systemd/nusantara-panel.service" \
  "${SCRIPT_DIR}/nusantara-panel.service" || true)"
if [[ -z "${SERVICE_UNIT_SRC:-}" ]]; then
  echo "Cannot find nusantara-panel.service (expected in deploy/systemd or script directory)"
  exit 1
fi

ENV_EXAMPLE_SRC="$(find_asset \
  "${ROOT_DIR}/configs/nusantara-panel.env.example" \
  "${SCRIPT_DIR}/configs/nusantara-panel.env.example" \
  "${SCRIPT_DIR}/nusantara-panel.env.example" || true)"
if [[ -z "${ENV_EXAMPLE_SRC:-}" ]]; then
  echo "Cannot find nusantara-panel.env.example (expected in configs or script directory)"
  exit 1
fi

install -m 0755 "${BINARY_SRC}" /usr/local/bin/nusantarad
install -m 0644 "${SERVICE_UNIT_SRC}" /etc/systemd/system/nusantara-panel.service

ENV_FILE="/etc/nusantara-panel/nusantara-panel.env"
CREATED_ENV_FILE="false"
if [[ ! -f "${ENV_FILE}" ]]; then
  install -m 0640 "${ENV_EXAMPLE_SRC}" "${ENV_FILE}"
  CREATED_ENV_FILE="true"
fi

BOOTSTRAP_PASSWORD=""
CURRENT_BOOTSTRAP_PASSWORD="$(sed -n 's/^NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=//p' "${ENV_FILE}" | head -n1)"
if [[ "${CREATED_ENV_FILE}" == "true" ]] || [[ -z "${CURRENT_BOOTSTRAP_PASSWORD}" ]] || [[ "${CURRENT_BOOTSTRAP_PASSWORD}" == "CHANGE_ME_STRONG_PASSWORD" ]]; then
  BOOTSTRAP_PASSWORD="$(openssl rand -base64 18 | tr -d '\n' | tr '/+' 'AB')"
  if grep -q '^NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=' "${ENV_FILE}"; then
    sed -i "s|^NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=.*|NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=${BOOTSTRAP_PASSWORD}|" "${ENV_FILE}"
  else
    echo "NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=${BOOTSTRAP_PASSWORD}" >> "${ENV_FILE}"
  fi
fi

if grep -q '^NUSANTARA_PROVISION_APPLY=' "${ENV_FILE}"; then
  sed -i "s|^NUSANTARA_PROVISION_APPLY=.*|NUSANTARA_PROVISION_APPLY=true|" "${ENV_FILE}"
else
  echo "NUSANTARA_PROVISION_APPLY=true" >> "${ENV_FILE}"
fi

if grep -q '^NUSANTARA_BACKUP_DIR=' "${ENV_FILE}"; then
  sed -i "s|^NUSANTARA_BACKUP_DIR=.*|NUSANTARA_BACKUP_DIR=/var/backups/nusantara-panel|" "${ENV_FILE}"
else
  echo "NUSANTARA_BACKUP_DIR=/var/backups/nusantara-panel" >> "${ENV_FILE}"
fi

chown root:nusantara "${ENV_FILE}"
chmod 0640 "${ENV_FILE}"

systemctl enable nginx mariadb redis-server
systemctl restart nginx mariadb redis-server

systemctl daemon-reload
systemctl enable nusantara-panel
systemctl restart nusantara-panel

echo "Nusantara Panel installed and started."
echo "Check status: systemctl status nusantara-panel"
echo "Bootstrap admin username: admin"
if [[ -n "${BOOTSTRAP_PASSWORD}" ]]; then
  echo "Bootstrap admin password: ${BOOTSTRAP_PASSWORD}"
else
  echo "Bootstrap admin password: unchanged (existing state user remains active)"
fi
