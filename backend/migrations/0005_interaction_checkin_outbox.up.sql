CREATE TABLE user_checkin (
  id BIGINT NOT NULL COMMENT '签到记录主键 ID',
  user_id BIGINT NOT NULL COMMENT '签到用户 ID',
  checkin_date DATE NOT NULL COMMENT '上海时区的签到日期',
  reward_coins TINYINT NOT NULL DEFAULT 1 COMMENT '本次签到奖励的代币数量',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '签到记录创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_checkin_date (user_id, checkin_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户每日签到记录表';

CREATE TABLE interaction_stat_outbox (
  id BIGINT NOT NULL COMMENT '互动统计事件 ID',
  video_id BIGINT NOT NULL COMMENT '关联的视频 ID',
  like_delta BIGINT NOT NULL DEFAULT 0 COMMENT '视频点赞数增量',
  coin_delta BIGINT NOT NULL DEFAULT 0 COMMENT '视频投币数增量',
  favorite_delta BIGINT NOT NULL DEFAULT 0 COMMENT '视频收藏人数增量',
  share_delta BIGINT NOT NULL DEFAULT 0 COMMENT '视频分享数增量',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '待投递记录创建时间',
  PRIMARY KEY (id),
  KEY idx_interaction_stat_outbox_video (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='互动统计增量待投递记录表';

ALTER TABLE watch_history MODIFY COLUMN watched_at DATETIME(6) NOT NULL COMMENT '最近一次观看时间，精确到微秒以比较更新与删除顺序';
