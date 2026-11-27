package video

import (
	"context"
	"time"

	"simple_bilibili/backend/internal/pkg/tx"
)

func (s *Store) dueParts(ctx context.Context, now time.Time) ([]part, error) {
	var rows []part
	err := s.db.SelectContext(ctx, &rows, "SELECT "+partColumns+` FROM video_part
		WHERE (transcode_status = ? AND transcode_dispatch_at <= ?)
		   OR (transcode_status = ? AND transcode_lease_until <= ?)
		ORDER BY id LIMIT 32`, partWaiting, now, partRunning, now)
	return rows, err
}

func (s *Store) startPart(ctx context.Context, p part, lease time.Time) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `UPDATE video_part SET transcode_status = ?, transcode_dispatch_at = NULL,
		transcode_lease_until = ? WHERE id = ? AND transcode_job_id = ? AND transcode_status = ?`,
		partRunning, lease, p.ID, p.JobID, partWaiting)
	return err
}

func (s *Store) markDispatched(ctx context.Context, p part) error {
	_, err := s.db.ExecContext(ctx, `UPDATE video_part SET transcode_dispatch_at = NULL WHERE id = ?
		AND transcode_job_id = ? AND transcode_status = ? AND transcode_dispatch_at IS NOT NULL`, p.ID, p.JobID, partWaiting)
	return err
}

func (s *Store) finishPart(ctx context.Context, p part) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, `UPDATE video_part SET duration_sec = ?, hls_1080p_url = ?,
		hls_720p_url = ?, hls_480p_url = ?, transcode_status = ?, transcode_dispatch_at = NULL, transcode_lease_until = NULL
		WHERE id = ? AND transcode_job_id = ?`, p.Duration, p.HLS1080, p.HLS720, p.HLS480, p.Status, p.ID, p.JobID)
	return err
}

func (s *Store) extendPartLease(ctx context.Context, p part, lease time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE video_part SET transcode_lease_until = ?
		WHERE id = ? AND transcode_job_id = ? AND transcode_status = ?`,
		lease, p.ID, p.JobID, partRunning)
	return err
}
