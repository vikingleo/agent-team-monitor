#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ICON_PATH="${ROOT_DIR}/assets/icons/agent-team-monitor.ico"
RSRC_VERSION="${RSRC_VERSION:-v0.10.2}"

if [[ ! -f "${ICON_PATH}" ]]; then
  echo "missing Windows icon: ${ICON_PATH}"
  echo "run: python3 ./scripts/generate-icons.py /path/to/AppIcon.iconset"
  exit 1
fi

for target_dir in "${ROOT_DIR}/cmd/monitor" "${ROOT_DIR}/cmd/desktop"; do
  for arch in amd64 arm64; do
    output="${target_dir}/rsrc_windows_${arch}.syso"
    rm -f "${output}"
    go run "github.com/akavel/rsrc@${RSRC_VERSION}" \
      -arch "${arch}" \
      -ico "${ICON_PATH}" \
      -o "${output}"
    echo "generated ${output}"
  done
done
