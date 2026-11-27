ALTER TABLE video_part
  ADD COLUMN transcode_job_id BIGINT NOT NULL DEFAULT 0 COMMENT '当前转码消息标识，用于识别重复或过期消息',
  ADD COLUMN transcode_attempt INT NOT NULL DEFAULT 0 COMMENT '本轮自动重试次数，从 0 开始',
  ADD COLUMN transcode_dispatch_at DATETIME(3) NULL DEFAULT NULL COMMENT '待投递任务的计划时间，成功投递后清空',
  ADD COLUMN transcode_lease_until DATETIME(3) NULL DEFAULT NULL COMMENT '转码任务执行期限，进程中断后可重新安排',
  ADD KEY idx_video_part_reuse (raw_object_key, transcode_status, id),
  ADD KEY idx_video_part_dispatch (transcode_status, transcode_dispatch_at, id),
  ADD KEY idx_video_part_lease (transcode_status, transcode_lease_until, id);
