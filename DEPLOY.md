# Kiro Stack 服务器部署指南

## 服务器信息

- 服务器 IP: `115.191.35.73`
- SSH 用户: `root`
- 项目路径: `/var/www/kiro-stack`
- Gitee 仓库: `https://gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git`

## 快速连接

```bash
ssh root@115.191.35.73
```

---

## 1. 准备服务器

```bash
# 连接服务器
ssh root@115.191.35.73

# 安装 Docker
apt update && apt install -y docker.io docker-compose git
systemctl enable --now docker

# 配置镜像加速（国内必做）
cat > /etc/docker/daemon.json << 'EOF'
{
  "registry-mirrors": [
    "https://mirror.ccs.tencentyun.com",
    "https://docker.m.daocloud.io"
  ]
}
EOF
systemctl restart docker
```

---

## 2. 克隆项目

```bash
cd /var/www
git clone https://gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git
cd kiro-stack

# 配置 Git 记住密码
git config credential.helper store
```

---

## 3. 配置环境变量

```bash
# 复制配置文件
cp .env.example .env

# 编辑配置
nano .env
```

修改以下内容：

```env
# 管理面板密码（必填）
ADMIN_PASSWORD=你的强密码

# 内部通信密钥（必填，使用下面命令生成）
INTERNAL_API_KEY=随机生成的密钥

# 可选：代理配置
# VPN_PROXY_URL=http://127.0.0.1:7890

# 调试模式
DEBUG_MODE=off
```

生成随机密钥：

```bash
openssl rand -hex 32
```

---

## 4. 启动服务

```bash
# 构建并启动
docker compose up -d --build

# 查看日志
docker compose logs -f

# 查看状态
docker compose ps
```

首次构建约 2-3 分钟。

**注意：** 项目已配置为只监听 `127.0.0.1`，需要通过 Nginx 反向代理访问。

---

## 5. 配置 Nginx 反向代理

```bash
# 安装 Nginx
apt install nginx -y

# 复制配置文件
cp nginx.conf /etc/nginx/sites-available/kiro-stack

# 修改 server_name（如果有域名）
nano /etc/nginx/sites-available/kiro-stack

# 创建软链接
ln -s /etc/nginx/sites-available/kiro-stack /etc/nginx/sites-enabled/

# 测试配置
nginx -t

# 重启 Nginx
systemctl restart nginx
```

---

## 6. 配置防火墙

```bash
# 开放端口
ufw allow 80/tcp
ufw allow 443/tcp

# 查看状态
ufw status
```

---

## 7. 配置 API Key 认证

通过 SSH 隧道访问管理面板：

```bash
# 在本地电脑运行
ssh -L 8088:127.0.0.1:8088 root@115.191.35.73

# 浏览器访问：http://localhost:8088/admin
# 用 ADMIN_PASSWORD 登录
# 在设置页面配置 API Key 并启用
```

或直接修改配置文件：

```bash
nano /var/www/kiro-stack/kiro-go/data/config.json
```

添加：

```json
{
  "password": "你的管理密码",
  "apiKey": "sk-kiro-your-secret-api-key",
  "requireApiKey": true,
  ...
}
```

重启服务：

```bash
docker compose restart kiro-go
```

---

## 8. 添加 Kiro 账号

1. 通过 SSH 隧道访问管理面板：`http://localhost:8088/admin`
2. 用 `ADMIN_PASSWORD` 登录
3. 点击添加账号（支持 Builder ID / IAM SSO / SSO Token）

---

## 9. 测试部署

```bash
# 健康检查（不需要 API Key）
curl http://115.191.35.73/health

# 测试 API（需要 API Key）
curl http://115.191.35.73/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4.5",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# 查看模型列表
curl http://115.191.35.73/v1/models
```

---

## 10. 日常更新

### 本地推送代码

```bash
# 在本地项目目录
cd D:\E\前端好玩的东西\getcode\kiro-stack

git add .
git commit -m "更新说明"
git push gitee main
```

### 服务器更新

```bash
ssh root@115.191.35.73
cd /var/www/kiro-stack
bash deploy-update.sh
```

---

## 11. 常用命令

```bash
# 查看日志
docker compose logs -f kiro-go
docker compose logs -f kiro-gateway
docker compose logs --tail=100

# 重启服务
docker compose restart

# 停止服务
docker compose down

# 启动服务
docker compose up -d

# 查看状态
docker compose ps

# 查看资源占用
docker stats

# 进入容器
docker exec -it kiro-go sh
docker exec -it kiro-gateway sh

# 备份配置
tar -czf ~/kiro-backup-$(date +%Y%m%d).tar.gz \
  kiro-go/data/config.json .env
```

---

## 12. HTTPS 配置（可选）

```bash
# 安装 Certbot
apt install certbot python3-certbot-nginx -y

# 申请证书（需要域名）
certbot --nginx -d your-domain.com

# 自动续期
crontab -e
# 添加：0 3 * * * certbot renew --quiet
```

---

## 13. 故障排查

### 服务无法启动

```bash
# 查看详细日志
docker compose logs --tail=200

# 检查端口占用
ss -tlnp | grep 8088
ss -tlnp | grep 8001

# 重新构建
docker compose down
docker compose build --no-cache
docker compose up -d
```

### 无法访问

```bash
# 检查 Nginx 状态
systemctl status nginx

# 检查 Nginx 配置
nginx -t

# 查看 Nginx 日志
tail -f /var/log/nginx/kiro-error.log

# 检查防火墙
ufw status
```

### 磁盘空间不足

```bash
# 清理 Docker
docker system prune -f
docker builder prune -f

# 查看磁盘使用
df -h
du -sh /var/lib/docker
```

---

## 架构图

```
客户端 (Claude Code / Cursor / Cline)
        │
        ▼  :8088
   ┌─────────────┐
   │   kiro-go    │  管理面板 + 账号池 + Token 刷新
   └──────┬──────┘
          │ (内部转发)
          ▼  :8000 (容器内部)
   ┌──────────────────┐
   │   kiro-gateway   │  稳定代理层：双端点回退 + 自动重试
   └──────┬───────────┘
          │
          ▼
      Kiro API (AWS)
```
