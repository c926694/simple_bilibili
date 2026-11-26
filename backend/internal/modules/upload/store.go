package upload

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/tx"
)

const taskColumns = "id, upload_id, user_id, file_hash, file_name, file_size, chunk_size, total_chunks, object_key, status"

type Store struct {
	db *sqlx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

func (s *Store) inTransaction(ctx context.Context, run func(context.Context) error) error {
	return tx.WithTx(ctx, s.db, run)
}

func (s *Store) completedKeys(ctx context.Context, hash string, size int64) ([]string, error) {
	const query = `SELECT DISTINCT object_key FROM upload_task
		WHERE file_hash = ? AND file_size = ? AND status = ?`
	var keys []string
	if err := s.db.SelectContext(ctx, &keys, query, hash, size, statusCompleted); err != nil {
		return nil, fmt.Errorf("查询可复用视频: %w", err)
	}
	return keys, nil
}

func (s *Store) activeTask(ctx context.Context, userID int64, hash string, size int64) (task, error) {
	const query = "SELECT " + taskColumns + ` FROM upload_task
		WHERE user_id = ? AND file_hash = ? AND file_size = ? AND status = ?
		ORDER BY updated_at DESC, id DESC LIMIT 1`
	var record task
	if err := s.db.GetContext(ctx, &record, query, userID, hash, size, statusUploading); err != nil {
		return task{}, fmt.Errorf("查询未完成的上传任务: %w", err)
	}
	return record, nil
}

func (s *Store) insert(ctx context.Context, record task) error {
	const query = `INSERT INTO upload_task (id, upload_id, user_id, file_hash, file_name, file_size,
		chunk_size, total_chunks, object_key, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, record.ID, record.UploadID, record.UserID, record.FileHash, record.FileName,
		record.FileSize, record.ChunkSize, record.TotalChunks, record.ObjectKey, record.Status)
	if err != nil {
		return fmt.Errorf("写入上传任务: %w", err)
	}
	return nil
}

func (s *Store) byID(ctx context.Context, id int64, lock bool) (task, error) {
	query := "SELECT " + taskColumns + " FROM upload_task WHERE id = ?"
	if lock {
		query += " FOR UPDATE"
	}
	var record task
	if err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &record, query, id); err != nil {
		return task{}, fmt.Errorf("读取上传任务: %w", err)
	}
	return record, nil
}

func (s *Store) setStatus(ctx context.Context, id int64, status int) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx,
		"UPDATE upload_task SET status = ? WHERE id = ? AND status = ?", status, id, statusUploading)
	if err != nil {
		return fmt.Errorf("更新上传任务状态: %w", err)
	}
	return nil
}
