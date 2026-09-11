#!/usr/bin/env bash
# httpYac CLI 运行器
# 用法:
#   ./test/scripts/run.sh              # 运行全部
#   ./test/scripts/run.sh health       # 仅 00-health.http
#   ./test/scripts/run.sh user         # 匹配 *user*
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

ENV="${HTTP_ENV:-dev}"
PATTERN="${1:-*}"

if [[ -n "${HTTPYAC:-}" ]]; then
  :
elif [[ -x "$ROOT/node_modules/.bin/httpyac" ]]; then
  HTTPYAC="$ROOT/node_modules/.bin/httpyac"
elif command -v httpyac >/dev/null 2>&1; then
  HTTPYAC=httpyac
elif command -v npx >/dev/null 2>&1; then
  HTTPYAC="npx httpyac"
else
  echo "error: httpyac not found. Run: make http-install" >&2
  exit 1
fi

if [[ "$PATTERN" == "*" ]]; then
  exec $HTTPYAC send test/http --all -e "$ENV"
fi

exec $HTTPYAC send "test/http/*${PATTERN}*.http" --all -e "$ENV"
