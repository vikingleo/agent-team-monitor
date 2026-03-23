#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# service.sh — Linux 部署管理脚本
#
# 用法:
#   ./scripts/service.sh start
#   ./scripts/service.sh stop
#   ./scripts/service.sh restart
#   ./scripts/service.sh status
#   ./scripts/service.sh logs
#
# 可选环境变量:
#   ATM_MODE=web|tui           默认 web
#   ATM_PORT=8080              web 模式端口
#   ATM_PROVIDER=both          claude|codex|openclaw|both
#   ATM_APP_BIN=bin/agent-team-monitor
#   ATM_RUN_DIR=run
#   ATM_LOG_DIR=logs
# ============================================================

ACTION="${1:-status}"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_BIN="${ATM_APP_BIN:-bin/agent-team-monitor}"
APP_VERSION="${ATM_APP_VERSION:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
MODE="${ATM_MODE:-web}"
PORT="${ATM_PORT:-8080}"
PROVIDER="${ATM_PROVIDER:-both}"
RUN_DIR="${ATM_RUN_DIR:-run}"
LOG_DIR="${ATM_LOG_DIR:-logs}"
PID_FILE="${ROOT_DIR}/${RUN_DIR}/agent-team-monitor.pid"
STDOUT_LOG="${ROOT_DIR}/${LOG_DIR}/agent-team-monitor.out.log"
STDERR_LOG="${ROOT_DIR}/${LOG_DIR}/agent-team-monitor.err.log"

mkdir -p "${ROOT_DIR}/${RUN_DIR}" "${ROOT_DIR}/${LOG_DIR}" "${ROOT_DIR}/bin"

build_binary() {
  echo ">> 构建中..."
  (
    cd "${ROOT_DIR}"
    go build -ldflags "-X main.appVersion=${APP_VERSION}" -o "${APP_BIN}" ./cmd/monitor
  )
  echo ">> 构建完成: ${APP_BIN}"
}

is_running() {
  if [[ ! -f "${PID_FILE}" ]]; then
    return 1
  fi

  local pid
  pid="$(cat "${PID_FILE}" 2>/dev/null || true)"
  if [[ -z "${pid}" ]]; then
    return 1
  fi

  if kill -0 "${pid}" 2>/dev/null; then
    return 0
  fi

  rm -f "${PID_FILE}"
  return 1
}

print_status() {
  if is_running; then
    local pid
    pid="$(cat "${PID_FILE}")"
    echo ">> 运行中，PID=${pid}"
    if [[ "${MODE}" == "web" ]]; then
      echo ">> Web 地址: http://localhost:${PORT}"
    fi
  else
    echo ">> 未运行"
  fi
}

start_service() {
  if is_running; then
    print_status
    return 0
  fi

  build_binary

  local cmd
  if [[ "${MODE}" == "web" ]]; then
    cmd=( "./${APP_BIN}" -web -addr ":${PORT}" -provider "${PROVIDER}" )
  else
    cmd=( "./${APP_BIN}" -provider "${PROVIDER}" )
  fi

  echo ">> 启动模式: ${MODE}"
  echo ">> 数据源: ${PROVIDER}"
  [[ "${MODE}" == "web" ]] && echo ">> 端口: ${PORT}"
  echo ">> 日志输出: ${STDOUT_LOG}"
  echo ">> 错误日志: ${STDERR_LOG}"

  (
    cd "${ROOT_DIR}"
    nohup "${cmd[@]}" >>"${STDOUT_LOG}" 2>>"${STDERR_LOG}" &
    echo $! > "${PID_FILE}"
  )

  sleep 1
  if is_running; then
    echo ">> 启动成功"
    print_status
    return 0
  fi

  echo ">> 启动失败，请检查日志:"
  echo "   tail -n 100 ${STDERR_LOG}"
  return 1
}

stop_service() {
  if ! is_running; then
    echo ">> 服务未运行"
    rm -f "${PID_FILE}"
    return 0
  fi

  local pid
  pid="$(cat "${PID_FILE}")"
  echo ">> 停止服务，PID=${pid}"
  kill "${pid}" 2>/dev/null || true

  local retries=20
  while (( retries > 0 )); do
    if ! kill -0 "${pid}" 2>/dev/null; then
      rm -f "${PID_FILE}"
      echo ">> 已停止"
      return 0
    fi
    sleep 0.5
    retries=$((retries - 1))
  done

  echo ">> 进程未在预期时间内退出，发送 SIGKILL"
  kill -9 "${pid}" 2>/dev/null || true
  rm -f "${PID_FILE}"
  echo ">> 已强制停止"
}

show_logs() {
  touch "${STDOUT_LOG}" "${STDERR_LOG}"
  echo ">> 实时查看日志，按 Ctrl+C 退出"
  tail -n 100 -f "${STDOUT_LOG}" "${STDERR_LOG}"
}

case "${ACTION}" in
  start)
    start_service
    ;;
  stop)
    stop_service
    ;;
  restart)
    stop_service
    start_service
    ;;
  status)
    print_status
    ;;
  logs)
    show_logs
    ;;
  *)
    echo "用法: $0 {start|stop|restart|status|logs}"
    exit 1
    ;;
esac
