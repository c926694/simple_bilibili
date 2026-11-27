package interaction

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/pkg/errcode"
)

func (s *Service) Folders(ctx context.Context, viewer, owner int64) ([]Folder, error) {
	if owner == 0 {
		owner = viewer
	}
	if owner == 0 {
		return nil, errcode.Unauthorized
	}
	if viewer == owner {
		if err := s.userTx(ctx, owner, func(ctx context.Context) error {
			// 本人的收藏夹列表始终包含一个默认夹。
			_, err := s.ensureDefault(ctx, owner)
			return err
		}); err != nil {
			return nil, err
		}
	}
	return s.store.folders(ctx, owner, viewer != owner)
}

func (s *Service) CreateFolder(ctx context.Context, userID int64, input folderInput) (Folder, error) {
	row := Folder{ID: s.ids.Next(), UserID: userID, Title: input.Title, IsPublic: input.IsPublic, CreatedAt: time.Now().UTC().Truncate(time.Second)}
	return row, s.store.insertFolder(ctx, row)
}

func (s *Service) EditFolder(ctx context.Context, userID, id int64, input folderInput) (Folder, error) {
	var row Folder
	err := s.userTx(ctx, userID, func(ctx context.Context) error {
		var err error
		// 在用户锁内确认收藏夹仍归当前用户。
		row, err = s.ownedFolder(ctx, userID, id)
		if err != nil {
			return err
		}
		row.Title, row.IsPublic = input.Title, input.IsPublic
		return s.store.updateFolder(ctx, row)
	})
	return row, err
}

func (s *Service) DeleteFolder(ctx context.Context, userID, id int64) error {
	return s.userTx(ctx, userID, func(ctx context.Context) error {
		// 默认夹由三连使用，始终保留。
		folder, err := s.ownedFolder(ctx, userID, id)
		if err != nil {
			return err
		}
		if folder.IsDefault {
			return errcode.DefaultFolderProtected
		}
		videos, err := s.store.videosByFolder(ctx, id)
		if err != nil {
			return err
		}
		if err := s.store.deleteFolder(ctx, id); err != nil {
			return err
		}
		// 只统计已从该用户所有收藏夹中删除的视频。
		left, err := s.store.stillFavorited(ctx, userID, videos)
		if err != nil {
			return err
		}
		for _, videoID := range videos {
			if !left[videoID] {
				if err := s.delta(ctx, videoID, 0, 0, -1, 0); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Service) FolderVideos(ctx context.Context, viewer, id int64, p page) (folderVideos, error) {
	folder, err := s.store.folder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return folderVideos{}, errcode.FavoriteFolderNotFound
	}
	if err != nil {
		return folderVideos{}, err
	}
	if !folder.IsPublic && folder.UserID != viewer {
		return folderVideos{}, errcode.Forbidden
	}
	ids, err := s.store.videosByFolder(ctx, id)
	if err != nil {
		return folderVideos{}, err
	}
	// 保留收藏时间顺序，并排除已不公开的视频。
	ids, err = s.publishedInOrder(ctx, ids)
	if err != nil {
		return folderVideos{}, err
	}
	start, end := p.bounds(len(ids))
	idsPage := ids[start:end]
	items, err := s.videos.SummariesByIDs(ctx, idsPage)
	if err != nil {
		return folderVideos{}, err
	}
	rows := make([]video.Summary, 0, len(idsPage))
	for _, id := range idsPage {
		if row, ok := items[id]; ok {
			rows = append(rows, row)
		}
	}
	return folderVideos{Folder: folder, Items: rows, Total: len(ids), page: p}, nil
}

func (s *Service) publishedInOrder(ctx context.Context, ids []int64) ([]int64, error) {
	result := make([]int64, 0, len(ids))
	for i := 0; i < len(ids); i += 500 {
		chunk := ids[i:min(i+500, len(ids))]
		found, err := s.videos.PublishedIDs(ctx, chunk)
		if err != nil {
			return nil, err
		}
		visible := make(map[int64]bool, len(found))
		for _, id := range found {
			visible[id] = true
		}
		for _, id := range chunk {
			if visible[id] {
				result = append(result, id)
			}
		}
	}
	return result, nil
}

func (s *Service) SetFavorites(ctx context.Context, userID int64, bvid string, target []int64) (favoriteResponse, error) {
	published, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return favoriteResponse{}, err
	}
	var result favoriteResponse
	err = s.userTx(ctx, userID, func(ctx context.Context) error {
		folders, err := s.store.foldersByIDs(ctx, target)
		if err != nil {
			return err
		}
		if len(folders) != len(target) {
			return errcode.FavoriteFolderNotFound
		}
		for _, folder := range folders {
			if folder.UserID != userID {
				return errcode.Forbidden
			}
		}
		// 一次提交该用户对本视频的收藏夹差异及计数。
		result, err = s.replaceFavorites(ctx, userID, published.ID, target)
		if err != nil {
			return err
		}
		return s.delta(ctx, published.ID, 0, 0, result.Delta, 0)
	})
	return result, err
}

func (s *Service) replaceFavorites(ctx context.Context, userID, videoID int64, target []int64) (favoriteResponse, error) {
	current, err := s.store.favoriteIDs(ctx, userID, videoID)
	if err != nil {
		return favoriteResponse{}, err
	}
	for _, id := range current {
		if !slices.Contains(target, id) {
			if err := s.store.removeFavorite(ctx, videoID, id); err != nil {
				return favoriteResponse{}, err
			}
		}
	}
	for _, id := range target {
		if !slices.Contains(current, id) {
			if err := s.store.addFavorite(ctx, s.ids.Next(), userID, videoID, id); err != nil {
				return favoriteResponse{}, err
			}
		}
	}
	result := favoriteResponse{Favorited: len(target) > 0, FolderIDs: stringIDs(target)}
	if len(current) == 0 && len(target) > 0 {
		result.Delta = 1
	}
	if len(current) > 0 && len(target) == 0 {
		result.Delta = -1
	}
	return result, nil
}

func (s *Service) favoriteForTriple(ctx context.Context, userID, videoID int64) (favoriteResponse, error) {
	folders, err := s.store.favoriteIDs(ctx, userID, videoID)
	if err != nil {
		return favoriteResponse{}, err
	}
	if len(folders) > 0 {
		return favoriteResponse{Favorited: true, FolderIDs: stringIDs(folders)}, nil
	}
	folder, err := s.ensureDefault(ctx, userID)
	if err != nil {
		return favoriteResponse{}, err
	}
	if err := s.store.addFavorite(ctx, s.ids.Next(), userID, videoID, folder.ID); err != nil {
		return favoriteResponse{}, err
	}
	return favoriteResponse{Favorited: true, FolderIDs: stringIDs([]int64{folder.ID}), Delta: 1}, nil
}

func (s *Service) ensureDefault(ctx context.Context, userID int64) (Folder, error) {
	folder, err := s.store.defaultFolder(ctx, userID)
	if !errors.Is(err, sql.ErrNoRows) {
		return folder, err
	}
	folder = Folder{ID: s.ids.Next(), UserID: userID, Title: "默认收藏夹", IsDefault: true, CreatedAt: time.Now().UTC().Truncate(time.Second)}
	return folder, s.store.insertFolder(ctx, folder)
}

func (s *Service) ownedFolder(ctx context.Context, userID, id int64) (Folder, error) {
	row, err := s.store.folder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, errcode.FavoriteFolderNotFound
	}
	if err != nil {
		return Folder{}, err
	}
	if row.UserID != userID {
		return Folder{}, errcode.Forbidden
	}
	return row, nil
}
