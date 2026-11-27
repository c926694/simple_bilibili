package interaction

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/mq"
)

func (s *Service) Run(ctx context.Context, cfg config.RocketMQ) error {
	history, err := rocketmq.NewPullConsumer(ctx, cfg, mq.WatchHistorySyncConsumerGroup,
		rocketmq.WithPullConsumeFromWhere(rocketmq.ConsumeFromFirstOffset))
	if err != nil {
		return err
	}
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return history.RunBatch(ctx, mq.WatchHistorySyncTopic, "*", rocketmq.PullBatchConfig{
			BatchSize: 128, PollTimeout: 500 * time.Millisecond, RetryBackoff: time.Second,
		}, s.persistHistory)
	})
	group.Go(func() error {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				final, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				// 清完待确认批次后再读取本轮尚未取出的播放增量。
				if err := s.flushPlays(final); err != nil {
					return err
				}
				return s.flushPlays(final)
			case <-ticker.C:
				if err := s.flushPlays(ctx); err != nil && ctx.Err() == nil {
					s.logger.ErrorContext(ctx, "刷新播放统计", "error", err)
				}
			}
		}
	})
	group.Go(func() error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if err := s.flushOutbox(ctx); err != nil && ctx.Err() == nil {
				s.logger.ErrorContext(ctx, "发送互动统计待投递记录", "error", err)
			}
			select {
			case <-ctx.Done():
				final, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				return s.flushOutbox(final)
			case <-s.wake:
			case <-ticker.C:
			}
		}
	})
	return group.Wait()
}

func (s *Service) flushOutbox(ctx context.Context) error {
	for {
		events, err := s.store.pendingDeltas(ctx)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := s.publishDelta(ctx, event); err != nil {
				return err
			}
			if err := s.store.removeDelta(ctx, event.EventID); err != nil {
				return err
			}
		}
		if len(events) < 100 {
			return nil
		}
	}
}
