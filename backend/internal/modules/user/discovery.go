package user

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/search"
	"simple_bilibili/backend/internal/pkg/tx"
)

const summaryColumns = "id, username, nickname, avatar_url"

type PublicSummary struct {
	ID        int64  `db:"id" json:"id,string"`
	Username  string `db:"username" json:"username"`
	Nickname  string `db:"nickname" json:"nickname"`
	AvatarURL string `db:"avatar_url" json:"avatar_url"`
}

type PublicProfile = publicProfileResponse

func (s *Service) PublicSummariesByIDs(ctx context.Context, ids []int64) (map[int64]PublicSummary, error) {
	result := make(map[int64]PublicSummary, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.store.summariesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func (s *Store) summariesByIDs(ctx context.Context, ids []int64) ([]PublicSummary, error) {
	query, args, err := sqlx.In("SELECT "+summaryColumns+" FROM user WHERE id IN (?)", ids)
	if err != nil {
		return nil, fmt.Errorf("构造公开资料批量查询: %w", err)
	}
	var rows []PublicSummary
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &rows, s.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("批量读取用户公开资料: %w", err)
	}
	return rows, nil
}

func (s *Service) SearchPublicUsers(ctx context.Context, keyword string, offset int64, limit int) ([]PublicProfile, int64, error) {
	return s.store.searchPublicUsers(ctx, search.ContainsPattern(keyword), offset, limit)
}

func (s *Store) searchPublicUsers(ctx context.Context, pattern string, offset int64, limit int) ([]PublicProfile, int64, error) {
	const where = " FROM user WHERE status = ? AND (nickname LIKE ? ESCAPE '!' OR username LIKE ? ESCAPE '!')"
	var total int64
	if err := s.db.GetContext(ctx, &total, "SELECT COUNT(*)"+where, statusActive, pattern, pattern); err != nil {
		return nil, 0, fmt.Errorf("统计匹配用户: %w", err)
	}
	rows := []PublicProfile{}
	if err := s.db.SelectContext(ctx, &rows, "SELECT "+publicProfileColumns+where+" ORDER BY id DESC LIMIT ? OFFSET ?", statusActive, pattern, pattern, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("搜索用户: %w", err)
	}
	return rows, total, nil
}
