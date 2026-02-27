#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${NUSANTARA_REPO_URL:-}"
BRANCH="${NUSANTARA_BRANCH:-main}"
WORKDIR="${NUSANTARA_WORKDIR:-/opt/nusantara-panel}"
GO_VERSION="${NUSANTARA_GO_VERSION:-1.26.0}"

usage() {
  cat <<'USAGE'
Usage:
  sudo bash install.sh [--repo <git-url>] [--branch <branch>] [--workdir <path>] [--go-version <version>]

Examples:
  sudo bash install.sh
  sudo bash install.sh --repo https://github.com/wayangm/Nusantara-Panel.git --branch main

Env alternatives:
  NUSANTARA_REPO_URL, NUSANTARA_BRANCH, NUSANTARA_WORKDIR, NUSANTARA_GO_VERSION
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO_URL="${2:-}"
      shift 2
      ;;
    --branch)
      BRANCH="${2:-}"
      shift 2
      ;;
    --workdir)
      WORKDIR="${2:-}"
      shift 2
      ;;
    --go-version)
      GO_VERSION="${2:-}"
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
  echo "Run as root: sudo bash install.sh ..."
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

echo "[1/5] Installing base build dependencies..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y curl ca-certificates git tar

install_go() {
  local arch
  case "$(uname -m)" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "Unsupported CPU architecture: $(uname -m)"
      exit 1
      ;;
  esac

  local go_tar="go${GO_VERSION}.linux-${arch}.tar.gz"
  local go_url="https://go.dev/dl/${go_tar}"

  echo "[2/5] Installing Go ${GO_VERSION}..."
  curl -fsSL "${go_url}" -o "/tmp/${go_tar}"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "/tmp/${go_tar}"
  rm -f "/tmp/${go_tar}"
  export PATH="/usr/local/go/bin:${PATH}"
  go version
}

install_go

resolve_source_dir() {
  local script_dir parent_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  parent_dir="$(cd "${script_dir}/.." && pwd)"

  if [[ -f "${script_dir}/go.mod" && -f "${script_dir}/cmd/nusantarad/main.go" ]]; then
    echo "${script_dir}"
    return
  fi
  if [[ -f "${parent_dir}/go.mod" && -f "${parent_dir}/cmd/nusantarad/main.go" ]]; then
    echo "${parent_dir}"
    return
  fi
  echo ""
}

SOURCE_DIR=""
if [[ -n "${REPO_URL}" ]]; then
  echo "[3/5] Pulling source from ${REPO_URL} (${BRANCH})..."
  mkdir -p "${WORKDIR}"
  if [[ -d "${WORKDIR}/.git" ]]; then
    git -C "${WORKDIR}" fetch --depth 1 origin "${BRANCH}"
    git -C "${WORKDIR}" checkout -f FETCH_HEAD
  else
    rm -rf "${WORKDIR}"
    git clone --depth 1 --branch "${BRANCH}" "${REPO_URL}" "${WORKDIR}"
  fi
  SOURCE_DIR="${WORKDIR}"
else
  SOURCE_DIR="$(resolve_source_dir)"
  if [[ -z "${SOURCE_DIR}" ]]; then
    echo "Source not found next to install.sh. Provide --repo <git-url>."
    exit 1
  fi
fi

echo "[4/5] Building nusantarad..."
cd "${SOURCE_DIR}"
mkdir -p bin
go build -o bin/nusantarad ./cmd/nusantarad

if [[ ! -x scripts/install_ubuntu_22.sh ]]; then
  chmod +x scripts/install_ubuntu_22.sh
fi

echo "[5/5] Installing Nusantara Panel service..."
./scripts/install_ubuntu_22.sh ./bin/nusantarad

echo "Nusantara Panel installation completed."
