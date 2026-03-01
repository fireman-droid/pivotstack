#!/bin/bash
# Kiro Stack 一键更新部署脚本

set -e

PROJECT_DIR="/var/www/kiro-stack"
cd "$PROJECT_DIR"

echo "📦 拉取最新代码..."
git pull gitee main

echo "🔧 停止服务..."
docker compose down

echo "🏗️  重新构建镜像..."
docker compose build --no-cache

echo "🚀 启动服务..."
docker compose up -d

echo ""
echo "⏳ 等待服务启动..."
sleep 5

echo ""
echo "✅ 部署完成！服务状态："
docker compose ps

echo ""
echo "📊 查看日志："
echo "  docker compose logs -f"
