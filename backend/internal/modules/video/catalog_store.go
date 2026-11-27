package video

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/search"
	"simple_bilibili/backend/internal/pkg/tx"
)

const videoColumns = "id, bv_id, author_id, category_id, title, COALESCE(description, '') AS description, cover_url, duration_sec, status, published_at, created_at"
const statColumns = "video_id, play_count, danmaku_count, like_count, coin_count, favorite_count, share_count, comment_count, hot_score"
const partColumns = "id, video_id, part_index, title, duration_sec, raw_object_key, hls_1080p_url, hls_720p_url, hls_480p_url, transcode_status, transcode_job_id, transcode_attempt, transcode_dispatch_at, transcode_lease_until"

func (s *Store) inTransaction(ctx context.Context, run func(context.Context) error) error {
	return tx.WithTx(ctx, s.db, run)
}

func (s *Store) categories(ctx context.Context) ([]categoryListing, error) {
	rows := []categoryListing{}
	err := s.db.SelectContext(ctx, &rows, "SELECT id, name, sort_order FROM category WHERE status = 1 ORDER BY sort_order, id")
	return rows, err
}

func (s *Store) categoryEnabled(ctx context.Context, id int32) (bool, error) {
	var enabled bool
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &enabled, "SELECT EXISTS(SELECT 1 FROM category WHERE id = ? AND status = 1)", id)
	return enabled, err
}

func (s *Store) byCode(ctx context.Context, code string, lock bool) (video, error) {
	query := "SELECT " + videoColumns + " FROM video WHERE bv_id = ?"
	if lock {
		query += " FOR UPDATE"
	}
	var row video
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, query, code)
	return row, err
}

func (s *Store) byID(ctx context.Context, id int64, lock bool) (video, error) {
	query := "SELECT " + videoColumns + " FROM video WHERE id = ?"
	if lock {
		query += " FOR UPDATE"
	}
	var row video
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, query, id)
	return row, err
}

func filterSQL(f listFilter) (string, []any) {
	where := " FROM video WHERE "
	var args []any
	switch {
	case !f.Mine:
		where += "status = ?"
		args = append(args, statusPublished)
	case f.Status != nil:
		where += "status = ?"
		args = append(args, *f.Status)
	default:
		where += "status <> ?"
		args = append(args, statusWithdrawn)
	}
	if f.AuthorID != 0 {
		where += " AND author_id = ?"
		args = append(args, f.AuthorID)
	}
	if f.CategoryID != 0 {
		where += " AND category_id = ?"
		args = append(args, f.CategoryID)
	}
	if f.Keyword != "" {
		where += " AND title LIKE ? ESCAPE '!'"
		args = append(args, search.ContainsPattern(f.Keyword))
	}
	return where, args
}

func (s *Store) latest(ctx context.Context, f listFilter) ([]video, int64, error) {
	where, args := filterSQL(f)
	var total int64
	if err := s.db.GetContext(ctx, &total, "SELECT COUNT(*)"+where, args...); err != nil {
		return nil, 0, fmt.Errorf("统计视频列表: %w", err)
	}
	order := "published_at DESC, id DESC"
	if f.Mine {
		order = "created_at DESC, id DESC"
	}
	rows := []video{}
	if err := s.db.SelectContext(ctx, &rows, "SELECT "+videoColumns+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", append(args, f.Size, f.offset())...); err != nil {
		return nil, 0, fmt.Errorf("读取视频列表: %w", err)
	}
	return rows, total, nil
}

func (s *Store) matchingIDs(ctx context.Context, f listFilter) ([]int64, error) {
	where, args := filterSQL(f)
	var ids []int64
	err := s.db.SelectContext(ctx, &ids, "SELECT id"+where, args...)
	return ids, err
}

func (s *Store) listByIDs(ctx context.Context, ids []int64) ([]video, error) {
	rows := []video{}
	err := s.selectByIDs(ctx, &rows, "SELECT "+videoColumns+" FROM video WHERE id IN (?)", ids)
	return rows, err
}

func (s *Store) categoriesByIDs(ctx context.Context, ids []int64) ([]Category, error) {
	rows := []Category{}
	err := s.selectByIDs(ctx, &rows, "SELECT id, name FROM category WHERE id IN (?)", ids)
	return rows, err
}

func (s *Store) statsByIDs(ctx context.Context, ids []int64) ([]statistic, error) {
	rows := []statistic{}
	err := s.selectByIDs(ctx, &rows, "SELECT "+statColumns+" FROM video_stat WHERE video_id IN (?)", ids)
	return rows, err
}

func (s *Store) partsByVideoIDs(ctx context.Context, ids []int64) ([]part, error) {
	rows := []part{}
	err := s.selectByIDs(ctx, &rows, "SELECT "+partColumns+" FROM video_part WHERE video_id IN (?) ORDER BY video_id, part_index", ids)
	return rows, err
}

func (s *Store) tagsByVideoIDs(ctx context.Context, videoIDs []int64) (map[int64][]string, error) {
	var links []struct {
		VideoID int64 `db:"video_id"`
		TagID   int64 `db:"tag_id"`
	}
	if err := s.selectByIDs(ctx, &links, "SELECT video_id, tag_id FROM video_tag WHERE video_id IN (?) ORDER BY video_id, id", videoIDs); err != nil {
		return nil, err
	}
	var tagIDs []int64
	for _, link := range links {
		tagIDs = append(tagIDs, link.TagID)
	}
	var rows []struct {
		ID   int64
		Name string
	}
	if err := s.selectByIDs(ctx, &rows, "SELECT id, name FROM tag WHERE id IN (?)", tagIDs); err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	result := make(map[int64][]string, len(videoIDs))
	for _, id := range videoIDs {
		result[id] = []string{}
	}
	for _, link := range links {
		if name, exists := names[link.TagID]; exists {
			result[link.VideoID] = append(result[link.VideoID], name)
		}
	}
	return result, nil
}

func (s *Store) selectByIDs(ctx context.Context, dst any, query string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(query, ids)
	if err != nil {
		return fmt.Errorf("构造视频模块批量查询: %w", err)
	}
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), dst, s.db.Rebind(query), args...); err != nil {
		return fmt.Errorf("执行视频模块批量查询: %w", err)
	}
	return nil
}
