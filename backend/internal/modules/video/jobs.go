package video

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/mq"
)

func (s *Service) invalidate(ctx context.Context, code string) error {
	if err := s.store.invalidateDetail(ctx, code, rand.Text()); err != nil {
		return fmt.Errorf("失效稿件详情缓存: %w", err)
	}
	return nil
}

func (s *Service) dispatch(ctx context.Context, row video, p part) error {
	event := mq.VideoTranscodeTask{VideoID: row.ID, PartID: p.ID, AuthorID: row.AuthorID, RawObjectKey: p.RawKey, Attempt: p.Attempt, JobID: p.JobID}
	options := []rocketmq.SendOption{rocketmq.WithKeys(strconv.FormatInt(p.ID, 10), strconv.FormatInt(p.JobID, 10))}
	if delay := time.Until(*p.DispatchAt); delay > 0 {
		options = append(options, rocketmq.WithDelay(delay))
	}
	if _, err := s.publisher.PublishJSON(ctx, mq.VideoTranscodeTopic, event, options...); err != nil {
		return err
	}
	return s.store.markDispatched(ctx, p)
}

func (s *Service) dispatchDue(ctx context.Context) error {
	now := time.Now().UTC()
	parts, err := s.store.dueParts(ctx, now)
	if err != nil {
		return err
	}
	var result error
	for _, p := range parts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.recoverDispatch(ctx, p, now); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (s *Service) recoverDispatch(ctx context.Context, p part, now time.Time) error {
	var row video
	var ready bool
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		var err error
		row, p, err = s.loadJob(ctx, p.VideoID, p.ID)
		if err != nil || row.ID == 0 {
			return err
		}
		switch {
		case p.Status == partRunning && p.LeaseUntil != nil && !p.LeaseUntil.After(now):
			// 执行期限已过，换一个任务标识后重新投递。
			p.JobID, p.DispatchAt = s.ids.Next(), &now
			if err := s.store.schedulePart(ctx, p); err != nil {
				return err
			}
			ready = true
		case p.Status == partWaiting && p.DispatchAt != nil && !p.DispatchAt.After(now):
			ready = true
		}
		return nil
	})
	if err != nil || !ready {
		return err
	}
	return s.dispatch(ctx, row, p)
}

func (s *Service) loadJob(ctx context.Context, id, partID int64) (video, part, error) {
	row, err := s.store.byID(ctx, id, true)
	if errors.Is(err, sql.ErrNoRows) {
		return video{}, part{}, nil
	}
	if err != nil {
		return video{}, part{}, err
	}
	if row.Status == statusWithdrawn {
		return video{}, part{}, nil
	}
	p, err := s.store.partByID(ctx, id, partID)
	if errors.Is(err, sql.ErrNoRows) {
		return video{}, part{}, nil
	}
	return row, p, err
}

func (s *Service) claimJob(ctx context.Context, event mq.VideoTranscodeTask, timeout time.Duration) (video, part, error) {
	var row video
	var p part
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		current, selected, err := s.loadJob(ctx, event.VideoID, event.PartID)
		if err != nil {
			return err
		}
		if current.ID == 0 || selected.Status != partWaiting || selected.JobID != event.JobID ||
			selected.Attempt != event.Attempt || selected.RawKey != event.RawObjectKey || current.AuthorID != event.AuthorID {
			return nil
		}
		// 投递可能早于数据库写入确认，领取后同时清除待投递标记。
		if err := s.store.startPart(ctx, selected, time.Now().UTC().Add(timeout+30*time.Second)); err != nil {
			return err
		}
		row, p = current, selected
		return nil
	})
	return row, p, err
}

func (s *Service) completeJob(ctx context.Context, row video, p part, workErr error, retryDelay time.Duration, canceled bool) error {
	var retry part
	var changed bool
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		current, saved, err := s.loadJob(ctx, row.ID, p.ID)
		if err != nil {
			return err
		}
		if current.ID == 0 || saved.JobID != p.JobID || saved.Status != partRunning {
			return nil
		}
		changed = true
		if canceled || retryDelay > 0 {
			now := time.Now().UTC().Add(retryDelay)
			saved.DispatchAt, saved.JobID = &now, s.ids.Next()
			if !canceled {
				saved.Attempt++
			}
			retry = saved
			return s.store.schedulePart(ctx, saved)
		}
		if workErr != nil {
			p.Status = partFailed
		} else {
			p.Status = partSucceeded
		}
		if err := s.store.finishPart(ctx, p); err != nil {
			return err
		}
		parts, err := s.store.partsByVideoIDs(ctx, []int64{row.ID})
		if err != nil {
			return err
		}
		summarize(&current, parts, time.Now().UTC())
		return s.store.updateVideoState(ctx, current)
	})
	if err != nil || !changed {
		return err
	}
	if err := s.invalidate(ctx, row.BVID); err != nil {
		return err
	}
	if retry.ID != 0 && !canceled {
		if err := s.dispatch(ctx, row, retry); err != nil {
			// 延迟消息发送失败时，由数据库中的计划时间继续安排。
			s.logger.WarnContext(ctx, "重新投递转码任务", "part_id", p.ID, "error", err)
		}
	}
	return nil
}
