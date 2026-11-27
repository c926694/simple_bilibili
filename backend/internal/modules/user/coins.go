package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/tx"
)

func (s *Service) CoinBalance(ctx context.Context, userID int64) (int, error) {
	var balance int
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.store.db), &balance, "SELECT coin_balance FROM user WHERE id = ?", userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errcode.UserNotFound
	}
	return balance, err
}

func (s *Service) LockForUpdate(ctx context.Context, userID int64) error {
	var id int64
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.store.db), &id, "SELECT id FROM user WHERE id = ? FOR UPDATE", userID)
	if errors.Is(err, sql.ErrNoRows) {
		return errcode.UserNotFound
	}
	return err
}

func (s *Service) ChangeCoins(ctx context.Context, userID int64, delta int) (int, error) {
	if tx.InTransaction(ctx) {
		// 互动流水和签到奖励沿用调用方的事务。
		return s.changeCoins(ctx, userID, delta)
	}
	var balance int
	err := tx.WithTx(ctx, s.store.db, func(ctx context.Context) error {
		var err error
		// 独立调用时由用户模块负责提交余额修改。
		balance, err = s.changeCoins(ctx, userID, delta)
		return err
	})
	return balance, err
}

func (s *Service) changeCoins(ctx context.Context, userID int64, delta int) (int, error) {
	conn := tx.Executor(ctx, s.store.db)
	for attempt := 0; attempt < 4; attempt++ {
		var row struct {
			Balance int `db:"coin_balance"`
			Version int `db:"version"`
		}
		query := "SELECT coin_balance, version FROM user WHERE id = ?"
		if attempt > 0 {
			// 冲突后读取当前版本，避免重复读取事务快照。
			query += " FOR UPDATE"
		}
		if err := sqlx.GetContext(ctx, conn, &row, query, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, errcode.UserNotFound
			}
			return 0, err
		}
		balance := row.Balance + delta
		if balance < 0 {
			return row.Balance, errcode.InsufficientCoins
		}
		if delta == 0 {
			return balance, nil
		}
		result, err := conn.ExecContext(ctx, "UPDATE user SET coin_balance = ?, version = version + 1 WHERE id = ? AND version = ?", balance, userID, row.Version)
		if err != nil {
			return 0, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if changed == 1 {
			return balance, nil
		}
	}
	return 0, fmt.Errorf("用户 %d 的代币余额版本多次冲突", userID)
}
