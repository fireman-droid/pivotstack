#!/bin/bash
# Kiro Stack 一键部署/更新脚本
# 用法:
#   bash deploy-update.sh          # 首次部署或全量更新
#   bash deploy-update.sh update   # 拉取最新代码并重建
#   bash deploy-update.sh restart  # 仅重启容器
#   bash deploy-update.sh logs     # 查看日志
#   bash deploy-update.sh status   # 查看运行状态

set -e
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

case "${1:-deploy}" in

deploy|update)
  echo "📦 拉取最新代码..."
  git pull origin main 2>/dev/null || true

  if [ ! -f .env ]; then
    echo "⚠️  未找到 .env 文件，从模板创建..."
    cp .env.example .env
    echo "❗ 请编辑 .env 填写实际配置后重新运行此脚本"
    exit 1
  fi

  echo "🔨 构建并启动服务..."
  docker compose up -d --build

  echo ""
  echo "✅ 部署完成！"
  echo "📊 管理面板: http://$(hostname -I | awk '{print $1}'):8088/admin"
  echo ""
  docker compose ps
  ;;

restart)
  echo "🔄 重启服务..."
  docker compose restart
  docker compose ps
  ;;

logs)
  docker compose logs -f --tail=50 ${2:-}
  ;;

status)
  docker compose ps
  echo ""
  echo "📊 最近日志:"
  docker compose logs --tail=10 kiro-go 2>&1 | grep -E "Refresh|Error|Request|Starting" | tail -10
  ;;

stop)
  echo "⏹️  停止服务..."
  docker compose down
  ;;

*)
  echo "用法: bash deploy-update.sh [deploy|update|restart|logs|status|stop]"
  exit 1
  ;;

esac
