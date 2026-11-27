CREATE TABLE danmaku_stat_outbox (
  id BIGINT NOT NULL COMMENT '弹幕统计增量事件 ID',
  video_id BIGINT NOT NULL COMMENT '关联的视频 ID',
  danmaku_delta BIGINT NOT NULL COMMENT '本批实际新增的弹幕条数',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '待投递记录的创建时间',
  PRIMARY KEY (id),
  KEY idx_danmaku_stat_outbox_video (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='弹幕统计待投递记录表';

CREATE TABLE video_stat_event (
  id BIGINT NOT NULL COMMENT '已处理的统计增量事件 ID',
  video_id BIGINT NOT NULL COMMENT '关联的视频 ID',
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '增量写入统计的时间',
  PRIMARY KEY (id),
  KEY idx_video_stat_event_video (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频统计增量事件处理记录表';
