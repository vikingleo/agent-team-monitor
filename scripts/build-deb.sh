#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
DIST_DIR="${ROOT_DIR}/dist"
PACKAGE_NAME="agent-team-monitor"
PACKAGE_VERSION="${1:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
PACKAGE_VERSION="${PACKAGE_VERSION#v}"
PACKAGE_VERSION="${PACKAGE_VERSION//-/.}"
ARCH="$(dpkg --print-architecture)"
BUILD_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/${PACKAGE_NAME}-deb-XXXXXX")"
PACKAGE_ROOT="${BUILD_ROOT}/${PACKAGE_NAME}_${PACKAGE_VERSION}_${ARCH}"

cleanup() {
  rm -rf "${BUILD_ROOT}"
}
trap cleanup EXIT

mkdir -p "${DIST_DIR}"

if [[ ! -x "${BIN_DIR}/agent-team-monitor" || ! -x "${BIN_DIR}/agent-team-monitor-desktop" ]]; then
  echo "missing binaries in ${BIN_DIR}"
  echo "run: make build-desktop"
  exit 1
fi

mkdir -p "${PACKAGE_ROOT}/DEBIAN"
mkdir -p "${PACKAGE_ROOT}/usr/bin"
mkdir -p "${PACKAGE_ROOT}/usr/share/applications"
mkdir -p "${PACKAGE_ROOT}/usr/share/icons/hicolor/32x32/apps"
mkdir -p "${PACKAGE_ROOT}/usr/share/icons/hicolor/64x64/apps"
mkdir -p "${PACKAGE_ROOT}/usr/share/icons/hicolor/128x128/apps"
mkdir -p "${PACKAGE_ROOT}/usr/share/icons/hicolor/256x256/apps"
mkdir -p "${PACKAGE_ROOT}/usr/share/icons/hicolor/512x512/apps"
mkdir -p "${PACKAGE_ROOT}/usr/share/doc/${PACKAGE_NAME}"
mkdir -p "${PACKAGE_ROOT}/usr/share/${PACKAGE_NAME}"

sed \
  -e "s|__VERSION__|${PACKAGE_VERSION}|g" \
  -e "s|__ARCH__|${ARCH}|g" \
  "${ROOT_DIR}/packaging/linux/debian/control" > "${PACKAGE_ROOT}/DEBIAN/control"

install -m 0755 "${ROOT_DIR}/packaging/linux/debian/postinst" "${PACKAGE_ROOT}/DEBIAN/postinst"
install -m 0755 "${ROOT_DIR}/packaging/linux/debian/prerm" "${PACKAGE_ROOT}/DEBIAN/prerm"
install -m 0755 "${ROOT_DIR}/packaging/linux/debian/postrm" "${PACKAGE_ROOT}/DEBIAN/postrm"

install -m 0755 "${BIN_DIR}/agent-team-monitor" "${PACKAGE_ROOT}/usr/bin/agent-team-monitor"
install -m 0755 "${BIN_DIR}/agent-team-monitor-desktop" "${PACKAGE_ROOT}/usr/bin/agent-team-monitor-desktop"

sed \
  -e 's|__APP_EXEC__|/usr/bin/agent-team-monitor-desktop|g' \
  "${ROOT_DIR}/packaging/linux/agent-team-monitor.desktop" > \
  "${PACKAGE_ROOT}/usr/share/applications/agent-team-monitor.desktop"

install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-32.png" \
  "${PACKAGE_ROOT}/usr/share/icons/hicolor/32x32/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-64.png" \
  "${PACKAGE_ROOT}/usr/share/icons/hicolor/64x64/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-128.png" \
  "${PACKAGE_ROOT}/usr/share/icons/hicolor/128x128/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-256.png" \
  "${PACKAGE_ROOT}/usr/share/icons/hicolor/256x256/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor.png" \
  "${PACKAGE_ROOT}/usr/share/icons/hicolor/512x512/apps/agent-team-monitor.png"

cp -R "${ROOT_DIR}/web" "${PACKAGE_ROOT}/usr/share/${PACKAGE_NAME}/"
cp -R "${ROOT_DIR}/assets" "${PACKAGE_ROOT}/usr/share/${PACKAGE_NAME}/"

sed \
  -e "s|__VERSION__|${PACKAGE_VERSION}|g" \
  -e "s|__DATE__|$(date -R)|g" \
  "${ROOT_DIR}/packaging/linux/changelog" > \
  "${PACKAGE_ROOT}/usr/share/doc/${PACKAGE_NAME}/changelog.Debian"
gzip -n -f "${PACKAGE_ROOT}/usr/share/doc/${PACKAGE_NAME}/changelog.Debian"

install -m 0644 "${ROOT_DIR}/packaging/linux/copyright" \
  "${PACKAGE_ROOT}/usr/share/doc/${PACKAGE_NAME}/copyright"

OUTPUT_PATH="${DIST_DIR}/${PACKAGE_NAME}_${PACKAGE_VERSION}_${ARCH}.deb"
dpkg-deb --build "${PACKAGE_ROOT}" "${OUTPUT_PATH}" >/dev/null

echo "built package: ${OUTPUT_PATH}"
