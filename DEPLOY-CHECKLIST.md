# 部署前检查清单

## 本地准备（在推送代码前完成）

### 1. 配置文件检查
- [ ] `docker-compose.yml` 已修改为 host 网络模式 ✅
- [ ] `.env.example` 文件存在且完整 ✅
- [ ] `deploy-update.sh` 脚本存在且可执行 ✅

### 2. 端口配置确认
- [ ] kiro-go 监听端口：需要 8088（当前默认 8080）⚠️
- [ ] kiro-gateway 监听端口：需要 8001（当前默认 8000）⚠️

**解决方案**：
- kiro-gateway: 通过环境变量 `SERVER_PORT=8001` 设置
- kiro-go: 需要在 config.json 中设置或修改代码支持环境变量

### 3. Git 推送
- [ ] 所有修改已提交到本地
- [ ] 推送到 Gitee 仓库

```bash
git add .
git commit -m "配置服务器部署：host 网络模式 + 端口适配"
git push gitee main
```

---

## 服务器部署（SSH 到服务器后执行）

### 步骤 1：连接服务器
```bash
ssh root@115.191.35.73
# 密码: Lin20050201
```

### 步骤 2：检查环境
```bash
# 查看 Docker 版本
docker --version

# 检查端口占用
ss -tlnp | grep -E ':(8088|8001)\s'

# 如果端口被占用，需要停止占用进程
```

### 步骤 3：克隆项目
```bash
cd /var/www

# 克隆项目（带凭据）
git clone https://18825133336:Lin20050201@gitee.com/ji-bo-chang-oli-gave-it-to/kiro-stack.git

cd kiro-stack

# 配置 Git 记住密码
git config credential.helper store
```

### 步骤 4：配置环境变量
```bash
# 复制配置文件
cp .env.example .env

# 生成随机密钥
openssl rand -hex 32

# 编辑配置
nano .env
```

修改内容：
```env
ADMIN_PASSWORD=Lin20050201_kiro_admin
INTERNAL_API_KEY=<生成的随机密钥>
DEBUG_MODE=off
```

### 步骤 5：修改 docker-compose.yml（添加端口配置）
```bash
nano docker-compose.yml
```

在 `kiro-gateway` 的 `environment` 部分添加：
```yaml
- SERVER_PORT=8001
```

在 `kiro-go` 的 `environment` 部分添加：
```yaml
- PORT=8088
- HOST=0.0.0.0
```

### 步骤 6：开放防火墙端口
```bash
# 开放 8088 端口
ufw allow 8088/tcp

# 查看防火墙状态
ufw status
```

### 步骤 7：构建并启动服务
```bash
# 加载环境变量
export $(grep -v '^#' .env | xargs)

# 构建镜像
docker compose build

# 启动服务
docker compose up -d

# 查看日志
docker compose logs -f
```

### 步骤 8：验证部署
```bash
# 查看容器状态
docker ps | grep kiro

# 测试健康检查
curl http://127.0.0.1:8088/health

# 查看日志
docker logs kiro-go --tail=50
docker logs kiro-gateway --tail=50
```

### 步骤 9：配置 API Key
通过 SSH 隧道访问管理面板：

在本地电脑执行：
```bash
ssh -L 8088:127.0.0.1:8088 root@115.191.35.73
```

浏览器访问：`http://localhost:8088/admin`

### 步骤 10：测试 API
```bash
curl http://115.191.35.73:8088/v1/models
```

---

## 问题排查

### 端口配置问题
如果 kiro-go 没有监听 8088 端口：
1. 检查环境变量是否生效
2. 查看容器日志确认监听端口
3. 如果需要，手动修改 config.json 中的 port 字段

### 容器无法启动
```bash
docker logs kiro-go --tail=200
docker logs kiro-gateway --tail=200
```

### 网络连接问题
```bash
# 在 kiro-go 容器内测试连接 kiro-gateway
docker exec -it kiro-go sh
curl http://127.0.0.1:8001/health
```
