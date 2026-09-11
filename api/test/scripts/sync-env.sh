#!/usr/bin/env bash
# 创建/登录测试账号，并把 token 写入 http-client.private.env.json
#
# 与模板时代的区别：不再依赖 cmd/devseed，直接打接口拿 token，
# 这样测试环境与真实链路一致。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BASE_URL="${DS_BASE_URL:-http://localhost:8300}"
EMAIL="${DS_TEST_EMAIL:-demo@dspecs.dev}"
PASSWORD="${DS_TEST_PASSWORD:-demo12345}"
NAME="${DS_TEST_NAME:-测试账号}"

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required (brew install jq)" >&2
  exit 1
fi

if ! curl -sf -m 5 "$BASE_URL/health" >/dev/null 2>&1; then
  echo "error: 后端未启动或不可达：$BASE_URL（先执行 make dev）" >&2
  exit 1
fi

echo ">> 确保测试账号存在：$EMAIL"
# 已存在会返回 409，忽略即可
curl -s -m 10 -o /dev/null -X POST "$BASE_URL/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg e "$EMAIL" --arg p "$PASSWORD" --arg n "$NAME" \
        '{email:$e, password:$p, name:$n}')" || true

echo ">> 登录并获取 token ..."
RESP="$(curl -s -m 10 -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg e "$EMAIL" --arg p "$PASSWORD" '{email:$e, password:$p}')")"

TOKEN="$(echo "$RESP" | jq -r '.data.token // empty')"
USER_ID="$(echo "$RESP" | jq -r '.data.user.id // empty')"

if [[ -z "$TOKEN" ]]; then
  echo "$RESP" >&2
  echo "error: 登录失败，无法获取 token" >&2
  exit 1
fi

ENV_FILE="$ROOT/http-client.private.env.json"
jq -n \
  --arg baseUrl "$BASE_URL" \
  --arg token "$TOKEN" \
  --arg userId "$USER_ID" \
  --arg email "$EMAIL" \
  '{dev: {baseUrl: $baseUrl, token: $token, userId: $userId, email: $email}}' >"$ENV_FILE"

echo ">> 已写入 $ENV_FILE"
echo "   userId:  $USER_ID"
echo "   baseUrl: $BASE_URL"
