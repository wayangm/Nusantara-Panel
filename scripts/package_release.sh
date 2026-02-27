#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_FILE="${ROOT_DIR}/VERSION"
DIST_DIR="${ROOT_DIR}/dist"

if [[ ! -f "${VERSION_FILE}" ]]; then
  echo "Missing VERSION file"
  exit 1
fi

VERSION="$(tr -d '[:space:]' < "${VERSION_FILE}")"
if [[ -z "${VERSION}" ]]; then
  echo "VERSION cannot be empty"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required to build release artifacts"
  exit 1
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

build_target() {
  local goos="$1"
  local goarch="$2"
  local outdir="${DIST_DIR}/nusantara-panel_${VERSION}_${goos}_${goarch}"
  local binary="${outdir}/nusantarad"

  mkdir -p "${outdir}"
  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o "${binary}" ./cmd/nusantarad

  cp "${ROOT_DIR}/configs/nusantara-panel.env.example" "${outdir}/nusantara-panel.env.example"
  cp "${ROOT_DIR}/deploy/systemd/nusantara-panel.service" "${outdir}/nusantara-panel.service"
  cp "${ROOT_DIR}/scripts/install_ubuntu_22.sh" "${outdir}/install_ubuntu_22.sh"
  cp "${ROOT_DIR}/README.md" "${outdir}/README.md"
  cp "${ROOT_DIR}/LICENSE" "${outdir}/LICENSE"

  chmod +x "${binary}" "${outdir}/install_ubuntu_22.sh"

  (
    cd "${DIST_DIR}"
    tar -czf "nusantara-panel_${VERSION}_${goos}_${goarch}.tar.gz" \
      "nusantara-panel_${VERSION}_${goos}_${goarch}"
  )
}

pushd "${ROOT_DIR}" >/dev/null
build_target "linux" "amd64"
build_target "linux" "arm64"
popd >/dev/null

(
  cd "${DIST_DIR}"
  cp "nusantara-panel_${VERSION}_linux_amd64.tar.gz" "nusantara-panel_linux_amd64.tar.gz"
  cp "nusantara-panel_${VERSION}_linux_arm64.tar.gz" "nusantara-panel_linux_arm64.tar.gz"
)

(
  cd "${DIST_DIR}"
  sha256sum nusantara-panel_*linux_*.tar.gz > checksums.txt
)

echo "Release artifacts generated in ${DIST_DIR}:"
ls -1 "${DIST_DIR}"
