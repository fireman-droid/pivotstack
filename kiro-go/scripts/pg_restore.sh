#!/bin/sh
# pg_restore.sh — 从 pg_backup.sh 产出的 .dump.gz 恢复 PivotStack 数据库。
#
# 使用：
#   ./scripts/pg_restore.sh ./data/pg-backup/pivotstack_2026-05-21_030000.dump.gz
#
# 注意：
#   - 这是**完整覆盖**操作；恢复前请先停止 kiro-go 应用、备份当前 PG 状态。
#   - --clean --if-exists 会 DROP 现有表后重建。
#   - 恢复后跑 `./kiro-migrate up` 确认 schema_migrations 最新。
#
# Plan §6 恢复演练步骤：
#   1) 新建空 PG 容器或 volume
#   2) docker compose up -d postgres
#   3) bash scripts/pg_restore.sh <dump>
#   4) docker compose run --rm migrate kiro-migrate up
#   5) docker compose up -d kiro-go
#   6) ./kiro-migrate-to-pg verify-only --source=./data （对比 JSON 兜底）

set -eu

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <dump.gz>" >&2
  echo "Example: $0 ./data/pg-backup/pivotstack_2026-05-21_030000.dump.gz" >&2
  exit 1
fi

DUMP_GZ="$1"
if [ ! -f "$DUMP_GZ" ]; then
  echo "[pg_restore] file not found: $DUMP_GZ" >&2
  exit 1
fi

: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD env required (matches docker-compose.yml)}"

PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-pivotstack}"
PG_DB="${PG_DB:-pivotstack}"

# 在宿主机执行：用 docker compose exec 进 postgres 容器，把 dump 流过去
echo "[pg_restore] from $DUMP_GZ -> $PG_USER@$PG_HOST:$PG_PORT/$PG_DB"

# gunzip -c 把 dump 流到 pg_restore；--clean 会先 DROP 现有对象
gunzip -c "$DUMP_GZ" | docker compose exec -T -e "PGPASSWORD=$POSTGRES_PASSWORD" postgres \
  pg_restore \
    --host="$PG_HOST" --port="$PG_PORT" \
    --username="$PG_USER" --dbname="$PG_DB" \
    --clean --if-exists --no-owner --no-acl \
    --verbose

echo "[pg_restore] OK; remember to run: docker compose run --rm migrate kiro-migrate up"
