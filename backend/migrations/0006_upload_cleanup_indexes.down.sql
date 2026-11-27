ALTER TABLE `video_part`
  DROP INDEX `idx_video_part_raw_key`;

ALTER TABLE `upload_task`
  DROP INDEX `idx_upload_task_object_key`,
  DROP INDEX `idx_upload_task_cleanup`;
