package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"simple_bilibili/backend/internal/config"
)

const createMigrationsTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version VARCHAR(128) NOT NULL COMMENT '迁移脚本文件名版本号',
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '迁移执行完成时间',
	PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='数据库结构迁移版本记录表'`

func Migrate(ctx context.Context, cfg config.MySQL, files fs.FS) (err error) {
	dsn := connectionConfig(cfg)
	dsn.MultiStatements = true
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		return fmt.Errorf("打开 MySQL 迁移连接: %w", err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("连接 MySQL 以执行迁移: %w", err)
	}
	defer func() { err = errors.Join(err, conn.Close()) }()

	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('simple_bilibili_schema_migrate', 30)").Scan(&acquired); err != nil {
		return fmt.Errorf("获取数据库迁移锁: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("未获取数据库迁移锁")
	}
	defer func() {
		// MySQL 会话锁须在原连接释放，启动取消后仍尝试释放。
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ConnectTimeout)
		defer cancel()
		var released sql.NullInt64
		switch releaseErr := conn.QueryRowContext(releaseCtx, "SELECT RELEASE_LOCK('simple_bilibili_schema_migrate')").Scan(&released); {
		case releaseErr != nil:
			err = errors.Join(err, fmt.Errorf("释放数据库迁移锁: %w", releaseErr))
		case !released.Valid || released.Int64 != 1:
			err = errors.Join(err, errors.New("未释放数据库迁移锁"))
		}
	}()

	if _, err := conn.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("创建数据库迁移版本记录表: %w", err)
	}
	versions, err := fs.Glob(files, "*.up.sql")
	if err != nil {
		return fmt.Errorf("列出数据库迁移脚本: %w", err)
	}
	slices.Sort(versions)
	for _, version := range versions {
		if err := applyMigration(ctx, conn, files, version); err != nil {
			return fmt.Errorf("迁移 %s: %w", version, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, conn *sql.Conn, files fs.FS, version string) error {
	var applied bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", version).Scan(&applied); err != nil {
		return fmt.Errorf("查询迁移版本: %w", err)
	}
	if applied {
		return nil
	}

	script, err := fs.ReadFile(files, version)
	if err != nil {
		return fmt.Errorf("读取迁移脚本: %w", err)
	}
	if _, err := conn.ExecContext(ctx, string(script)); err != nil {
		return fmt.Errorf("执行迁移脚本: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("记录迁移版本: %w", err)
	}
	return nil
}
