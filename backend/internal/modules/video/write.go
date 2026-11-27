package video

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/media"
)

func (s *Service) Create(ctx context.Context, authorID int64, input draftInput) (videoDetail, error) {
	now := time.Now().UTC().Truncate(time.Second)
	id := s.ids.Next()
	row := video{ID: id, BVID: videoCode(id), AuthorID: authorID, CreatedAt: now, Status: statusTranscoding}
	var parts []part
	var promotedCoverKey string
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 先核验关联对象，再写入稿件、分 P 和统计。
		var err error
		promotedCoverKey, err = s.applyMetadata(ctx, &row, input)
		if err != nil {
			return err
		}
		parts, err = s.prepareParts(ctx, row, *input.Parts, nil)
		if err != nil {
			return err
		}
		summarize(&row, parts, now)
		if err := s.store.insertVideo(ctx, row); err != nil {
			return err
		}
		for _, p := range parts {
			if err := s.store.insertPart(ctx, p); err != nil {
				return err
			}
		}
		if err := s.store.insertStat(ctx, row.ID); err != nil {
			return err
		}
		// 标签和稿件同时提交，关系通过标签 ID 保存。
		if input.Tags != nil {
			return s.replaceTags(ctx, row.ID, *input.Tags)
		}
		return nil
	})
	if err != nil {
		if promotedCoverKey != "" {
			s.removeCoverQuietly(ctx, promotedCoverKey, "回滚删除已转正封面失败")
		}
		return videoDetail{}, err
	}
	// 数据库已保存待投递的分 P，提交后再发送消息。
	return s.finishWrite(ctx, row, parts)
}

func (s *Service) Edit(ctx context.Context, authorID int64, code string, input draftInput) (videoDetail, error) {
	var row video
	var parts []part
	var oldCover, promotedCoverKey string
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 编辑与转码状态更新使用同一稿件行锁。
		var err error
		row, err = s.ownedVideo(ctx, authorID, code)
		if err != nil {
			return err
		}
		if row.Status == statusWithdrawn {
			return errcode.InvalidVideoState
		}
		oldCover = row.CoverURL
		// 更新提交的字段并核验新的分区、封面。
		promotedCoverKey, err = s.applyMetadata(ctx, &row, input)
		if err != nil {
			return err
		}
		if input.Parts != nil {
			// 保留现有分 P 的转码结果，读取追加分 P 的上传任务。
			parts, err = s.replaceParts(ctx, row, *input.Parts)
			if err != nil {
				return err
			}
			summarize(&row, parts, time.Now().UTC())
		}
		if err := s.store.updateVideo(ctx, row); err != nil {
			return err
		}
		if input.Tags != nil {
			// 标签字段存在时按提交的列表更新关联。
			return s.replaceTags(ctx, row.ID, *input.Tags)
		}
		return nil
	})
	if err != nil {
		if promotedCoverKey != "" {
			s.removeCoverQuietly(ctx, promotedCoverKey, "回滚删除已转正封面失败")
		}
		return videoDetail{}, err
	}
	if oldCover != "" && oldCover != row.CoverURL {
		if oldKey, ok := media.PublicImageKey("covers", oldCover); ok {
			if refs, refErr := s.store.countCoverRefs(ctx, oldCover); refErr == nil && refs == 0 {
				s.removeCoverQuietly(ctx, oldKey, "删除旧封面失败")
			}
		}
	}
	// 完成数据库更新后失效公开详情，并发送新增分 P 的任务。
	return s.finishWrite(ctx, row, parts)
}

func (s *Service) Withdraw(ctx context.Context, authorID int64, code string) error {
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 按稿件归属加锁，提交后后台任务也不能改变下架状态。
		row, err := s.ownedVideo(ctx, authorID, code)
		if err != nil {
			return err
		}
		if row.Status == statusWithdrawn {
			return nil
		}
		row.Status = statusWithdrawn
		if err := s.store.updateVideoState(ctx, row); err != nil {
			return err
		}
		return s.store.abandonDispatch(ctx, row.ID)
	})
	if err != nil {
		return err
	}
	return s.invalidate(ctx, code)
}

func (s *Service) Retry(ctx context.Context, authorID int64, code string, partID int64) (videoDetail, error) {
	var row video
	var p part
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 仅作者可以重新安排本稿件中已失败的分 P。
		var err error
		row, err = s.ownedVideo(ctx, authorID, code)
		if err != nil {
			return err
		}
		p, err = s.store.partByID(ctx, row.ID, partID)
		if errors.Is(err, sql.ErrNoRows) {
			return errcode.VideoPartNotFound
		}
		if err != nil {
			return err
		}
		if row.Status == statusWithdrawn || p.Status != partFailed {
			return errcode.InvalidVideoState
		}
		now := time.Now().UTC()
		p.Status, p.Attempt, p.JobID, p.DispatchAt = partWaiting, 0, s.ids.Next(), &now
		if err := s.store.schedulePart(ctx, p); err != nil {
			return err
		}
		row.Status = statusTranscoding
		return s.store.updateVideoState(ctx, row)
	})
	if err != nil {
		return videoDetail{}, err
	}
	// 新消息使用新的任务标识，已过期的消息不会领取分 P。
	return s.finishWrite(ctx, row, []part{p})
}

func (s *Service) finishWrite(ctx context.Context, row video, parts []part) (videoDetail, error) {
	if err := s.invalidate(ctx, row.BVID); err != nil {
		return videoDetail{}, err
	}
	for _, p := range parts {
		if p.Status != partWaiting || p.DispatchAt == nil {
			continue
		}
		if err := s.dispatch(ctx, row, p); err != nil {
			// 投递记录仍在数据库，后台会再次投递。
			s.logger.WarnContext(ctx, "投递稿件转码任务", "part_id", p.ID, "error", err)
		}
	}
	return s.buildDetail(ctx, row)
}

