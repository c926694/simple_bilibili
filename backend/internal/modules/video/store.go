package video

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/pkg/tx"
)

type Store struct {
	db    *sqlx.DB
	redis *redis.Client
}

func NewStore(db *sqlx.DB, client *redis.Client) *Store {
	return &Store{db: db, redis: client}
}

func (s *Store) videosByIDs(ctx context.Context, ids []int64) ([]video, error) {
	rows := []video{}
	if len(ids) == 0 {
		return rows, nil
	}

	query, args, err := sqlx.In("SELECT id, author_id, title, status FROM video WHERE id IN (?)", ids)
	if err != nil {
		return nil, fmt.Errorf("构造视频批量查询: %w", err)
	}
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &rows, s.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("批量查询视频: %w", err)
	}
	return rows, nil
}
