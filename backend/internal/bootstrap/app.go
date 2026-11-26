package bootstrap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/infra/logger"
	"simple_bilibili/backend/internal/infra/mysql"
)

type App struct {
	Modules Modules
	config  config.App
	db      *sqlx.DB
	logger  *slog.Logger
}

func Run(serve func(*App, context.Context) error) (err error) {
	path := flag.String("config", "configs/config.yaml", "YAML 配置文件路径")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := New(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, app.Close())
	}()
	return serve(app, ctx)
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	db, err := mysql.Connect(ctx, cfg.MySQL)
	if err != nil {
		return nil, err
	}
	return &App{
		Modules: NewModules(db),
		config:  cfg.App,
		db:      db,
		logger:  logger.New(cfg.App.Mode),
	}, nil
}

func (a *App) Close() error {
	if err := a.db.Close(); err != nil {
		return fmt.Errorf("关闭 MySQL 连接池: %w", err)
	}
	return nil
}
