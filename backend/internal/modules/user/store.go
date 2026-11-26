package user

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/tx"
)

type Store struct {
	db *sqlx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

func (s *Store) profilesByIDs(ctx context.Context, ids []int64) ([]profile, error) {
	rows := []profile{}
	if len(ids) == 0 {
		return rows, nil
	}

	query, args, err := sqlx.In("SELECT id, nickname FROM user WHERE id IN (?)", ids)
	if err != nil {
		return nil, fmt.Errorf("构造用户批量查询: %w", err)
	}
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &rows, s.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("批量查询用户昵称: %w", err)
	}
	return rows, nil
}
