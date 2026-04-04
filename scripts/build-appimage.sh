#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
DIST_DIR="${ROOT_DIR}/dist"
PACKAGE_NAME="agent-team-monitor"
APPIMAGE_DISPLAY_NAME="Agent Team Monitor"
PACKAGE_VERSION="${1:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
PACKAGE_VERSION="${PACKAGE_VERSION#v}"
PACKAGE_VERSION="${PACKAGE_VERSION//\//-}"
APPIMAGE_TOOL="${APPIMAGE_TOOL:-$(command -v appimagetool || true)}"

map_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      echo "x86_64"
      ;;
    aarch64|arm64)
      echo "aarch64"
      ;;
    armv7l)
      echo "armhf"
      ;;
    *)
      uname -m
      ;;
  esac
}

ARCH="$(map_arch)"
BUILD_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/${PACKAGE_NAME}-appimage-XXXXXX")"
APPDIR="${BUILD_ROOT}/AppDir"
OUTPUT_PATH="${DIST_DIR}/${PACKAGE_NAME}-${PACKAGE_VERSION}-${ARCH}.AppImage"
DEFAULT_OUTPUT_NAME="${APPIMAGE_DISPLAY_NAME// /_}-${PACKAGE_VERSION}-${ARCH}.AppImage"

cleanup() {
  rm -rf "${BUILD_ROOT}"
}
trap cleanup EXIT

mkdir -p "${DIST_DIR}"

if [[ -z "${APPIMAGE_TOOL}" ]]; then
  echo "missing appimagetool"
  echo "install appimagetool or set APPIMAGE_TOOL=/path/to/appimagetool"
  exit 1
fi

if [[ ! -x "${BIN_DIR}/agent-team-monitor" || ! -x "${BIN_DIR}/agent-team-monitor-desktop" ]]; then
  echo "missing binaries in ${BIN_DIR}"
  echo "run: make build-desktop"
  exit 1
fi

mkdir -p "${APPDIR}/usr/bin"
mkdir -p "${APPDIR}/usr/share/applications"
mkdir -p "${APPDIR}/usr/share/icons/hicolor/32x32/apps"
mkdir -p "${APPDIR}/usr/share/icons/hicolor/64x64/apps"
mkdir -p "${APPDIR}/usr/share/icons/hicolor/128x128/apps"
mkdir -p "${APPDIR}/usr/share/icons/hicolor/256x256/apps"
mkdir -p "${APPDIR}/usr/share/icons/hicolor/512x512/apps"
mkdir -p "${APPDIR}/usr/share/${PACKAGE_NAME}"
chmod 0755 "${BUILD_ROOT}" "${APPDIR}"

install -m 0755 "${BIN_DIR}/agent-team-monitor" "${APPDIR}/usr/bin/agent-team-monitor"
install -m 0755 "${BIN_DIR}/agent-team-monitor-desktop" "${APPDIR}/usr/bin/agent-team-monitor-desktop"

sed \
  -e 's|__APP_EXEC__|agent-team-monitor-desktop|g' \
  "${ROOT_DIR}/packaging/linux/agent-team-monitor.desktop" > \
  "${APPDIR}/usr/share/applications/agent-team-monitor.desktop"

install -m 0644 "${APPDIR}/usr/share/applications/agent-team-monitor.desktop" \
  "${APPDIR}/agent-team-monitor.desktop"

install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-32.png" \
  "${APPDIR}/usr/share/icons/hicolor/32x32/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-64.png" \
  "${APPDIR}/usr/share/icons/hicolor/64x64/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-128.png" \
  "${APPDIR}/usr/share/icons/hicolor/128x128/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-256.png" \
  "${APPDIR}/usr/share/icons/hicolor/256x256/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor.png" \
  "${APPDIR}/usr/share/icons/hicolor/512x512/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor.png" \
  "${APPDIR}/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor.png" \
  "${APPDIR}/.DirIcon"

cp -R "${ROOT_DIR}/web" "${APPDIR}/usr/share/${PACKAGE_NAME}/"
cp -R "${ROOT_DIR}/assets" "${APPDIR}/usr/share/${PACKAGE_NAME}/"

cat > "${APPDIR}/AppRun" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

APPDIR="${APPDIR:-$(cd "$(dirname "$0")" && pwd)}"
export APPDIR
export PATH="${APPDIR}/usr/bin:${PATH}"
export XDG_DATA_DIRS="${APPDIR}/usr/share:${XDG_DATA_DIRS:-/usr/local/share:/usr/share}"

exec "${APPDIR}/usr/bin/agent-team-monitor-desktop" "$@"
EOF
chmod 0755 "${APPDIR}/AppRun"

if command -v desktop-file-validate >/dev/null 2>&1; then
  desktop-file-validate "${APPDIR}/agent-team-monitor.desktop" >/dev/null 2>&1 || true
fi

rm -f "${OUTPUT_PATH}" "${DIST_DIR}/${DEFAULT_OUTPUT_NAME}"
(
  cd "${DIST_DIR}"
  VERSION="${PACKAGE_VERSION}" ARCH="${ARCH}" "${APPIMAGE_TOOL}" "${APPDIR}"
)

if [[ ! -f "${DIST_DIR}/${DEFAULT_OUTPUT_NAME}" ]]; then
  echo "expected output not found: ${DIST_DIR}/${DEFAULT_OUTPUT_NAME}"
  exit 1
fi

if [[ "${DIST_DIR}/${DEFAULT_OUTPUT_NAME}" != "${OUTPUT_PATH}" ]]; then
  mv -f "${DIST_DIR}/${DEFAULT_OUTPUT_NAME}" "${OUTPUT_PATH}"
fi

chmod 0755 "${OUTPUT_PATH}"
echo "built package: ${OUTPUT_PATH}"
