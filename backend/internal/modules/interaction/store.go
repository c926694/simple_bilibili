package interaction

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/tx"
)

type Store struct {
	db    *sqlx.DB
	redis *redis.Client
}

func NewStore(db *sqlx.DB, cache *redis.Client) *Store { return &Store{db: db, redis: cache} }

func (s *Store) interaction(ctx context.Context, userID, videoID int64) (interactionRow, error) {
	var row interactionRow
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, "SELECT id, liked, coin_count FROM video_interaction WHERE user_id = ? AND video_id = ?", userID, videoID)
	if errors.Is(err, sql.ErrNoRows) {
		return interactionRow{}, nil
	}
	return row, err
}

func (s *Store) saveInteraction(ctx context.Context, row interactionRow, userID, videoID int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `INSERT INTO video_interaction (id,user_id,video_id,liked,coin_count)
		VALUES (?,?,?,?,?) AS incoming ON DUPLICATE KEY UPDATE liked = incoming.liked, coin_count = incoming.coin_count`,
		row.ID, userID, videoID, row.Liked, row.CoinCount)
	return err
}

func (s *Store) ledger(ctx context.Context, userID int64, requestID string) (ledgerRow, bool, error) {
	var row ledgerRow
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, "SELECT video_id, coin_count, balance_after FROM coin_ledger WHERE user_id = ? AND request_id = ?", userID, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	return row, err == nil, err
}

func (s *Store) addLedger(ctx context.Context, id, userID, videoID int64, requestID string, coins, balance int) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `INSERT INTO coin_ledger (id,user_id,video_id,request_id,coin_count,balance_after)
		VALUES (?,?,?,?,?,?)`, id, userID, videoID, requestID, coins, balance)
	return err
}

func (s *Store) saveDelta(ctx context.Context, event mq.VideoStatDelta) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `INSERT INTO interaction_stat_outbox (id,video_id,like_delta,coin_delta,favorite_delta,share_delta)
		VALUES (?,?,?,?,?,?)`, event.EventID, event.VideoID, event.LikeDelta, event.CoinDelta, event.FavoriteDelta, event.ShareDelta)
	return err
}

func (s *Store) pendingDeltas(ctx context.Context) ([]mq.VideoStatDelta, error) {
	var rows []mq.VideoStatDelta
	err := s.db.SelectContext(ctx, &rows, "SELECT id, video_id, like_delta, coin_delta, favorite_delta, share_delta FROM interaction_stat_outbox ORDER BY id LIMIT 100")
	return rows, err
}

func (s *Store) removeDelta(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM interaction_stat_outbox WHERE id = ?", id)
	return err
}
