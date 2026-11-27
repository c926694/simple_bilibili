package upload

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/infra/minio"
)

const (
	cleanupLockKey = "upload:cleanup:lock"
	cleanupLockTTL = 5 * time.Minute
)

var releaseCleanupLock = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

type RawKeyRefChecker func(ctx context.Context, keys []string) (map[string]bool, error)

func (s *Service) RunCleanup(ctx context.Context, checkRefs RawKeyRefChecker) error {
	ticker := time.NewTicker(s.cleanup.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.CleanupOnce(ctx, checkRefs); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.WarnContext(ctx, "执行过期上传清理失败", "error", err)
			}
		}
	}
}

func (s *Service) CleanupOnce(ctx context.Context, checkRefs RawKeyRefChecker) error {
	token := rand.Text()
	acquired, err := s.redis.SetNX(ctx, cleanupLockKey, token, cleanupLockTTL).Result()
	if err != nil || !acquired {
		return err
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if _, releaseErr := releaseCleanupLock.Run(releaseCtx, s.redis, []string{cleanupLockKey}, token).Result(); releaseErr != nil {
			s.logger.WarnContext(ctx, "释放上传清理锁失败", "error", releaseErr)
		}
	}()

	cutoff := time.Now().UTC().Add(-s.cleanup.ExpireAge)
	if err := s.cleanupUploadingTasks(ctx, cutoff); err != nil {
		return err
	}
	return s.cleanupUnreferencedCompletedTasks(ctx, cutoff, checkRefs)
}

func (s *Service) cleanupUploadingTasks(ctx context.Context, cutoff time.Time) error {
	var afterID int64
	for {
		rows, err := s.store.expiredTasksBatch(ctx, statusUploading, cutoff, afterID, s.cleanup.BatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if row.UploadID != nil && *row.UploadID != "" {
				if err := s.objects.AbortMultipartUpload(ctx, row.ObjectKey, *row.UploadID); err != nil && !errors.Is(err, minio.ErrUploadNotFound) {
					s.logger.WarnContext(ctx, "中止过期分片会话失败", "task_id", row.ID, "error", err)
				}
			}
			if err := s.store.expireUploadingTask(ctx, row.ID, cutoff); err != nil {
				return err
			}
		}
		afterID = rows[len(rows)-1].ID
		if len(rows) < s.cleanup.BatchSize {
			return nil
		}
	}
}

func (s *Service) cleanupUnreferencedCompletedTasks(ctx context.Context, cutoff time.Time, checkRefs RawKeyRefChecker) error {
	var afterID int64
	for {
		rows, err := s.store.expiredTasksBatch(ctx, statusCompleted, cutoff, afterID, s.cleanup.BatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		keys := uniqueObjectKeys(rows)
		referenced, err := checkRefs(ctx, keys)
		if err != nil {
			return err
		}
		candidates := filterUnreferencedKeys(keys, referenced)
		active, err := s.store.activeCompletedKeys(ctx, candidates, cutoff)
		if err != nil {
			return err
		}
		for _, key := range candidates {
			if active[key] {
				continue
			}
			s.cleanCompletedObject(ctx, key, cutoff, checkRefs)
		}
		afterID = rows[len(rows)-1].ID
		if len(rows) < s.cleanup.BatchSize {
			return nil
		}
	}
}

func (s *Service) cleanCompletedObject(ctx context.Context, key string, cutoff time.Time, checkRefs RawKeyRefChecker) {
	var expiredRows int64
	err := s.store.inTransaction(ctx, func(txCtx context.Context) error {
		// 事务内复核无活跃任务且未被 video_part 引用，再将任务标记为过期。
		hasRecent, err := s.store.hasRecentCompletedByKeyForUpdate(txCtx, key, cutoff)
		if err != nil || hasRecent {
			return err
		}
		referenced, err := checkRefs(txCtx, []string{key})
		if err != nil || referenced[key] {
			return err
		}
		expiredRows, err = s.store.expireCompletedByKey(txCtx, key)
		return err
	})
	if err != nil {
		s.logger.WarnContext(ctx, "清理过期视频任务状态失败", "object_key", key, "error", err)
		return
	}
	if expiredRows == 0 {
		return
	}
	if err := s.objects.RemoveObject(ctx, key); err != nil && !errors.Is(err, minio.ErrObjectNotFound) {
		s.logger.WarnContext(ctx, "删除过期原始视频对象失败", "object_key", key, "error", err)
	}
}

func uniqueObjectKeys(rows []task) []string {
	keys := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.ObjectKey == "" || seen[row.ObjectKey] {
			continue
		}
		seen[row.ObjectKey] = true
		keys = append(keys, row.ObjectKey)
	}
	return keys
}

func filterUnreferencedKeys(keys []string, referenced map[string]bool) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if !referenced[key] {
			out = append(out, key)
		}
	}
	return out
}
