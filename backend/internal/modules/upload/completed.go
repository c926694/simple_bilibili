package upload

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/tx"
)

type CompletedTask struct {
	ObjectKey string `db:"object_key"`
	FileName  string `db:"file_name"`
	FileSize  int64  `db:"file_size"`
	FileHash  string `db:"file_hash"`
}

func (s *Service) GetCompletedTasks(ctx context.Context, userID int64, ids []int64) (map[int64]CompletedTask, error) {
	result := make(map[int64]CompletedTask, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.store.tasksByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID != userID {
			return nil, errcode.Forbidden
		}
		if row.Status != statusCompleted {
			return nil, errcode.InvalidUploadState
		}
		result[row.ID] = CompletedTask{row.ObjectKey, row.FileName, row.FileSize, row.FileHash}
	}
	for _, id := range ids {
		if _, ok := result[id]; !ok {
			return nil, errcode.UploadNotFound
		}
	}
	return result, nil
}

func (s *Store) tasksByIDs(ctx context.Context, ids []int64) ([]task, error) {
	query, args, err := sqlx.In("SELECT "+taskColumns+" FROM upload_task WHERE id IN (?)", ids)
	if err != nil {
		return nil, fmt.Errorf("构造上传任务批量查询: %w", err)
	}
	var rows []task
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &rows, s.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("批量读取上传任务: %w", err)
	}
	return rows, nil
}
