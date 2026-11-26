package mysql

import (
	"context"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/config"
)

func Connect(ctx context.Context, cfg config.MySQL) (*sqlx.DB, error) {
	dsn := mysql.NewConfig()
	dsn.User = cfg.User
	dsn.Passwd = cfg.Password
	dsn.Net = "tcp"
	dsn.Addr = cfg.Addr
	dsn.DBName = cfg.Database
	dsn.ParseTime = true
	dsn.Timeout = cfg.ConnectTimeout

	ctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	db, err := sqlx.ConnectContext(ctx, "mysql", dsn.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL %s/%s: %w", cfg.Addr, cfg.Database, err)
	}
	return db, nil
}
