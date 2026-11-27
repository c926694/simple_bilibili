package tx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type key struct{}

func InTransaction(ctx context.Context) bool {
	_, ok := ctx.Value(key{}).(*sqlx.Tx)
	return ok
}

func WithTx(ctx context.Context, db *sqlx.DB, run func(context.Context) error) (err error) {
	transaction, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务: %w", err)
	}
	defer func() {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("回退事务: %w", rollbackErr))
		}
	}()

	if err := run(context.WithValue(ctx, key{}, transaction)); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("提交事务: %w", err)
	}
	return nil
}

func Executor(ctx context.Context, db *sqlx.DB) sqlx.ExtContext {
	if transaction, ok := ctx.Value(key{}).(*sqlx.Tx); ok {
		return transaction
	}
	return db
}
