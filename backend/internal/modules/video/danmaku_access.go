package video

import (
	"context"
	"database/sql"
	"errors"

	"simple_bilibili/backend/internal/pkg/errcode"
)

type PublishedPart struct {
	VideoID     int64
	PartID      int64
	DurationSec int
}

func (s *Service) PublishedVideoID(ctx context.Context, bvid string) (int64, error) {
	if !videoCodePattern.MatchString(bvid) {
		return 0, errcode.VideoNotFound
	}
	row, err := s.store.byCode(ctx, bvid, false)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errcode.VideoNotFound
	}
	if err != nil {
		return 0, err
	}
	if row.Status != statusPublished {
		return 0, errcode.VideoNotFound
	}
	return row.ID, nil
}

func (s *Service) PublishedVideoPart(ctx context.Context, bvid string, partID int64) (PublishedPart, error) {
	videoID, err := s.PublishedVideoID(ctx, bvid)
	if err != nil {
		return PublishedPart{}, err
	}
	row, err := s.store.partByID(ctx, videoID, partID)
	if errors.Is(err, sql.ErrNoRows) {
		return PublishedPart{}, errcode.VideoPartNotFound
	}
	if err != nil {
		return PublishedPart{}, err
	}
	return PublishedPart{VideoID: videoID, PartID: row.ID, DurationSec: row.Duration}, nil
}
