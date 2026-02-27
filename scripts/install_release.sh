#!/usr/bin/env bash
set -euo pipefail

REPO="${NUSANTARA_GH_REPO:-wayangm/Nusantara-Panel}"
TAG="${NUSANTARA_RELEASE_TAG:-latest}"

usage() {
  cat <<'USAGE'
Usage:
  sudo bash install_release.sh [--repo <owner/repo>] [--tag <release-tag|latest>]

Examples:
  sudo bash install_release.sh
  sudo bash install_release.sh --repo wayangm/Nusantara-Panel --tag v0.1.0

Environment alternatives:
  NUSANTARA_GH_REPO (default: wayangm/Nusantara-Panel), NUSANTARA_RELEASE_TAG
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --tag)
      TAG="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash install_release.sh ..."
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

ARCH=""
case "$(uname -m)" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported CPU architecture: $(uname -m)"
    exit 1
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get install -y curl ca-certificates tar
fi

ASSET="nusantara-panel_linux_${ARCH}.tar.gz"
if [[ "${TAG}" == "latest" ]]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "Downloading ${URL}"
curl -fsSL "${URL}" -o "${TMP_DIR}/${ASSET}"

tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}"

PAYLOAD_DIR="$(find "${TMP_DIR}" -maxdepth 1 -mindepth 1 -type d -name 'nusantara-panel_*_linux_*' | head -n 1)"
if [[ -z "${PAYLOAD_DIR}" ]]; then
  echo "Failed to locate extracted release payload"
  exit 1
fi

if [[ ! -x "${PAYLOAD_DIR}/install_ubuntu_22.sh" ]] || [[ ! -x "${PAYLOAD_DIR}/nusantarad" ]]; then
  echo "Invalid release payload: installer or binary not found"
  exit 1
fi

echo "Installing Nusantara Panel from release..."
"${PAYLOAD_DIR}/install_ubuntu_22.sh" "${PAYLOAD_DIR}/nusantarad"

echo "Done."
