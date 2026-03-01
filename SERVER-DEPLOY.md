# Kiro Stack 服务器部署指南

> 基于服务器 `115.191.35.73` 实际环境编写，与 kiro-account-shop 同服务器部署。

---

## 一、服务器信息

| 项目 | 值 |
|------|------|
| 服务器 IP | `115.191.35.73` |
| SSH 用户 | `root` |
| SSH 密码 | `Lin20050201` |
| 项目路径 | `/var/www/kiro-stack` |
| Gitee 仓库 | `https://gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git` |
| Gitee 账号 | `18825133336` |
| Gitee 密码 | `Lin20050201` |
| 数据库密码 | `Lin20050201` |

### 端口规划（同服务器所有项目）

| 端口 | 项目 | 服务 |
|------|------|------|
| 80 | kiro-account-manager | 前端（宿主机 nginx） |
| 3306 | MySQL | 数据库（所有项目共用） |
| 3457 | kiro-account-shop | 后端 |
| 5001 | carManager | 后端 |
| **8001** | **kiro-stack** | **kiro-gateway（内部）** |
| 8080 | carManager | 前端 |
| 8082 | kiro-account-shop | 前端 |
| **8088** | **kiro-stack** | **kiro-go（对外 API + 管理面板）** |
| 19457 | kiro-account-manager | 后端（PM2） |

---

## 二、部署架构

```
客户端 (Claude Code / Cursor / Cline)
        │
        ▼  :8088
   ┌─────────────┐
   │   kiro-go   │  管理面板 + 账号池 + Token 刷新
   │  (Docker)   │  --network host
   └──────┬──────┘
          │ (内部转发 127.0.0.1:8001)
          ▼  :8001
   ┌──────────────────┐
   │   kiro-gateway   │  稳定代理层：双端点回退 + 自动重试
   │    (Docker)      │  --network host
   └──────┬───────────┘
          │
          ▼
      Kiro API (AWS)
```

**关键点**：
- 两个容器都用 `--network host` 模式（与服务器其他项目保持一致）
- 不做端口映射，直接监听宿主机端口
- kiro-go 监听 8088（对外），kiro-gateway 监听 8001（内部）

---

## 三、首次部署（一步一步执行）

### 步骤 1：连接服务器

```bash
ssh root@115.191.35.73
# 密码: Lin20050201
```

### 步骤 2：检查环境

```bash
# 查看 Docker 版本
docker --version

# 查看 /var/www 目录
ls -la /var/www/

# 查看当前运行的容器
docker ps

# 检查端口 8088 和 8001 是否被占用
ss -tlnp | grep -E ':(8088|8001)\s'
```

如果端口被占用，需要修改端口配置。

### 步骤 3：克隆项目

```bash
cd /var/www

# 克隆项目（带凭据，避免输密码）
git clone https://18825133336:Lin20050201@gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git

cd kiro-stack

# 配置 Git 记住密码（后续 git pull 不用输密码）
git config credential.helper store
```

### 步骤 4：配置环境变量

```bash
# 复制配置文件
cp .env.example .env

# 编辑配置
nano .env
```

修改以下内容（按 `Ctrl+X` 保存，按 `Y` 确认，按 `Enter` 退出）：

```env
# 管理面板密码（必填）
ADMIN_PASSWORD=Lin20050201_kiro_admin

# 内部通信密钥（必填，下面命令生成）
INTERNAL_API_KEY=生成的随机密钥

# 可选：代理配置（如果需要）
# VPN_PROXY_URL=http://127.0.0.1:7890

# 调试模式
DEBUG_MODE=off
```

生成随机密钥：

```bash
openssl rand -hex 32
```

把生成的密钥复制到 `.env` 文件的 `INTERNAL_API_KEY=` 后面。

### 步骤 5：修改 docker-compose.yml（适配 host 网络）

```bash
nano docker-compose.yml
```

修改为以下内容：

