#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# start.sh — 一键构建并运行 Agent Team Monitor
#
# 用法:
#   ./scripts/start.sh              # TUI 模式（默认）
#   ./scripts/start.sh desktop      # Linux 桌面应用
#   ./scripts/start.sh web          # Web 模式（默认端口 8080）
#   ./scripts/start.sh web 3000     # Web 模式，自定义端口
# ============================================================

CLI_APP="bin/agent-team-monitor"
DESKTOP_APP="bin/agent-team-monitor-desktop"
MODE="${1:-tui}"
PORT="${2:-8080}"
APP_VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

# 项目根目录
cd "$(dirname "$0")/.."

# 构建
echo ">> 构建中..."
go build -ldflags "-X main.appVersion=${APP_VERSION}" -o "$CLI_APP" ./cmd/monitor
if [ "$MODE" = "desktop" ]; then
  go build -ldflags "-X main.appVersion=${APP_VERSION}" -o "$DESKTOP_APP" ./cmd/desktop
fi
echo ">> 构建完成"

# 运行
case "$MODE" in
  desktop)
    echo ">> 启动 Linux 桌面应用"
    exec ./"$DESKTOP_APP"
    ;;
  web)
    echo ">> 启动 Web 模式 — http://localhost:${PORT}"
    exec ./"$CLI_APP" -web -addr ":${PORT}"
    ;;
  tui|*)
    echo ">> 启动 TUI 模式"
    exec ./"$CLI_APP"
    ;;
esac
