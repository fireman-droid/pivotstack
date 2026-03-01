#!/bin/bash
# Kiro Stack 服务器首次部署脚本
# 在服务器上执行: bash deploy-server.sh

set -e

echo "=========================================="
echo "  Kiro Stack 服务器部署"
echo "=========================================="
echo ""

# 检查是否在服务器上
if [ ! -d "/var/www" ]; then
    echo "❌ 错误：请在服务器上执行此脚本"
    exit 1
fi

# 步骤 1: 检查环境
echo "📋 步骤 1/7: 检查环境"
echo "Docker 版本:"
docker --version
echo ""

echo "当前 /var/www 目录:"
ls -la /var/www/ | head -10
echo ""

echo "运行中的容器:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | head -10
echo ""

echo "检查端口占用:"
if ss -tlnp | grep -E ':(8088|8001)\s'; then
    echo "⚠️  警告：端口 8088 或 8001 已被占用"
    read -p "是否继续？(y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo "✅ 端口 8088 和 8001 可用"
fi
echo ""

# 步骤 2: 克隆项目
echo "📦 步骤 2/7: 克隆项目"
cd /var/www

if [ -d "kiro-stack" ]; then
    echo "⚠️  目录 kiro-stack 已存在"
    read -p "是否删除并重新克隆？(y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf kiro-stack
    else
        cd kiro-stack
        git pull origin main
        echo "✅ 已更新代码"
    fi
fi

if [ ! -d "kiro-stack" ]; then
    echo "正在克隆项目..."
    git clone https://18825133336:Lin20050201@gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git
    cd kiro-stack
    git config credential.helper store
    echo "✅ 项目克隆完成"
else
    cd kiro-stack
fi
echo ""

# 步骤 3: 配置环境变量
echo "🔧 步骤 3/7: 配置环境变量"
if [ -f .env ]; then
    echo "⚠️  .env 文件已存在"
    read -p "是否重新配置？(y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "✅ 使用现有配置"
    else
        rm .env
    fi
fi

if [ ! -f .env ]; then
    echo "生成随机密钥..."
    INTERNAL_API_KEY=$(openssl rand -hex 32)

    cat > .env << EOF
# 管理面板密码（必填）
ADMIN_PASSWORD=Lin20050201_kiro_admin

# 内部通信密钥（必填）
INTERNAL_API_KEY=${INTERNAL_API_KEY}

# 调试模式
DEBUG_MODE=off
EOF

    echo "✅ .env 文件已创建"
    echo "   ADMIN_PASSWORD: Lin20050201_kiro_admin"
    echo "   INTERNAL_API_KEY: ${INTERNAL_API_KEY}"
else
    echo "✅ 使用现有 .env 配置"
fi
echo ""

# 步骤 4: 开放防火墙端口
echo "🔓 步骤 4/7: 开放防火墙端口"
if command -v ufw &> /dev/null; then
    ufw allow 8088/tcp
    echo "✅ 已开放端口 8088"
else
    echo "⚠️  未检测到 ufw，请手动开放端口 8088"
fi
echo ""

# 步骤 5: 构建镜像
echo "🏗️  步骤 5/7: 构建 Docker 镜像（需要 3-5 分钟）"
export $(grep -v '^#' .env | xargs)
docker compose build
echo "✅ 镜像构建完成"
echo ""

# 步骤 6: 启动服务
echo "🚀 步骤 6/7: 启动服务"
docker compose up -d
echo "✅ 服务已启动"
echo ""

# 步骤 7: 验证部署
echo "✅ 步骤 7/7: 验证部署"
echo "等待服务启动..."
sleep 5

echo ""
echo "容器状态:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep kiro

echo ""
echo "健康检查:"
if curl -s http://127.0.0.1:8088/health | grep -q "ok"; then
    echo "✅ kiro-go 健康检查通过"
else
    echo "⚠️  kiro-go 健康检查失败"
fi

echo ""
echo "=========================================="
echo "  🎉 部署完成！"
echo "=========================================="
echo ""
echo "📊 服务信息:"
echo "  - API 端点: http://115.191.35.73:8088"
echo "  - 管理面板: http://115.191.35.73:8088/admin"
echo "  - 健康检查: http://115.191.35.73:8088/health"
echo ""
echo "📝 下一步:"
echo "  1. 通过 SSH 隧道访问管理面板配置账号"
echo "     本地执行: ssh -L 8088:127.0.0.1:8088 root@115.191.35.73"
echo "     浏览器访问: http://localhost:8088/admin"
echo ""
echo "  2. 查看日志:"
echo "     docker logs kiro-go -f"
echo "     docker logs kiro-gateway -f"
echo ""
echo "  3. 测试 API:"
echo "     curl http://115.191.35.73:8088/v1/models"
echo ""