```yaml
services:
  kiro-gateway:
    build: ./kiro-gateway
    image: kiro-gateway-local:latest
    container_name: kiro-gateway
    restart: unless-stopped
    network_mode: host
    environment:
      - PROXY_API_KEY=${INTERNAL_API_KEY}
      - VPN_PROXY_URL=${VPN_PROXY_URL:-}
      - DEBUG_MODE=${DEBUG_MODE:-off}
      - SKIP_STARTUP_CREDENTIAL_CHECK=true

  kiro-go:
    build: ./kiro-go
    container_name: kiro-go
    restart: unless-stopped
    network_mode: host
    depends_on:
      - kiro-gateway
    volumes:
      - ./kiro-go/data:/app/data
    environment:
      - CONFIG_PATH=/app/data/config.json
      - ADMIN_PASSWORD=${ADMIN_PASSWORD}
      - KIRO_GATEWAY_BASE=http://127.0.0.1:8001
      - KIRO_GATEWAY_API_KEY=${INTERNAL_API_KEY}
```

### 步骤 6：开放防火墙端口

```bash
# 开放 8088 端口（对外 API）
ufw allow 8088/tcp

# 8001 端口不需要开放（仅内部使用）

# 查看防火墙状态
ufw status
```

### 步骤 7：构建并启动服务

```bash
# 加载环境变量
export $(grep -v '^#' .env | xargs)

# 构建镜像（首次会比较慢，约 3-5 分钟）
docker compose build

# 启动服务
docker compose up -d

# 查看启动日志
docker compose logs -f
```

按 `Ctrl+C` 退出日志查看。

### 步骤 8：验证部署

```bash
# 1. 查看容器状态（应该显示两个容器都在运行）
docker ps | grep kiro

# 2. 测试健康检查
curl http://127.0.0.1:8088/health

# 3. 查看日志
docker logs kiro-go --tail=50
docker logs kiro-gateway --tail=50
```

如果健康检查返回 `{"status":"ok",...}`，说明部署成功！

### 步骤 9：配置 API Key 认证

**方式 1：通过 SSH 隧道访问管理面板（推荐）**

在**本地电脑**新开一个终端，执行：

```bash
ssh -L 8088:127.0.0.1:8088 root@115.191.35.73
# 密码: Lin20050201
```

保持这个连接，然后在浏览器访问：`http://localhost:8088/admin`

用 `ADMIN_PASSWORD` 登录，在设置页面配置 API Key 并启用。

**方式 2：直接修改配置文件**

```bash
# 等服务启动后，编辑配置
nano /var/www/kiro-stack/kiro-go/data/config.json
```

找到并修改：

```json
{
  "password": "Lin20050201_kiro_admin",
  "apiKey": "sk-kiro-your-secret-api-key-here",
  "requireApiKey": true,
  ...
}
```

重启服务：

```bash
docker restart kiro-go
```

### 步骤 10：测试 API

```bash
# 测试 API（需要 API Key）
curl http://115.191.35.73:8088/v1/chat/completions \
  -H "Authorization: Bearer sk-kiro-your-secret-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4.5",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'

# 查看模型列表（不需要 API Key）
curl http://115.191.35.73:8088/v1/models
```

---

## 四、日常更新

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
# SSH 到服务器
ssh root@115.191.35.73

cd /var/www/kiro-stack

# 使用一键更新脚本
bash deploy-update.sh
```

---

## 五、常用命令

```bash
# ========== 查看状态 ==========
docker ps | grep kiro                    # 查看 kiro 相关容器
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"

# ========== 查看日志 ==========
docker logs kiro-go --tail=50            # 查看最后 50 行
docker logs kiro-go -f                   # 实时跟踪
docker logs kiro-gateway --tail=50
docker logs kiro-gateway -f

# ========== 重启服务 ==========
docker restart kiro-go
docker restart kiro-gateway
docker compose restart                   # 重启所有

# ========== 停止服务 ==========
docker stop kiro-go kiro-gateway
docker compose down

# ========== 启动服务 ==========
docker compose up -d

# ========== 进入容器调试 ==========
docker exec -it kiro-go sh
docker exec -it kiro-gateway sh

