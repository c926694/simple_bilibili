package video

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/singleflight"

	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/modules/upload"
	"simple_bilibili/backend/internal/modules/user"
	"simple_bilibili/backend/internal/pkg/snowflake"
)

type Service struct {
	store       *Store
	users       *user.Service
	uploads     *upload.Service
	objects     *minio.Client
	publisher   *rocketmq.Producer
	ids         *snowflake.Generator
	logger      *slog.Logger
	detailLoads singleflight.Group
}

func NewService(store *Store, users *user.Service, uploads *upload.Service, objects *minio.Client, publisher *rocketmq.Producer, ids *snowflake.Generator, logger *slog.Logger) *Service {
	return &Service{store: store, users: users, uploads: uploads, objects: objects, publisher: publisher, ids: ids, logger: logger}
}

func (s *Service) DetailsByIDs(ctx context.Context, ids []int64) ([]Detail, error) {
	videos, err := s.store.videosByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	// 同一作者可能有多个稿件，合并作者 ID 后再批量读取昵称。
	authorIDs := uniqueAuthorIDs(videos)
	nicknames, err := s.users.NicknamesByIDs(ctx, authorIDs)
	if err != nil {
		return nil, fmt.Errorf("读取视频作者昵称: %w", err)
	}
	// 根据作者 ID 组装视频详情，并报告缺失的作者记录。
	return buildDetails(videos, nicknames)
}

func uniqueAuthorIDs(videos []video) []int64 {
	authorIDs := make([]int64, 0, len(videos))
	seen := make(map[int64]struct{}, len(videos))
	for _, row := range videos {
		if _, exists := seen[row.AuthorID]; !exists {
			seen[row.AuthorID] = struct{}{}
			authorIDs = append(authorIDs, row.AuthorID)
		}
	}
	return authorIDs
}

func buildDetails(videos []video, nicknames map[int64]string) ([]Detail, error) {
	details := make([]Detail, 0, len(videos))
	for _, row := range videos {
		nickname, exists := nicknames[row.AuthorID]
		if !exists {
			return nil, fmt.Errorf("视频 %d 的作者 %d 不存在", row.ID, row.AuthorID)
		}
		details = append(details, Detail{ID: row.ID, AuthorID: row.AuthorID, AuthorNickname: nickname, Title: row.Title})
	}
	return details, nil
}

func (s *Service) PublishedIDs(ctx context.Context, ids []int64) ([]int64, error) {
	rows, err := s.store.videosByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	published := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.Status == statusPublished {
			published = append(published, row.ID)
		}
	}
	return published, nil
}

func (s *Service) AuthorsByIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	rows, err := s.store.videosByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	authors := make(map[int64]int64, len(rows))
	for _, row := range rows {
		authors[row.ID] = row.AuthorID
	}
	return authors, nil
}

func (s *Service) TitlesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := s.store.videosByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	titles := make(map[int64]string, len(rows))
	for _, row := range rows {
		titles[row.ID] = row.Title
	}
	return titles, nil
}

func (s *Service) ReferencedRawKeys(ctx context.Context, keys []string) (map[string]bool, error) {
	return s.store.referencedRawKeys(ctx, keys)
}
