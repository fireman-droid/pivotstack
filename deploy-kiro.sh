#!/bin/bash
# PivotStack 服务器部署/更新脚本
# 用法: bash deploy-kiro.sh [deploy|update|stop|status|logs]

set -e
PROJECT_DIR="/var/www/kiro-stack"

deploy() {
  echo "🚀 首次部署 PivotStack..."

  # 克隆项目
  if [ ! -d "$PROJECT_DIR" ]; then
    git clone https://18825133336:Lin20050201@gitee.com/ji-bo-chang-oli-gave-it-to/kirofandai.git "$PROJECT_DIR"
  fi
  cd "$PROJECT_DIR"

  # 创建 .env（不在 Git 里）
  cat > .env << 'EOF'
# ============================================
# PivotStack 配置文件
# ============================================

# -------------------- 管理面板 --------------------
ADMIN_PASSWORD=Lin20050201

# -------------------- 内部通信 --------------------
INTERNAL_API_KEY=kiro-internal-key-2024

# -------------------- 代理配置 --------------------
# 服务器上 clash-meta 运行在 host 网络，容器通过 host.docker.internal 访问
VPN_PROXY_URL=http://host.docker.internal:7890

# 调试模式
DEBUG_MODE=off
EOF

  echo "📦 构建并启动..."
  docker compose up -d --build

  echo ""
  echo "✅ 部署完成!"
  status
}

update() {
  echo "🔄 更新 PivotStack..."
  cd "$PROJECT_DIR"

  echo "📥 拉取最新代码..."
  git pull origin main

  echo "📦 重建并重启..."
  docker compose up -d --build

  echo ""
  echo "✅ 更新完成!"
  status
}

stop() {
  echo "⏹ 停止 PivotStack..."
  cd "$PROJECT_DIR"
  docker compose down
  echo "✅ 已停止"
}

status() {
  echo ""
  echo "📊 服务状态:"
  docker ps --filter "name=kiro" --filter "name=clash" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
}

logs() {
  cd "$PROJECT_DIR"
  docker compose logs -f --tail=50
}

TARGET=${1:-status}

case $TARGET in
  deploy) deploy ;;
  update) update ;;
  stop)   stop ;;
  status) status ;;
  logs)   logs ;;
  *)      echo "用法: bash deploy-kiro.sh [deploy|update|stop|status|logs]"; exit 1 ;;
esac
