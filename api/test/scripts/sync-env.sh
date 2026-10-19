#!/usr/bin/env bash
# 直连 d-auth 登录取 access token，写入 http-client.private.env.json
#
# 本地 users 表已移除、本后端也不再代理登录：登录统一在 d-auth 完成。
# 后端仍接受 `Authorization: Bearer <d-auth access token>`（本地 JWKS 验签），
# 所以该脚本直接打 d-auth 的密码登录接口拿 token，供后续 .http 用例使用。
#
# 账号必须先在 d-auth 存在（管理员建号，或人工完成一次短信验证码登录）。
# 可选 DS_TOKEN 直接注入现成 token，跳过登录。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BASE_URL="${DS_BASE_URL:-http://localhost:8401}"
# d-auth 的 API origin（不带 /v1）。本地默认 8681，生产 https://auth.x-rv.com
DAUTH_BASE_URL="${DAUTH_BASE_URL:-${DS_DAUTH_BASE_URL:-http://localhost:8681}}"
# d-auth 的登录标识：手机号或邮箱
IDENTIFIER="${DS_TEST_IDENTIFIER:-${DS_TEST_EMAIL:-}}"
PASSWORD="${DS_TEST_PASSWORD:-}"

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required (brew install jq)" >&2
  exit 1
fi

# 后端探活只是提醒：token 由 d-auth 签发，与后端是否在线独立
if ! curl -sf -m 5 "$BASE_URL/health" >/dev/null 2>&1; then
  echo "warning: 后端未启动或不可达：$BASE_URL（后续用 token 调接口会失败，先执行 make dev）" >&2
fi

TOKEN="${DS_TOKEN:-}"
USER_ID=""

if [[ -n "$TOKEN" ]]; then
  echo ">> 使用注入的 DS_TOKEN"
  USER_ID="$(curl -s -m 10 "$BASE_URL/api/v1/me" -H "Authorization: Bearer $TOKEN" \
    | jq -r '.data.id // empty')"
else
  if [[ -z "$IDENTIFIER" || -z "$PASSWORD" ]]; then
    echo "error: 需要 DS_TEST_IDENTIFIER（手机号或邮箱）与 DS_TEST_PASSWORD，" >&2
    echo "       或 DS_TOKEN 直接注入会话 token。" >&2
    echo "       本地用户表已移除，测试账号必须先在 d-auth 存在。" >&2
    exit 1
  fi

  echo ">> 直连 d-auth 登录并获取 token：$IDENTIFIER（$DAUTH_BASE_URL）"
  RESP="$(curl -s -m 10 -X POST "$DAUTH_BASE_URL/v1/auth/password/login" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg i "$IDENTIFIER" --arg p "$PASSWORD" '{identifier:$i, password:$p}')")"

  TOKEN="$(echo "$RESP" | jq -r '.data.access_token // empty')"
  USER_ID="$(echo "$RESP" | jq -r '.data.user.id // empty')"

  if [[ -z "$TOKEN" ]]; then
    echo "$RESP" >&2
    echo "error: 登录失败。确认该账号已在 d-auth 注册、DAUTH_BASE_URL 指向正确的账户中心" >&2
    exit 1
  fi
fi

ENV_FILE="$ROOT/http-client.private.env.json"
jq -n \
  --arg baseUrl "$BASE_URL" \
  --arg dauthBaseUrl "$DAUTH_BASE_URL" \
  --arg token "$TOKEN" \
  --arg userId "$USER_ID" \
  --arg identifier "$IDENTIFIER" \
  --arg password "$PASSWORD" \
  '{dev: {baseUrl: $baseUrl, dauthBaseUrl: $dauthBaseUrl, token: $token, userId: $userId, identifier: $identifier, password: $password}}' >"$ENV_FILE"

echo ">> 已写入 $ENV_FILE"
echo "   userId:  $USER_ID"
echo "   baseUrl: $BASE_URL"
echo "   dauth:   $DAUTH_BASE_URL"