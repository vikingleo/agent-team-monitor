#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
APP_BIN="${BIN_DIR}/agent-team-monitor-desktop"
CLI_BIN="${BIN_DIR}/agent-team-monitor"
DESKTOP_TEMPLATE="${ROOT_DIR}/packaging/linux/agent-team-monitor.desktop"
DESKTOP_TARGET_DIR="${HOME}/.local/share/applications"
ICON_TARGET_DIR="${HOME}/.local/share/icons/hicolor"
LOCAL_BIN_DIR="${HOME}/.local/bin"

if [[ ! -x "${APP_BIN}" ]]; then
  echo "missing desktop app binary: ${APP_BIN}"
  echo "run: make build-desktop"
  exit 1
fi

if [[ ! -x "${CLI_BIN}" ]]; then
  echo "missing cli binary: ${CLI_BIN}"
  echo "run: make build-desktop"
  exit 1
fi

mkdir -p "${DESKTOP_TARGET_DIR}"
mkdir -p "${ICON_TARGET_DIR}/128x128/apps"
mkdir -p "${ICON_TARGET_DIR}/256x256/apps"
mkdir -p "${ICON_TARGET_DIR}/512x512/apps"
mkdir -p "${LOCAL_BIN_DIR}"

install -m 0755 "${CLI_BIN}" "${LOCAL_BIN_DIR}/agent-team-monitor"
install -m 0755 "${APP_BIN}" "${LOCAL_BIN_DIR}/agent-team-monitor-desktop"

install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-128.png" \
  "${ICON_TARGET_DIR}/128x128/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor-256.png" \
  "${ICON_TARGET_DIR}/256x256/apps/agent-team-monitor.png"
install -m 0644 "${ROOT_DIR}/assets/icons/agent-team-monitor.png" \
  "${ICON_TARGET_DIR}/512x512/apps/agent-team-monitor.png"

desktop_file="${DESKTOP_TARGET_DIR}/agent-team-monitor.desktop"
sed "s|__APP_EXEC__|${LOCAL_BIN_DIR}/agent-team-monitor-desktop|g" "${DESKTOP_TEMPLATE}" > "${desktop_file}"
chmod 0644 "${desktop_file}"

if command -v desktop-file-validate >/dev/null 2>&1; then
  desktop-file-validate "${desktop_file}" >/dev/null 2>&1 || true
fi

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "${DESKTOP_TARGET_DIR}" >/dev/null 2>&1 || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q -t "${HOME}/.local/share/icons/hicolor" >/dev/null 2>&1 || true
fi

echo "installed desktop entry: ${desktop_file}"
echo "application icon name: agent-team-monitor"
