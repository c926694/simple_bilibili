package video

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/pkg/tx"
)

func (s *Store) insertVideo(ctx context.Context, row video) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `INSERT INTO video
		(id,bv_id,author_id,category_id,title,description,cover_url,duration_sec,status,published_at,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, row.ID, row.BVID, row.AuthorID, row.CategoryID, row.Title, row.Description, row.CoverURL,
		row.DurationSec, row.Status, row.PublishedAt, row.CreatedAt)
	return err
}

func (s *Store) updateVideo(ctx context.Context, row video) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `UPDATE video SET category_id = ?, title = ?, description = ?,
		cover_url = ?, duration_sec = ?, status = ?, published_at = ? WHERE id = ?`, row.CategoryID, row.Title, row.Description,
		row.CoverURL, row.DurationSec, row.Status, row.PublishedAt, row.ID)
	return err
}

func (s *Store) updateVideoState(ctx context.Context, row video) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE video SET duration_sec = ?, status = ?, published_at = ? WHERE id = ?",
		row.DurationSec, row.Status, row.PublishedAt, row.ID)
	return err
}

func (s *Store) insertStat(ctx context.Context, id int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "INSERT INTO video_stat (video_id) VALUES (?)", id)
	return err
}

func (s *Store) insertPart(ctx context.Context, p part) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `INSERT INTO video_part
		(id,video_id,part_index,title,duration_sec,raw_object_key,hls_1080p_url,hls_720p_url,hls_480p_url,
		 transcode_status,transcode_job_id,transcode_attempt,transcode_dispatch_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.VideoID, p.Index, p.Title, p.Duration, p.RawKey,
		p.HLS1080, p.HLS720, p.HLS480, p.Status, p.JobID, p.Attempt, p.DispatchAt)
	return err
}

func (s *Store) reusablePart(ctx context.Context, key string) (part, error) {
	var row part
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row,
		"SELECT "+partColumns+" FROM video_part WHERE raw_object_key = ? AND transcode_status = ? ORDER BY id LIMIT 1", key, partSucceeded)
	return row, err
}

func (s *Store) partByID(ctx context.Context, videoID, partID int64) (part, error) {
	var row part
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &row, "SELECT "+partColumns+" FROM video_part WHERE video_id = ? AND id = ?", videoID, partID)
	return row, err
}

func (s *Store) movePartIndexes(ctx context.Context, id int64) error {
	// 先移入负数区间，避免调换两个选集时触发序号唯一约束。
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE video_part SET part_index = -part_index WHERE video_id = ?", id)
	return err
}

func (s *Store) updatePartPosition(ctx context.Context, p part) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE video_part SET part_index = ?, title = ? WHERE id = ?", p.Index, p.Title, p.ID)
	return err
}

func (s *Store) removeUnusedParts(ctx context.Context, id int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "DELETE FROM video_part WHERE video_id = ? AND part_index < 0", id)
	return err
}

func (s *Store) clearTags(ctx context.Context, id int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "DELETE FROM video_tag WHERE video_id = ?", id)
	return err
}

func (s *Store) ensureTag(ctx context.Context, id int64, name string) (int64, error) {
	conn := tx.Executor(ctx, s.db)
	_, err := conn.ExecContext(ctx, "INSERT INTO tag (id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = tag.name", id, name)
	if err != nil {
		return 0, err
	}
	if err := sqlx.GetContext(ctx, conn, &id, "SELECT id FROM tag WHERE name = ?", name); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) linkTag(ctx context.Context, linkID, videoID, tagID int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "INSERT INTO video_tag (id, video_id, tag_id) VALUES (?, ?, ?)", linkID, videoID, tagID)
	return err
}

func (s *Store) schedulePart(ctx context.Context, p part) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `UPDATE video_part SET transcode_status = ?, transcode_job_id = ?,
		transcode_attempt = ?, transcode_dispatch_at = ?, transcode_lease_until = NULL WHERE id = ?`,
		partWaiting, p.JobID, p.Attempt, p.DispatchAt, p.ID)
	return err
}

func (s *Store) abandonDispatch(ctx context.Context, id int64) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE video_part SET transcode_dispatch_at = NULL, transcode_lease_until = NULL WHERE video_id = ?", id)
	return err
}

func (s *Store) setAutomaticCover(ctx context.Context, id int64, url string) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE video SET cover_url = ? WHERE id = ? AND cover_url = '' AND status <> ?", url, id, statusWithdrawn)
	if err != nil {
		return fmt.Errorf("回写默认封面: %w", err)
	}
	return nil
}

func summarize(row *video, parts []part, now time.Time) {
	var failed, pending bool
	row.DurationSec = 0
	for _, p := range parts {
		row.DurationSec += int64(p.Duration)
		failed = failed || p.Status == partFailed
		pending = pending || p.Status == partRunning || p.Status == partWaiting
	}
	switch {
	case failed:
		row.Status = statusFailed
	case pending:
		row.Status = statusTranscoding
	default:
		row.Status = statusPublished
		if row.PublishedAt == nil {
			row.PublishedAt = &now
		}
	}
}
