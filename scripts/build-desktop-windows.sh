#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
VERSION="${1:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
VERSION="${VERSION#v}"
ARCH="${ARCH:-amd64}"

case "${ARCH}" in
  amd64|arm64)
    ;;
  *)
    echo "unsupported ARCH=${ARCH}; expected amd64 or arm64"
    exit 1
    ;;
esac

if [[ ! -f "${ROOT_DIR}/cmd/desktop/rsrc_windows_${ARCH}.syso" ]]; then
  echo "missing desktop Windows resource file for ${ARCH}"
  echo "run: ./scripts/generate-windows-resource.sh"
  exit 1
fi

mkdir -p "${BIN_DIR}"
(
  cd "${ROOT_DIR}"
  CGO_ENABLED=1 GOOS=windows GOARCH="${ARCH}" \
    go build -ldflags "-H=windowsgui -X main.appVersion=${VERSION}" \
    -o "${BIN_DIR}/agent-team-monitor-desktop-windows-${ARCH}.exe" ./cmd/desktop
)

echo "built Windows desktop executable: ${BIN_DIR}/agent-team-monitor-desktop-windows-${ARCH}.exe"
