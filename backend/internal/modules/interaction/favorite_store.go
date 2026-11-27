package interaction

import (
	"context"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/tx"
)

const folderColumns = "id, user_id, title, is_default, is_public, item_count, created_at"

func (s *Store) folders(ctx context.Context, userID int64, publicOnly bool) ([]Folder, error) {
	query := "SELECT " + folderColumns + " FROM favorite_folder WHERE user_id = ?"
	if publicOnly {
		query += " AND is_public = 1"
	}
	rows := []Folder{}
	err := s.db.SelectContext(ctx, &rows, query+" ORDER BY is_default DESC, created_at DESC, id DESC", userID)
	return rows, err
}

func (s *Store) folder(ctx context.Context, id int64) (Folder, error) {
	var row Folder
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, "SELECT "+folderColumns+" FROM favorite_folder WHERE id = ?", id)
	return row, err
}

func (s *Store) defaultFolder(ctx context.Context, userID int64) (Folder, error) {
	var row Folder
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, "SELECT "+folderColumns+" FROM favorite_folder WHERE user_id = ? AND is_default = 1", userID)
	return row, err
}

func (s *Store) insertFolder(ctx context.Context, folder Folder) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "INSERT INTO favorite_folder (id,user_id,title,is_default,is_public,created_at) VALUES (?,?,?,?,?,?)",
		folder.ID, folder.UserID, folder.Title, folder.IsDefault, folder.IsPublic, folder.CreatedAt)
	return err
}

func (s *Store) updateFolder(ctx context.Context, folder Folder) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE favorite_folder SET title = ?, is_public = ? WHERE id = ?", folder.Title, folder.IsPublic, folder.ID)
	return err
}

func (s *Store) favoriteIDs(ctx context.Context, userID, videoID int64) ([]int64, error) {
	var ids []int64
	err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &ids, "SELECT folder_id FROM favorite_item WHERE user_id = ? AND video_id = ? ORDER BY folder_id", userID, videoID)
	return ids, err
}

func (s *Store) videosByFolder(ctx context.Context, folderID int64) ([]int64, error) {
	var ids []int64
	err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &ids, "SELECT video_id FROM favorite_item WHERE folder_id = ? ORDER BY created_at DESC, id DESC", folderID)
	return ids, err
}

func (s *Store) foldersByIDs(ctx context.Context, ids []int64) ([]Folder, error) {
	rows := []Folder{}
	if len(ids) == 0 {
		return rows, nil
	}
	query, args, err := sqlx.In("SELECT "+folderColumns+" FROM favorite_folder WHERE id IN (?)", ids)
	if err != nil {
		return nil, err
	}
	err = sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &rows, s.db.Rebind(query), args...)
	return rows, err
}

func (s *Store) addFavorite(ctx context.Context, id, userID, videoID, folderID int64) error {
	conn := tx.Executor(ctx, s.db)
	if _, err := conn.ExecContext(ctx, "INSERT INTO favorite_item (id,folder_id,user_id,video_id) VALUES (?,?,?,?)", id, folderID, userID, videoID); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, "UPDATE favorite_folder SET item_count = item_count + 1 WHERE id = ?", folderID)
	return err
}

func (s *Store) removeFavorite(ctx context.Context, videoID, folderID int64) error {
	conn := tx.Executor(ctx, s.db)
	if _, err := conn.ExecContext(ctx, "DELETE FROM favorite_item WHERE folder_id = ? AND video_id = ?", folderID, videoID); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, "UPDATE favorite_folder SET item_count = item_count - 1 WHERE id = ?", folderID)
	return err
}

func (s *Store) deleteFolder(ctx context.Context, id int64) error {
	conn := tx.Executor(ctx, s.db)
	if _, err := conn.ExecContext(ctx, "DELETE FROM favorite_item WHERE folder_id = ?", id); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, "DELETE FROM favorite_folder WHERE id = ?", id)
	return err
}

func (s *Store) stillFavorited(ctx context.Context, userID int64, ids []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(ids))
	for i := 0; i < len(ids); i += 500 {
		query, args, err := sqlx.In("SELECT DISTINCT video_id FROM favorite_item WHERE user_id = ? AND video_id IN (?)", userID, ids[i:min(i+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var existing []int64
		if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &existing, s.db.Rebind(query), args...); err != nil {
			return nil, err
		}
		for _, id := range existing {
			result[id] = true
		}
	}
	return result, nil
}
