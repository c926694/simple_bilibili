ALTER TABLE `upload_task`
  ADD KEY `idx_upload_task_cleanup` (`status`, `updated_at`, `id`),
  ADD KEY `idx_upload_task_object_key` (`object_key`);

ALTER TABLE `video_part`
  ADD KEY `idx_video_part_raw_key` (`raw_object_key`);
