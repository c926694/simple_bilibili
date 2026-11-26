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
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/infra/logger"
	"simple_bilibili/backend/internal/infra/mysql"
	redisinfra "simple_bilibili/backend/internal/infra/redis"
	"simple_bilibili/backend/internal/pkg/jwt"
	"simple_bilibili/backend/internal/pkg/snowflake"
)

type App struct {
	Modules Modules
	config  config.App
	db      *sqlx.DB
	redis   *redis.Client
	tokens  *jwt.Manager
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
	ids, err := snowflake.New(cfg.App.NodeID)
	if err != nil {
		return nil, fmt.Errorf("初始化 Snowflake 节点: %w", err)
	}
	db, err := mysql.Connect(ctx, cfg.MySQL)
	if err != nil {
		return nil, err
	}
	client, err := redisinfra.Connect(ctx, cfg.Redis)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	tokens := jwt.New(cfg.JWT)
	return &App{
		Modules: NewModules(db, client, tokens, ids),
		config:  cfg.App,
		db:      db,
		redis:   client,
		tokens:  tokens,
		logger:  logger.New(cfg.App.Mode),
	}, nil
}

func (a *App) Close() error {
	return errors.Join(a.redis.Close(), a.db.Close())
}
