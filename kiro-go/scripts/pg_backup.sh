#!/bin/sh
# pg_backup.sh — PivotStack PostgreSQL nightly logical dump.
#
# 用法（宿主 cron 每日 03:00 调用）：
#   cd /opt/pivotstack/kiro-go && docker compose --profile backup run --rm pg-backup
#
# 输出：./data/pg-backup/pivotstack_YYYY-MM-DD_HHMMSS.dump.gz
# 自动清理：保留 BACKUP_RETENTION_DAYS 天（默认 30）的备份。
#
# 失败时 exit code != 0，docker compose 退出非零；ops 可以 grep failure 报警。
#
# 恢复：使用 scripts/pg_restore.sh。

set -eu

if [ -z "${PGPASSWORD:-}" ]; then
  echo "[pg_backup] PGPASSWORD env required" >&2
  exit 1
fi

PG_HOST="${PG_HOST:-postgres}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-pivotstack}"
PG_DB="${PG_DB:-pivotstack}"
BACKUP_DIR="${BACKUP_DIR:-/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

mkdir -p "$BACKUP_DIR"

STAMP="$(date -u +%Y-%m-%d_%H%M%S)"
DUMP_FILE="$BACKUP_DIR/pivotstack_${STAMP}.dump"
GZ_FILE="${DUMP_FILE}.gz"

echo "[pg_backup] starting at $(date -u)"
echo "[pg_backup] target=$GZ_FILE"

# 用 custom format，便于 pg_restore --clean --if-exists 选择性恢复
pg_dump \
  --host="$PG_HOST" \
  --port="$PG_PORT" \
  --username="$PG_USER" \
  --dbname="$PG_DB" \
  --format=custom \
  --no-owner \
  --no-acl \
  --file="$DUMP_FILE"

gzip -f "$DUMP_FILE"

SIZE=$(wc -c <"$GZ_FILE" | tr -d ' ')
echo "[pg_backup] dump_ok size=${SIZE}B file=$GZ_FILE"

# 清理 > RETENTION_DAYS 的旧备份（按 mtime）
echo "[pg_backup] cleaning > ${RETENTION_DAYS} day backups"
find "$BACKUP_DIR" -maxdepth 1 -type f -name 'pivotstack_*.dump.gz' -mtime +"$RETENTION_DAYS" -print -delete || true

echo "[pg_backup] done"
