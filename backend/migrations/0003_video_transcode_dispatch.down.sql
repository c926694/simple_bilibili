ALTER TABLE video_part
  DROP KEY idx_video_part_reuse,
  DROP KEY idx_video_part_dispatch,
  DROP KEY idx_video_part_lease,
  DROP COLUMN transcode_job_id,
  DROP COLUMN transcode_attempt,
  DROP COLUMN transcode_dispatch_at,
  DROP COLUMN transcode_lease_until;
