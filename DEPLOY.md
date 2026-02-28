# Kiro Stack Docker 部署指南

## 1. 准备服务器

- 系统：Ubuntu 20.04+ / Debian 11+ / CentOS 8+
- Docker + Docker Compose 已安装
- 开放端口：`8088`（API + 管理面板）

```bash
# 安装 Docker（如果没装）
curl -fsSL https://get.docker.com | sh
sudo systemctl enable docker && sudo systemctl start docker
```

---

## 2. 上传项目

将 `kiro-stack` 整个目录上传到服务器：

```bash
# 方式一：scp
scp -r ./kiro-stack user@your-server:/opt/kiro-stack

# 方式二：git clone（如果有仓库）
cd /opt && git clone https://github.com/your-username/kiro-stack.git
```

---

## 3. 配置环境变量

```bash
cd /opt/kiro-stack
cp .env.example .env
nano .env
```

编辑 `.env`，**必填两项**：

```env
# 管理面板登录密码
ADMIN_PASSWORD=你的密码

# 内部通信密钥（随便生成一个）
INTERNAL_API_KEY=随机字符串
```

生成随机密钥：
```bash
openssl rand -hex 32
```

可选配置：
```env
# 代理（国内服务器需要）
VPN_PROXY_URL=http://127.0.0.1:7890

# 调试模式
DEBUG_MODE=off
```

---

## 4. 启动服务

```bash
docker compose up -d --build
```

首次构建约 2-3 分钟。启动后：

| 地址 | 用途 |
|------|------|
| `http://服务器IP:8088/admin` | 管理面板 |
| `http://服务器IP:8088/v1/chat/completions` | OpenAI 兼容 API |
| `http://服务器IP:8088/v1/messages` | Claude 兼容 API |

---

## 5. 添加账号

### 方式一：管理面板手动添加

1. 打开 `http://服务器IP:8088/admin`
2. 用 `ADMIN_PASSWORD` 登录
3. 点击添加账号（支持 Builder ID / IAM SSO / SSO Token）

### 方式二：脚本批量导入（从 kiro-account-manager 数据库）

```bash
# 本地运行（需要 Python + pymysql + rich）
python import_accounts.py --replenish --min 5

# 查看当前状态
python import_accounts.py --status

# 守护进程自动补充（每5分钟检查，保持至少3个可用）
python import_accounts.py --daemon --min 3 --interval 300
```

---

## 6. 客户端使用

### Claude Code
```bash
export ANTHROPIC_BASE_URL=http://服务器IP:8088
claude
```

### Cursor / Cline / ChatBox
- Base URL: `http://服务器IP:8088`
- Model: `claude-sonnet-4.5`（或其他可用模型）

### curl 测试
```bash
curl http://服务器IP:8088/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4.5","messages":[{"role":"user","content":"Hello"}]}'
```

---

## 7. 常用运维命令

```bash
# 查看日志
docker compose logs -f
docker compose logs kiro-go -f
docker compose logs kiro-gateway -f

# 重启
docker compose restart

# 停止
docker compose down

# 更新代码后重新构建
docker compose up -d --build

# 查看容器状态
docker compose ps
```

---

## 8. 数据持久化

账号数据保存在 `kiro-go/data/config.json`，通过 volume 挂载到宿主机：

```
./kiro-go/data:/app/data
```

备份时只需保存 `kiro-go/data/` 目录和 `.env` 文件。

---

## 9. 反向代理（可选，HTTPS）

如果需要域名 + HTTPS，用 Nginx：

```nginx
server {
    listen 443 ssl;
    server_name api.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8088;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_buffering off;          # 流式响应必须关闭缓冲
        proxy_read_timeout 300s;
    }
}
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
