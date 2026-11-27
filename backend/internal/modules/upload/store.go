package upload

import (
	"context"
	"fmt"
	"time"

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

func (s *Store) expiredTasksBatch(ctx context.Context, status int, cutoff time.Time, afterID int64, limit int) ([]task, error) {
	const query = "SELECT " + taskColumns + ` FROM upload_task
		WHERE status = ? AND updated_at < ? AND id > ?
		ORDER BY id ASC LIMIT ?`
	var rows []task
	if err := s.db.SelectContext(ctx, &rows, query, status, cutoff, afterID, limit); err != nil {
		return nil, fmt.Errorf("分批查询过期上传任务: %w", err)
	}
	return rows, nil
}

func (s *Store) expireUploadingTask(ctx context.Context, id int64, cutoff time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE upload_task SET status = ? WHERE id = ? AND status = ? AND updated_at < ?",
		statusCanceled, id, statusUploading, cutoff)
	if err != nil {
		return fmt.Errorf("标记过期上传中任务: %w", err)
	}
	return nil
}

func (s *Store) activeCompletedKeys(ctx context.Context, keys []string, cutoff time.Time) (map[string]bool, error) {
	active := make(map[string]bool)
	if len(keys) == 0 {
		return active, nil
	}
	query, args, err := sqlx.In(
		"SELECT DISTINCT object_key FROM upload_task WHERE object_key IN (?) AND status = ? AND updated_at >= ?",
		keys, statusCompleted, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("构造活跃对象键查询: %w", err)
	}
	var found []string
	if err := s.db.SelectContext(ctx, &found, s.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("查询近期活跃对象键: %w", err)
	}
	for _, key := range found {
		active[key] = true
	}
	return active, nil
}

func (s *Store) hasRecentCompletedByKeyForUpdate(ctx context.Context, key string, cutoff time.Time) (bool, error) {
	// 锁定该对象键下所有已完成任务，与投稿时的 FOR SHARE 互斥。
	const query = `SELECT updated_at FROM upload_task
		WHERE object_key = ? AND status = ? FOR UPDATE`
	var times []time.Time
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &times, query, key, statusCompleted); err != nil {
		return true, fmt.Errorf("锁定并检查已完成对象任务: %w", err)
	}
	if len(times) == 0 {
		return true, nil
	}
	for _, updatedAt := range times {
		if !updatedAt.Before(cutoff) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) expireCompletedByKey(ctx context.Context, key string) (int64, error) {
	res, err := tx.Executor(ctx, s.db).ExecContext(ctx,
		"UPDATE upload_task SET status = ? WHERE object_key = ? AND status = ?",
		statusCanceled, key, statusCompleted)
	if err != nil {
		return 0, fmt.Errorf("标记过期已完成任务: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取受影响行数: %w", err)
	}
	return affected, nil
}
