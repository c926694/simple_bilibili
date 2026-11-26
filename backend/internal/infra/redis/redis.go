package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/config"
)

func Connect(ctx context.Context, cfg config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.Addr, Username: cfg.Username, Password: cfg.Password, DB: cfg.DB,
	})
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("连接 Redis %s: %w", cfg.Addr, err), client.Close())
	}
	return client, nil
}
