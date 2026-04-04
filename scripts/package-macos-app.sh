#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "package-macos-app.sh must be run on macOS"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
APP_NAME="Agent Team Monitor"
APP_DIR="${DIST_DIR}/${APP_NAME}.app"
VERSION="${1:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
VERSION="${VERSION#v}"
CONTENTS_DIR="${APP_DIR}/Contents"
MACOS_DIR="${CONTENTS_DIR}/MacOS"
RESOURCES_DIR="${CONTENTS_DIR}/Resources"
BIN_PATH="${MACOS_DIR}/agent-team-monitor-desktop"
INFO_TEMPLATE="${ROOT_DIR}/packaging/macos/Info.plist.template"
ICON_PATH="${ROOT_DIR}/assets/icons/agent-team-monitor.icns"

mkdir -p "${DIST_DIR}" "${MACOS_DIR}" "${RESOURCES_DIR}"
rm -rf "${APP_DIR}"
mkdir -p "${MACOS_DIR}" "${RESOURCES_DIR}"

if [[ ! -f "${ICON_PATH}" ]]; then
  echo "missing macOS icon: ${ICON_PATH}"
  echo "run: python3 ./scripts/generate-icons.py /path/to/AppIcon.iconset"
  exit 1
fi

(
  cd "${ROOT_DIR}"
  CGO_ENABLED=1 go build -ldflags "-X main.appVersion=${VERSION}" -o "${BIN_PATH}" ./cmd/desktop
)

install -m 0644 "${ICON_PATH}" "${RESOURCES_DIR}/agent-team-monitor.icns"
sed "s|__VERSION__|${VERSION}|g" "${INFO_TEMPLATE}" > "${CONTENTS_DIR}/Info.plist"
chmod 0755 "${BIN_PATH}"

ZIP_PATH="${DIST_DIR}/agent-team-monitor-macos-app-${VERSION}.zip"
rm -f "${ZIP_PATH}"
(
  cd "${DIST_DIR}"
  /usr/bin/zip -qry "${ZIP_PATH}" "${APP_NAME}.app"
)

echo "built macOS app bundle: ${APP_DIR}"
echo "packaged zip: ${ZIP_PATH}"
