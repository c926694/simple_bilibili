package video

import (
	"context"
	"database/sql"
	"errors"

	"simple_bilibili/backend/internal/pkg/errcode"
)

type PublishedVideo struct {
	ID          int64
	BVID        string
	AuthorID    int64
	DurationSec int64
}

type Summary = videoItem

type PartSummary struct {
	ID          int64  `db:"id"`
	VideoID     int64  `db:"video_id"`
	PartIndex   int    `db:"part_index"`
	Title       string `db:"title"`
	DurationSec int    `db:"duration_sec"`
}

func (s *Service) PublishedVideo(ctx context.Context, bvid string) (PublishedVideo, error) {
	if !videoCodePattern.MatchString(bvid) {
		return PublishedVideo{}, errcode.VideoNotFound
	}
	row, err := s.store.byCode(ctx, bvid, false)
	if errors.Is(err, sql.ErrNoRows) {
		return PublishedVideo{}, errcode.VideoNotFound
	}
	if err != nil {
		return PublishedVideo{}, err
	}
	if row.Status != statusPublished {
		return PublishedVideo{}, errcode.VideoNotFound
	}
	return PublishedVideo{ID: row.ID, BVID: row.BVID, AuthorID: row.AuthorID, DurationSec: row.DurationSec}, nil
}

func (s *Service) SummariesByIDs(ctx context.Context, ids []int64) (map[int64]Summary, error) {
	result := make(map[int64]Summary, len(ids))
	for start := 0; start < len(ids); start += 500 {
		rows, err := s.store.listByIDs(ctx, ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		published := rows[:0]
		for _, row := range rows {
			if row.Status == statusPublished {
				published = append(published, row)
			}
		}
		// 作者、分区与统计继续按 ID 分步读取。
		items, err := s.buildItems(ctx, published, false)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			result[item.ID] = item
		}
	}
	return result, nil
}

func (s *Service) PartSummariesByIDs(ctx context.Context, ids []int64) (map[int64]PartSummary, error) {
	result := make(map[int64]PartSummary, len(ids))
	for start := 0; start < len(ids); start += 500 {
		var rows []PartSummary
		if err := s.store.selectByIDs(ctx, &rows, "SELECT id, video_id, part_index, title, duration_sec FROM video_part WHERE id IN (?)",
			ids[start:min(start+500, len(ids))]); err != nil {
			return nil, err
		}
		for _, part := range rows {
			result[part.ID] = part
		}
	}
	return result, nil
}
