package danmaku

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/tx"
)

func (s *Service) RunPersistence(ctx context.Context, cfg config.RocketMQ) error {
	client, err := rocketmq.NewPullConsumer(ctx, cfg, mq.DanmakuPersistConsumerGroup,
		rocketmq.WithPullConsumeFromWhere(rocketmq.ConsumeFromFirstOffset))
	if err != nil {
		return err
	}
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return client.RunBatch(ctx, mq.DanmakuPersistTopic, "*", rocketmq.PullBatchConfig{
			BatchSize: 128, PollTimeout: 500 * time.Millisecond, RetryBackoff: time.Second,
		}, s.persistBatch)
	})
	group.Go(func() error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for ctx.Err() == nil {
			if err := s.flushDeltas(ctx); err != nil && ctx.Err() == nil {
				s.logger.ErrorContext(ctx, "投递待发送的弹幕统计", "error", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
		return nil
	})
	return group.Wait()
}

func (s *Service) persistBatch(ctx context.Context, batch []rocketmq.Message) error {
	groups := make(map[int64][]Message)
	for _, msg := range batch {
		var item Message
		if err := msg.DecodeJSON(&item); err != nil || !validEvent(item) {
			s.logger.WarnContext(ctx, "跳过无效的弹幕事件", "msg_id", msg.MsgID, "error", err)
			continue
		}
		groups[item.VideoID] = append(groups[item.VideoID], item)
	}
	ids := make([]int64, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	err := tx.WithTx(ctx, s.store.db, func(ctx context.Context) error {
		for _, id := range ids {
			items := groups[id]
			sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
			// 按视频批量插入，以受影响行数记录实际新增的弹幕。
			count, err := s.store.insertBatch(ctx, items)
			if err != nil {
				return err
			}
			if count > 0 {
				if err := s.store.saveDelta(ctx, s.ids.Next(), id, count); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 同一事务已保存待投递记录，消息重放时不会重新生成相同增量。
	return s.flushDeltas(ctx)
}

func (s *Service) flushDeltas(ctx context.Context) error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	events, err := s.store.pendingDeltas(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		key := strconv.FormatInt(event.EventID, 10)
		if _, err := s.publisher.PublishJSON(ctx, mq.VideoStatSyncTopic, event, rocketmq.WithKeys(key)); err != nil {
			return err
		}
		if err := s.store.removeDelta(ctx, event.EventID); err != nil {
			return err
		}
	}
	return nil
}

func validEvent(item Message) bool {
	return item.ID > 0 && item.VideoID > 0 && item.PartID > 0 && item.UserID > 0 && !item.CreatedAt.IsZero() &&
		item.TimeOffsetMS >= 0 && item.TimeOffsetMS <= math.MaxInt32 && item.Content != "" && utf8.RuneCountInString(item.Content) <= 100 &&
		item.Mode >= 1 && item.Mode <= 3 && allowedFontSize(item.FontSize) && colorPattern.MatchString(item.Color)
}
