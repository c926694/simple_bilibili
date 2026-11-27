ALTER TABLE watch_history MODIFY COLUMN watched_at DATETIME NOT NULL COMMENT '最近一次观看时间，由播放进度事件提供';
DROP TABLE IF EXISTS interaction_stat_outbox;
DROP TABLE IF EXISTS user_checkin;