# ========== 查看端口占用 ==========
ss -tlnp | grep -E '8088|8001'

# ========== 备份配置 ==========
tar -czf ~/kiro-backup-$(date +%Y%m%d).tar.gz \
  /var/www/kiro-stack/kiro-go/data/config.json \
  /var/www/kiro-stack/.env
```

---

## 六、故障排查

### 1. 容器无法启动

```bash
# 查看详细日志
docker logs kiro-go --tail=200
docker logs kiro-gateway --tail=200

# 检查端口占用
ss -tlnp | grep -E '8088|8001'

# 如果端口被占用，停止占用进程或修改端口
kill -9 <PID>

# 重新构建
docker compose down
docker compose build --no-cache
docker compose up -d
```

### 2. API 返回 503 错误

```bash
# 检查 kiro-gateway 是否正常运行
docker ps | grep kiro-gateway

# 检查 kiro-go 能否连接 kiro-gateway
docker exec -it kiro-go sh
# 在容器内执行：
curl http://127.0.0.1:8001/health
```

### 3. 管理面板无法访问

```bash
# 检查 kiro-go 是否正常运行
docker logs kiro-go --tail=50

# 检查端口是否监听
ss -tlnp | grep 8088

# 检查防火墙
ufw status | grep 8088
```

### 4. Git pull 需要输密码

```bash
cd /var/www/kiro-stack

# 重新配置 Git 凭据
git config credential.helper store

# 或者修改远程仓库 URL（带凭据）
git remote set-url origin https://18825133336:Lin20050201@gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git
```

### 5. 磁盘空间不足

```bash
# 查看磁盘使用
df -h

# 清理 Docker
docker system prune -f
docker builder prune -f

# 查看 Docker 占用
du -sh /var/lib/docker
```

---

## 七、客户端使用

### Claude Code

```bash
export ANTHROPIC_BASE_URL=http://115.191.35.73:8088
export ANTHROPIC_API_KEY=sk-kiro-your-secret-api-key-here
claude
```

### Cursor / Cline / ChatBox

- Base URL: `http://115.191.35.73:8088`
- API Key: `sk-kiro-your-secret-api-key-here`
- Model: `claude-sonnet-4.5`（或其他可用模型）

### curl 测试

```bash
curl http://115.191.35.73:8088/v1/chat/completions \
  -H "Authorization: Bearer sk-kiro-your-secret-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4.5",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## 八、安全建议

1. **修改默认密码**：部署后立即修改 `ADMIN_PASSWORD` 和 `INTERNAL_API_KEY`
2. **启用 API Key**：必须启用 API Key 认证，防止未授权访问
3. **定期备份**：定期备份 `kiro-go/data/config.json` 和 `.env` 文件
4. **监控日志**：定期查看日志，及时发现异常访问
5. **更新密钥**：定期更换 API Key

---

## 九、访问地址

- **API 端点**: `http://115.191.35.73:8088`
- **管理面板**: `http://115.191.35.73:8088/admin`（需要 SSH 隧道或配置 Nginx）
- **健康检查**: `http://115.191.35.73:8088/health`
- **模型列表**: `http://115.191.35.73:8088/v1/models`

---

## 十、注意事项

1. **服务器 IP 可能变化**：服务器没绑弹性 IP，重启后公网 IP 可能变化
2. **使用 host 网络**：与服务器其他项目保持一致，使用 `--network host` 模式
3. **内部通信**：kiro-go 通过 `http://127.0.0.1:8001` 连接 kiro-gateway
4. **数据持久化**：账号数据保存在 `kiro-go/data/config.json`，已通过 volume 挂载
5. **防火墙配置**：只需开放 8088 端口，8001 端口仅内部使用

---

## 十一、容器信息

| 容器名 | 镜像 | 网络模式 | 端口 | 作用 |
|------|------|------|------|------|
| `kiro-gateway` | `kiro-gateway-local:latest` | host | 8001 | 稳定代理层 |
| `kiro-go` | `kiro-go:latest` | host | 8088 | 管理面板 + 账号池 |
