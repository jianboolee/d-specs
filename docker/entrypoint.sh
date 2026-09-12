#!/bin/sh
# 在单容器内同时运行 nginx 与 Go API。
# 任一进程意外退出都让容器一并结束，避免「nginx 还活着、后端已死」的假健康状态。
set -eu

MIGRATE_BIN=/app/migrate
SERVER_BIN=/app/server

# stop_requested 用于区分「收到停止信号」与「子进程意外挂掉」：
# 前者应优雅退出（exit 0），后者才是异常（exit 1）。
stop_requested=0

# 显式传命令时直接透传，便于跑一次性任务，例如：
#   docker run --rm -e DATABASE_URL=... <image> migrate up
#   docker run --rm -e DATABASE_URL=... <image> migrate down 1
case "${1:-serve}" in
  migrate)
    shift
    exec "$MIGRATE_BIN" "$@"
    ;;
  server)
    shift
    exec "$SERVER_BIN" "$@"
    ;;
  serve)
    ;;
  *)
    exec "$@"
    ;;
esac

if [ "${RUN_MIGRATIONS:-true}" = "true" ]; then
  echo "[entrypoint] applying database migrations..."
  "$MIGRATE_BIN" up
else
  echo "[entrypoint] skipping migrations (RUN_MIGRATIONS=${RUN_MIGRATIONS})"
fi

echo "[entrypoint] starting api server on :${HTTP_PORT:-8401}"
"$SERVER_BIN" &
server_pid=$!

echo "[entrypoint] starting nginx on :80"
nginx -g 'daemon off;' &
nginx_pid=$!

shutdown() {
  stop_requested=1
  kill -TERM "$server_pid" "$nginx_pid" 2>/dev/null || true
}

trap shutdown TERM INT

while :; do
  # 收到停止信号：排空子进程后正常退出
  if [ "$stop_requested" = "1" ]; then
    wait "$server_pid" 2>/dev/null || true
    wait "$nginx_pid" 2>/dev/null || true
    echo "[entrypoint] stopped"
    exit 0
  fi

  # 子进程意外退出：整体退出，让编排系统重启容器
  if ! kill -0 "$server_pid" 2>/dev/null; then
    echo "[entrypoint] api server exited unexpectedly, stopping container"
    shutdown
    wait "$server_pid" 2>/dev/null || true
    wait "$nginx_pid" 2>/dev/null || true
    exit 1
  fi
  if ! kill -0 "$nginx_pid" 2>/dev/null; then
    echo "[entrypoint] nginx exited unexpectedly, stopping container"
    shutdown
    wait "$server_pid" 2>/dev/null || true
    wait "$nginx_pid" 2>/dev/null || true
    exit 1
  fi

  sleep 1
done
