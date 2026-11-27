package danmaku

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/tx"
)

const columns = "id, video_id, part_id, user_id, content, time_offset_ms, mode, font_size, color, created_at"

type Store struct {
	db    *sqlx.DB
	redis *redis.Client
}

func NewStore(db *sqlx.DB, client *redis.Client) *Store {
	return &Store{db: db, redis: client}
}

func (s *Store) segment(ctx context.Context, videoID, partID, segment int64) ([]Message, error) {
	items := []Message{}
	start := segment * segmentDurationMS
	err := s.db.SelectContext(ctx, &items, "SELECT "+columns+
		" FROM danmaku WHERE video_id = ? AND part_id = ? AND time_offset_ms >= ? AND time_offset_ms < ? ORDER BY time_offset_ms, id",
		videoID, partID, start, start+segmentDurationMS)
	return items, err
}

func (s *Store) insertBatch(ctx context.Context, items []Message) (int64, error) {
	var sql strings.Builder
	sql.WriteString("INSERT IGNORE INTO danmaku (" + columns + ") VALUES ")
	args := make([]any, 0, len(items)*10)
	for i, item := range items {
		if i != 0 {
			sql.WriteString(",")
		}
		sql.WriteString("(?,?,?,?,?,?,?,?,?,?)")
		args = append(args, item.ID, item.VideoID, item.PartID, item.UserID, item.Content, item.TimeOffsetMS, item.Mode, item.FontSize, item.Color, item.CreatedAt)
	}
	res, err := tx.Executor(ctx, s.db).ExecContext(ctx, sql.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("批量写入弹幕: %w", err)
	}
	return res.RowsAffected()
}

func (s *Store) saveDelta(ctx context.Context, id, videoID, count int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "INSERT INTO danmaku_stat_outbox (id, video_id, danmaku_delta) VALUES (?,?,?)", id, videoID, count)
	return err
}

func (s *Store) pendingDeltas(ctx context.Context) ([]mq.VideoStatDelta, error) {
	var rows []mq.VideoStatDelta
	err := s.db.SelectContext(ctx, &rows, "SELECT id, video_id, danmaku_delta FROM danmaku_stat_outbox ORDER BY id LIMIT 100")
	return rows, err
}

func (s *Store) removeDelta(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM danmaku_stat_outbox WHERE id = ?", id)
	return err
}

func segmentKeys(partID, segment int64) (string, string) {
	bucket := fmt.Sprintf("danmaku:{%d:%d}", partID, segment)
	return bucket + ":history", bucket + ":hot"
}

func (s *Store) addHot(ctx context.Context, item Message, payload string) error {
	_, key := segmentKeys(item.PartID, item.TimeOffsetMS/segmentDurationMS)
	expire := item.CreatedAt.Add(-hotRetention).UnixMilli()
	_, err := s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprint(expire))
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(item.CreatedAt.UnixMilli()), Member: payload})
		pipe.Expire(ctx, key, hotRetention)
		return nil
	})
	return err
}
