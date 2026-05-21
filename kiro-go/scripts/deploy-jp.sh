#!/bin/bash
# deploy-jp.sh — PivotStack JP server 部署 / 升级脚本
#
# 第一次部署：
#   1) ssh 上 JP server
#   2) sudo mkdir -p /opt/pivotstack && cd /opt/pivotstack
#   3) git clone https://github.com/fireman-droid/pivotstack.git .
#   4) cd kiro-go && cp .env.example .env && vim .env （填密码 + 加密 key）
#   5) 把宿主本地的 data/config.json, data/users.json, data/recharge_records.jsonl
#      用 scp 传到 /opt/pivotstack/kiro-go/data/
#   6) bash scripts/deploy-jp.sh
#
# 后续升级：
#   cd /opt/pivotstack && bash kiro-go/scripts/deploy-jp.sh
#
# 这个脚本干的事（按顺序）：
#   1) git pull --ff-only        拉最新代码
#   2) docker compose pull       不构建，拉远端镜像（如果 image: 模式）
#      或 docker compose build   构建本地代码（当前 compose 走 build:）
#   3) docker compose run --rm migrate    跑 PG schema migration
#   4) docker compose up -d                重启服务
#   5) 显示运行状态 + tail 30s 日志

set -euo pipefail

if [ ! -f kiro-go/docker-compose.yml ]; then
  echo "[deploy-jp] 在错误目录运行。请进入 /opt/pivotstack 或 PivotStack 项目根目录" >&2
  exit 1
fi

if [ ! -f kiro-go/.env ]; then
  echo "[deploy-jp] kiro-go/.env 不存在。先 cp .env.example .env 并填写" >&2
  exit 1
fi

ROOT=$(pwd)
COMPOSE_DIR="$ROOT/kiro-go"

cd "$ROOT"

echo "[deploy-jp] 1/5  git pull"
git pull --ff-only || {
  echo "[deploy-jp] git pull 失败（fast-forward 不行，可能 server 上有 local commit）"
  exit 1
}

cd "$COMPOSE_DIR"

echo "[deploy-jp] 2/5  docker compose build"
docker compose build --pull kiro-go

echo "[deploy-jp] 3/5  PG schema migration"
docker compose run --rm migrate || {
  rc=$?
  # kiro-migrate up exit 2 = already at latest，正常
  if [ "$rc" -ne 2 ] && [ "$rc" -ne 0 ]; then
    echo "[deploy-jp] migrate 失败 exit=$rc" >&2
    exit "$rc"
  fi
}

echo "[deploy-jp] 4/5  docker compose up -d"
docker compose up -d postgres kiro-go

echo "[deploy-jp] 5/5  状态确认"
sleep 3
docker compose ps
echo
echo "=== kiro-go 启动日志 ==="
docker compose logs --tail 30 kiro-go
echo
echo "[deploy-jp] OK. 业务端口默认 8080（容器内）→ host 8990。"
echo "[deploy-jp] 配合 nginx + Cloudflare 反代到 8990。"
