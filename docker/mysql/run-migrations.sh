#!/bin/sh
set -eu

MYSQL_HOST="${MYSQL_HOST:-mysql}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-simple_bilibili}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-local_mysql_password}"
MYSQL_DATABASE="${MYSQL_DATABASE:-simple_bilibili}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

export MYSQL_PWD="${MYSQL_PASSWORD}"

mysql_exec() {
  mysql \
    --default-character-set=utf8mb4 \
    -h"${MYSQL_HOST}" \
    -P"${MYSQL_PORT}" \
    -u"${MYSQL_USER}" \
    --batch \
    --skip-column-names \
    "${MYSQL_DATABASE}" \
    "$@"
}

echo "初始化迁移版本记录表 schema_migrations..."
mysql_exec <<'SQL'
CREATE TABLE IF NOT EXISTS `schema_migrations` (
  `version` VARCHAR(128) NOT NULL COMMENT '迁移脚本文件名版本号',
  `applied_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '迁移执行完成时间',
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据库结构迁移版本记录表';
SQL

files="$(find "${MIGRATIONS_DIR}" -maxdepth 1 -type f -name '*.up.sql' | sort)"

if [ -z "${files}" ]; then
  echo "未在 ${MIGRATIONS_DIR} 发现 .up.sql 迁移脚本"
  exit 0
fi

for file in ${files}; do
  version="$(basename "${file}")"
  exists="$(mysql_exec -e "SELECT COUNT(1) FROM schema_migrations WHERE version = '${version}';")"
  if [ "${exists}" -gt 0 ]; then
    echo "跳过已执行的迁移脚本: ${version}"
    continue
  fi

  echo "开始执行迁移脚本: ${version}"
  mysql \
    --default-character-set=utf8mb4 \
    -h"${MYSQL_HOST}" \
    -P"${MYSQL_PORT}" \
    -u"${MYSQL_USER}" \
    "${MYSQL_DATABASE}" < "${file}"

  mysql_exec -e "INSERT INTO schema_migrations (version) VALUES ('${version}');"
  echo "迁移脚本执行完成: ${version}"
done

echo "当前已应用的数据库迁移版本:"
mysql_exec -e "SELECT version, applied_at FROM schema_migrations ORDER BY version ASC;"
