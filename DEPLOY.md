# Kiro Stack 部署指南（任何人/AI 部署前必读）

> **⚠️ 绝对不要在 `/var/www/kiro-stack/` 根目录跑 `docker compose up`！**
> 真生产入口是端口 **8990**，compose 文件在 **`kiro-go/` 子目录**。
> 历史上根目录有过 `docker-compose.yml`（端口 8088），是个废弃的开发副本，已删除。

## 真生产入口

| 项 | 值 |
|---|---|
| 公网入口 | `http://115.191.35.73:8990/admin/` |
| Compose 文件 | `kiro-go/docker-compose.yml` ← **子目录里这一个** |
| 项目名 | `kiro-go` |
| 容器名 | `kiro-go-kiro-go-1` |
| 镜像名 | `kiro-go-kiro-go` |
| 不依赖 gateway | ✅ 独立运行 |

## 标准部署流程

### 本地修改 → 推 gitee

```bash
cd /path/to/kiro-stack
git add .
git commit -m "改动说明"
git push gitee main      # remote 'gitee' 带凭证，能直接 push
```

### 服务器 SSH 部署

```bash
ssh root@115.191.35.73         # SSH MCP 名为 my-server

cd /var/www/kiro-stack
git fetch origin main
git reset --hard origin/main   # ← 不允许在服务器手改代码累积分歧

cd /var/www/kiro-stack/kiro-go     # ← 这一步是关键，进 kiro-go 子目录
docker compose up -d --build kiro-go
```

约 60 秒完成（前端 vite build + go build）。

## 服务器关键文件

| 文件 | 路径 | git 状态 |
|---|---|---|
| 真生产 compose | `kiro-go/docker-compose.yml` | ✅ tracked，绝不能再删 |
| Dockerfile | `kiro-go/Dockerfile` | ✅ tracked |
| 账号配置 | `kiro-go/data/config.json` | ❌ ignored（含 token） |
| 调用日志 | `kiro-go/data/call_logs.jsonl` | ❌ ignored |
| 环境变量 | `.env` | ❌ ignored |
| clash 节点 | `clash/config.yaml` | ❌ ignored（含密码），备份在 `/root/kiro-clash-backup/config.yaml` |

## 出口代理（clash-meta）

业务调用 AWS Kiro/CodeWhisperer，必须**走台湾或美国出口** —— 香港会被 AWS 返回 INVALID_MODEL_ID。

- 容器：`clash-meta`（host network）
- 监听：`:7890` (http) `:7891` (socks)
- 配置：`/var/www/kiro-stack/clash/config.yaml`（5 个台湾 anytls 节点）
- 启动：`docker run -d --name clash-meta --restart unless-stopped --network host -v /var/www/kiro-stack/clash/config.yaml:/root/.config/mihomo/config.yaml:ro metacubex/mihomo:latest`

**万一 clash/config.yaml 被 git clean 误删**：

```bash
cp /root/kiro-clash-backup/config.yaml /var/www/kiro-stack/clash/config.yaml
docker restart clash-meta
```

## 不能在服务器手改代码

服务器的 git 仓库**只用于 pull**。修改流程严格走：

```
本地编辑 → git push gitee main → 服务器 git pull → docker compose up --build
```

历史教训：服务器曾累积过 39 个未提交修改（6446 行）+ detached HEAD 状态，导致 git pull 不能用。这些工作已永久备份在 gitee `server-backup-20260504` 分支，**禁止再这样操作**。

## 常用诊断

```bash
# 容器状态
docker ps --filter name=kiro-go-kiro-go-1
docker logs kiro-go-kiro-go-1 --tail 30

# 调用日志（最近）
docker exec kiro-go-kiro-go-1 tail -10 /app/data/call_logs.jsonl

# clash 出口 IP（应是台湾）
curl -s -x http://127.0.0.1:7890 https://ipinfo.io/json

# 健康
curl http://localhost:8990/admin/   # 应 200
curl http://localhost:8990/health   # 应 200

# git 状态健康检查
cd /var/www/kiro-stack
git rev-parse --abbrev-ref HEAD     # 应是 "main" 不是 "HEAD"
git log -1 --oneline                # 应和 gitee/main 一致
```

## 模型路由 / 掺水链路（设计意图）

```
用户传 model: opus 4.7
  → ResolveModelPool       → pro 池（关键词 "opus"）
  → DeterminePoolTier      → pro 池（同一逻辑）
  → ParseModelAndThinking  → claude-opus-4.6   ← billingModel（按 opus 计费）
  → ApplyStealth (95%)     → claude-sonnet-4.5  ← upstreamModel（实际发上游）
  → AWS Kiro               → 200
```

掺水盈利保留：用户付 opus PRO 价，上游真实用便宜 sonnet。
Stealth 配置：`kiro-go/data/config.json` 里 `stealth.opusFakeTarget` / `sonnetFakeTarget`。

## 故障排查（按现象）

| 现象 | 大概率原因 |
|---|---|
| 全部调用 `connection refused: 7890` | clash-meta 没起，参考"出口代理"恢复 |
| 全部 `INVALID_MODEL_ID` 拒绝 | clash 出口不是 TW/US（如切到 HK） |
| 部分调用 `INVALID_MODEL_ID` | stealth 偷换 target 写错（如 4.6 vs 4.5），看 AWS 当前接受哪个 model id |
| 503 `No available accounts in pool` | pool 选错（DeterminePoolTier 没识别该 model 名） |
| `git pull` 报冲突 | 服务器有未提交改动，禁止手改，先 stash 再 pull |
