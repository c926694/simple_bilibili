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
	PartIndex   int
	Title       string
}

func (s *Service) PublishedVideoID(ctx context.Context, bvid string) (int64, error) {
	row, err := s.PublishedVideo(ctx, bvid)
	return row.ID, err
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
	return PublishedPart{VideoID: videoID, PartID: row.ID, DurationSec: row.Duration, PartIndex: row.Index, Title: row.Title}, nil
}
