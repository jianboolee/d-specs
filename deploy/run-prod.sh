#!/usr/bin/env bash
# d-specs 生产部署（docker compose v2 版）—— 在业务机（47.95.176.70 / 172.17.255.64）执行
#
# 用法：TAG=<镜像tag> bash /opt/d-specs/run-prod.sh    （不带 TAG 默认 latest）
# 前置：/opt/d-specs/docker-compose.yml 已下发（make deploy-only 会自动 scp）；
#       /opt/d-specs/.env.prod 已填好；机器已 docker login ACR。
# 幂等：已被 compose 管理的容器不重建（除非镜像/配置变了，由 compose 判断）。
set -euo pipefail
cd /opt/d-specs

ENV_FILE=/opt/d-specs/.env.prod
NAME=d-specs
export TAG=${TAG:-latest}
IMAGE="registry.cn-beijing.aliyuncs.com/xrv/d-specs:$TAG"

[ -f "$ENV_FILE" ] || { echo "错误：$ENV_FILE 不存在，请先填写生产变量（DATABASE_URL、DAUTH_BASE_URL、DAUTH_CLIENT_KEY、DAUTH_API_KEY）"; exit 1; }
[ -f docker-compose.yml ] || { echo "错误：/opt/d-specs/docker-compose.yml 缺失（仓库 deploy/ 下，make deploy-only 会下发）"; exit 1; }

# 一次性迁移：旧 docker run 起的无 compose 标签同名容器 → 让位给 compose
if docker inspect "$NAME" >/dev/null 2>&1; then
  if ! docker inspect "$NAME" --format '{{json .Config.Labels}}' 2>/dev/null | grep -q com.docker.compose; then
    echo "▶ 检测到非 compose 管理的旧容器 $NAME，移除后交接给 compose"
    docker rm -f "$NAME" >/dev/null
  fi
fi

echo "▶ 拉取 $IMAGE"
docker compose pull

echo "▶ compose up -d"
docker compose up -d

echo "▶ 等待健康检查（首启要跑迁移，最多约 60s）"
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:8400/health" >/dev/null 2>&1; then
    docker compose ps
    echo "✔ d-specs 就绪：172.17.255.64:8400（入口机 nginx 反代此内网地址）"
    exit 0
  fi
  sleep 2
done
echo "✘ 健康检查超时，最近日志："
docker compose logs --tail 40 app
exit 1