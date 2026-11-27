package video

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/tx"
)

func (s *Service) RunStatSync(ctx context.Context, cfg config.RocketMQ) error {
	client, err := rocketmq.NewPullConsumer(ctx, cfg, mq.VideoStatSyncConsumerGroup,
		rocketmq.WithPullConsumeFromWhere(rocketmq.ConsumeFromFirstOffset))
	if err != nil {
		return err
	}
	return client.RunBatch(ctx, mq.VideoStatSyncTopic, "*", rocketmq.PullBatchConfig{
		BatchSize: 64, PollTimeout: 500 * time.Millisecond, RetryBackoff: time.Second,
	}, s.consumeStatBatch)
}

func (s *Service) consumeStatBatch(ctx context.Context, batch []rocketmq.Message) error {
	for _, msg := range batch {
		var event mq.VideoStatDelta
		if err := msg.DecodeJSON(&event); err != nil || event.EventID <= 0 || event.VideoID <= 0 {
			s.logger.WarnContext(ctx, "跳过无效的视频统计增量", "msg_id", msg.MsgID, "error", err)
			continue
		}
		bvid, err := s.store.applyStatDelta(ctx, event)
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.WarnContext(ctx, "跳过不存在的视频统计增量", "video_id", event.VideoID)
			continue
		}
		if err != nil {
			return err
		}
		// 重投即使已经写入统计，也重新失效缓存，补齐此前未完成的刷新。
		if err := s.invalidate(ctx, bvid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyStatDelta(ctx context.Context, event mq.VideoStatDelta) (string, error) {
	fields := []struct {
		column string
		delta  int64
	}{
		{"play_count", event.PlayDelta}, {"danmaku_count", event.DanmakuDelta}, {"like_count", event.LikeDelta},
		{"coin_count", event.CoinDelta}, {"favorite_count", event.FavoriteDelta}, {"share_count", event.ShareDelta}, {"comment_count", event.CommentDelta},
	}
	updates := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+1)
	for _, field := range fields {
		if field.delta == 0 {
			continue
		}
		updates = append(updates, field.column+" = "+field.column+" + ?")
		args = append(args, field.delta)
	}

	var bvid string
	err := s.inTransaction(ctx, func(ctx context.Context) error {
		conn := tx.Executor(ctx, s.db)
		if err := sqlx.GetContext(ctx, conn, &bvid, "SELECT bv_id FROM video WHERE id = ?", event.VideoID); err != nil {
			return err
		}
		if len(updates) == 0 {
			return nil
		}
		// 增量与处理记录同一事务提交，重复事件不会再次增加计数。
		result, err := conn.ExecContext(ctx, "INSERT IGNORE INTO video_stat_event (id, video_id) VALUES (?, ?)", event.EventID, event.VideoID)
		if err != nil {
			return fmt.Errorf("写入视频统计处理记录: %w", err)
		}
		inserted, err := result.RowsAffected()
		if err != nil || inserted == 0 {
			return err
		}
		result, err = conn.ExecContext(ctx, "UPDATE video_stat SET "+strings.Join(updates, ", ")+" WHERE video_id = ?", append(args, event.VideoID)...)
		if err != nil {
			return fmt.Errorf("更新视频统计: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return fmt.Errorf("视频 %d 没有统计记录", event.VideoID)
		}
		return nil
	})
	return bvid, err
}
