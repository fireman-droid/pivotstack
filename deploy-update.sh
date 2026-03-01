#!/bin/bash
# Kiro Stack 一键更新部署脚本
# 服务器: 115.191.35.73

set -e

PROJECT_DIR="/var/www/kiro-stack"
cd "$PROJECT_DIR"

# 加载 .env
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

echo "📦 拉取最新代码..."
git pull origin main

echo "🔧 停止服务..."
docker stop kiro-go kiro-gateway 2>/dev/null || true
docker rm kiro-go kiro-gateway 2>/dev/null || true

echo "🏗️  重新构建镜像..."
docker build -t kiro-gateway-local:latest ./kiro-gateway
docker build -t kiro-go:latest ./kiro-go

echo "🚀 启动 kiro-gateway..."
docker run -d \
  --name kiro-gateway \
  --network host \
  --restart unless-stopped \
  -e PROXY_API_KEY=${INTERNAL_API_KEY} \
  -e VPN_PROXY_URL=${VPN_PROXY_URL:-} \
  -e DEBUG_MODE=${DEBUG_MODE:-off} \
  -e SKIP_STARTUP_CREDENTIAL_CHECK=true \
  -e SERVER_PORT=8001 \
  -e SERVER_HOST=0.0.0.0 \
  kiro-gateway-local:latest

echo "⏳ 等待 kiro-gateway 启动..."
sleep 3

echo "🚀 启动 kiro-go..."
docker run -d \
  --name kiro-go \
  --network host \
  --restart unless-stopped \
  -v "$PROJECT_DIR/kiro-go/data:/app/data" \
  -e CONFIG_PATH=/app/data/config.json \
  -e ADMIN_PASSWORD=${ADMIN_PASSWORD} \
  -e KIRO_GATEWAY_BASE=http://127.0.0.1:8001 \
  -e KIRO_GATEWAY_API_KEY=${INTERNAL_API_KEY} \
  -e PORT=8088 \
  -e HOST=0.0.0.0 \
  kiro-go:latest

echo ""
echo "⏳ 等待服务启动..."
sleep 5

echo ""
echo "✅ 部署完成！服务状态："
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}" | grep kiro

echo ""
echo "📊 查看日志："
echo "  docker logs kiro-go -f"
echo "  docker logs kiro-gateway -f"