func (s *Service) ownedVideo(ctx context.Context, authorID int64, code string) (video, error) {
	row, err := s.store.byCode(ctx, code, true)
	if errors.Is(err, sql.ErrNoRows) {
		return video{}, errcode.VideoNotFound
	}
	if err != nil {
		return video{}, err
	}
	if row.AuthorID != authorID {
		return video{}, errcode.Forbidden
	}
	return row, nil
}

func (s *Service) applyMetadata(ctx context.Context, row *video, input draftInput) (string, error) {
	if input.Title != nil {
		row.Title = *input.Title
	}
	if input.Description != nil {
		row.Description = *input.Description
	}
	if input.CategoryID != nil {
		enabled, err := s.store.categoryEnabled(ctx, *input.CategoryID)
		if err != nil {
			return "", err
		}
		if !enabled {
			return "", errcode.CategoryNotFound
		}
		row.CategoryID = input.CategoryID
	}
	var promotedKey string
	if input.CoverURL != nil && *input.CoverURL != row.CoverURL {
		nextCover := *input.CoverURL
		if nextCover != "" {
			if strings.HasPrefix(nextCover, "/storage/tmp/covers/") {
				promoted, err := media.PromoteOwnedImage(ctx, s.objects, "covers", row.AuthorID, nextCover)
				if errors.Is(err, media.ErrInvalidImage) {
					return "", errcode.InvalidVideoCover
				}
				if err != nil {
					return "", err
				}
				nextCover = promoted
				if key, ok := media.PublicImageKey("covers", promoted); ok {
					promotedKey = key
				}
			} else {
				err := media.VerifyOwnedImage(ctx, s.objects, "covers", row.AuthorID, nextCover)
				if errors.Is(err, media.ErrInvalidImage) {
					return "", errcode.InvalidVideoCover
				}
				if err != nil {
					return "", err
				}
			}
		}
		row.CoverURL = nextCover
	}
	return promotedKey, nil
}

func (s *Service) removeCoverQuietly(ctx context.Context, key, msg string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := s.objects.RemoveObject(cleanupCtx, key); err != nil && !errors.Is(err, minio.ErrObjectNotFound) {
		s.logger.WarnContext(ctx, msg, "object_key", key, "error", err)
	}
}

func (s *Service) prepareParts(ctx context.Context, row video, inputs []partInput, old map[int64]part) ([]part, error) {
	var ids []int64
	for _, item := range inputs {
		if item.UploadTaskID != 0 {
			ids = append(ids, item.UploadTaskID)
		}
	}
	uploads, err := s.uploads.GetCompletedTasks(ctx, row.AuthorID, ids)
	if err != nil {
		return nil, err
	}
	parts := make([]part, 0, len(inputs))
	now := time.Now().UTC()
	for i, item := range inputs {
		p := part{ID: s.ids.Next(), VideoID: row.ID}
		if item.ID != 0 {
			var found bool
			p, found = old[item.ID]
			if !found {
				return nil, errcode.VideoPartNotFound
			}
		} else {
			p.RawKey = uploads[item.UploadTaskID].ObjectKey
			reused, err := s.store.reusablePart(ctx, p.RawKey)
			if err == nil {
				p.Status, p.Duration, p.HLS1080, p.HLS720, p.HLS480 = partSucceeded, reused.Duration, reused.HLS1080, reused.HLS720, reused.HLS480
			} else if errors.Is(err, sql.ErrNoRows) {
				p.JobID, p.DispatchAt = s.ids.Next(), &now
			} else {
				return nil, err
			}
		}
		p.Index, p.Title = i+1, item.Title
		parts = append(parts, p)
	}
	return parts, nil
}

func (s *Service) replaceParts(ctx context.Context, row video, inputs []partInput) ([]part, error) {
	oldParts, err := s.store.partsByVideoIDs(ctx, []int64{row.ID})
	if err != nil {
		return nil, err
	}
	old := make(map[int64]part, len(oldParts))
	for _, p := range oldParts {
		old[p.ID] = p
	}
	parts, err := s.prepareParts(ctx, row, inputs, old)
	if err != nil {
		return nil, err
	}
	if err := s.store.movePartIndexes(ctx, row.ID); err != nil {
		return nil, err
	}
	for _, p := range parts {
		if _, exists := old[p.ID]; exists {
			if err := s.store.updatePartPosition(ctx, p); err != nil {
				return nil, err
			}
		} else if err := s.store.insertPart(ctx, p); err != nil {
			return nil, err
		}
	}
	if err := s.store.removeUnusedParts(ctx, row.ID); err != nil {
		return nil, err
	}
	return parts, nil
}

func (s *Service) replaceTags(ctx context.Context, videoID int64, tags []string) error {
	if err := s.store.clearTags(ctx, videoID); err != nil {
		return err
	}
	seen := make(map[int64]bool, len(tags))
	for _, tag := range tags {
		id, err := s.store.ensureTag(ctx, s.ids.Next(), tag)
		if err != nil {
			return fmt.Errorf("保存稿件标签: %w", err)
		}
		if seen[id] {
			continue
		}
		if err := s.store.linkTag(ctx, s.ids.Next(), videoID, id); err != nil {
			return err
		}
		seen[id] = true
	}
	return nil
}

func videoCode(id int64) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var buf [11]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = alphabet[id%62]
		id /= 62
	}
	return "BV" + string(buf[i:])
}
