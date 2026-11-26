CREATE TABLE `user` (
  `id` BIGINT NOT NULL COMMENT '用户主键 ID',
  `username` VARCHAR(64) NOT NULL COMMENT '登录账号名',
  `nickname` VARCHAR(64) NOT NULL COMMENT '用户展示昵称',
  `email` VARCHAR(128) NULL DEFAULT NULL COMMENT '绑定邮箱，未绑定时为空',
  `phone` VARCHAR(32) NULL DEFAULT NULL COMMENT '绑定手机号，未绑定时为空',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '使用 bcrypt 生成的密码哈希',
  `avatar_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '头像访问地址',
  `bio` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '个人签名简介',
  `coin_balance` INT NOT NULL DEFAULT 0 COMMENT '可使用的代币余额',
  `follower_count` INT NOT NULL DEFAULT 0 COMMENT '粉丝数量',
  `following_count` INT NOT NULL DEFAULT 0 COMMENT '关注数量',
  `version` INT NOT NULL DEFAULT 0 COMMENT '代币余额更新使用的乐观锁版本号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '账号状态（1 正常，2 封禁）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '注册时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_username` (`username`),
  UNIQUE KEY `uk_user_email` (`email`),
  UNIQUE KEY `uk_user_phone` (`phone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户账号与个人资料表';

CREATE TABLE `upload_task` (
  `id` BIGINT NOT NULL COMMENT '上传任务主键 ID',
  `upload_id` VARCHAR(128) COLLATE utf8mb4_0900_bin NULL DEFAULT NULL COMMENT 'MinIO 分片上传 ID，秒传时为空',
  `user_id` BIGINT NOT NULL COMMENT '上传者用户 ID',
  `file_hash` VARCHAR(64) NOT NULL COMMENT '小写十六进制 SHA-256 文件哈希',
  `file_name` VARCHAR(255) NOT NULL COMMENT '原始文件名',
  `file_size` BIGINT NOT NULL COMMENT '文件总字节数',
  `chunk_size` INT NOT NULL DEFAULT 0 COMMENT '单个分片的字节数，秒传时为 0',
  `total_chunks` INT NOT NULL DEFAULT 0 COMMENT '分片总数，秒传时为 0',
  `object_key` VARCHAR(512) NOT NULL COMMENT 'MinIO 目标对象键',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '上传状态（0 上传中，1 已合并或秒传完成，2 已取消或过期）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_upload_task_upload_id` (`upload_id`),
  KEY `idx_upload_task_resume` (`user_id`, `file_hash`, `file_size`, `status`, `updated_at` DESC, `id` DESC),
  KEY `idx_upload_task_dedup` (`file_hash`, `file_size`, `status`, `updated_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='分片上传任务表';

CREATE TABLE `category` (
  `id` INT NOT NULL COMMENT '分区主键 ID',
  `name` VARCHAR(64) NOT NULL COMMENT '分区名称',
  `sort_order` INT NOT NULL DEFAULT 0 COMMENT '展示顺序，数值越小越靠前',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '分区状态（0 停用，1 启用）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_category_name` (`name`),
  KEY `idx_category_display` (`status`, `sort_order`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频分区表';

CREATE TABLE `home_banner` (
  `id` BIGINT NOT NULL COMMENT '首页轮播主键 ID',
  `title` VARCHAR(128) NOT NULL COMMENT '轮播标题',
  `image_url` VARCHAR(512) NOT NULL COMMENT '轮播图片访问地址',
  `link_url` VARCHAR(512) NOT NULL COMMENT '点击轮播后的跳转地址',
  `sort_order` INT NOT NULL DEFAULT 0 COMMENT '展示顺序，数值越小越靠前',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '展示状态（0 停用，1 启用）',
  `starts_at` DATETIME NULL DEFAULT NULL COMMENT '展示开始时间，留空表示启用后即可展示',
  `ends_at` DATETIME NULL DEFAULT NULL COMMENT '展示结束时间，留空表示持续展示',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_home_banner_display` (`status`, `sort_order`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='首页轮播配置表';

CREATE TABLE `video` (
  `id` BIGINT NOT NULL COMMENT '视频主键 ID',
  `bv_id` VARCHAR(32) COLLATE utf8mb4_0900_bin NOT NULL COMMENT '对外展示的视频唯一编码，区分英文大小写',
  `author_id` BIGINT NOT NULL COMMENT '稿件作者用户 ID',
  `category_id` INT NULL DEFAULT NULL COMMENT '所属分区 ID，草稿未选择分区时为空',
  `title` VARCHAR(128) NOT NULL COMMENT '视频标题',
  `description` TEXT NULL DEFAULT NULL COMMENT '视频简介',
  `cover_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '视频封面访问地址，尚未设置时为空字符串',
  `duration_sec` INT NOT NULL DEFAULT 0 COMMENT '视频总时长，单位为秒',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '稿件状态（0 草稿，1 转码中，2 审核中，3 已发布，4 转码失败，5 已下架）',
  `published_at` DATETIME NULL DEFAULT NULL COMMENT '正式发布时间，首次发布前为空',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_video_bv_id` (`bv_id`),
  KEY `idx_video_author_status_published` (`author_id`, `status`, `published_at` DESC, `id` DESC),
  KEY `idx_video_category_status_published` (`category_id`, `status`, `published_at` DESC, `id` DESC),
  KEY `idx_video_status_published` (`status`, `published_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频稿件主表';

CREATE TABLE `video_part` (
  `id` BIGINT NOT NULL COMMENT '视频分 P 主键 ID',
  `video_id` BIGINT NOT NULL COMMENT '所属视频 ID',
  `part_index` INT NOT NULL COMMENT '分 P 序号，从 1 开始',
  `title` VARCHAR(128) NOT NULL COMMENT '分 P 标题',
  `duration_sec` INT NOT NULL DEFAULT 0 COMMENT '当前分 P 的时长，单位为秒',
  `raw_object_key` VARCHAR(512) NOT NULL COMMENT '原始视频在 MinIO 中的对象键',
  `hls_1080p_url` VARCHAR(512) NULL DEFAULT NULL COMMENT '1080P 的 HLS 播放列表地址，未生成时为空',
  `hls_720p_url` VARCHAR(512) NULL DEFAULT NULL COMMENT '720P 的 HLS 播放列表地址，未生成时为空',
  `hls_480p_url` VARCHAR(512) NULL DEFAULT NULL COMMENT '480P 的 HLS 播放列表地址，未生成时为空',
  `transcode_status` TINYINT NOT NULL DEFAULT 0 COMMENT '转码状态（0 待转码，1 转码中，2 已成功，3 已失败）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_video_part_order` (`video_id`, `part_index`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频选集表';

CREATE TABLE `video_stat` (
  `video_id` BIGINT NOT NULL COMMENT '对应的视频主键 ID',
  `play_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计播放量',
  `danmaku_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计弹幕数',
  `like_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计点赞数',
  `coin_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计投币数',
  `favorite_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计收藏数',
  `share_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计转发数',
  `comment_count` BIGINT NOT NULL DEFAULT 0 COMMENT '累计评论数',
  `hot_score` DOUBLE NOT NULL DEFAULT 0 COMMENT '最近一次计算的热度分值',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最近一次统计同步时间',
  PRIMARY KEY (`video_id`),
  KEY `idx_video_stat_play` (`play_count` DESC, `video_id` DESC),
  KEY `idx_video_stat_danmaku` (`danmaku_count` DESC, `video_id` DESC),
  KEY `idx_video_stat_hot` (`hot_score` DESC, `video_id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频互动统计表';

CREATE TABLE `tag` (
  `id` BIGINT NOT NULL COMMENT '标签主键 ID',
  `name` VARCHAR(64) NOT NULL COMMENT '标签名称',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tag_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频标签表';

CREATE TABLE `video_tag` (
  `id` BIGINT NOT NULL COMMENT '视频标签关系主键 ID',
  `video_id` BIGINT NOT NULL COMMENT '关联的视频 ID',
  `tag_id` BIGINT NOT NULL COMMENT '关联的标签 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '建立关联的时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_video_tag_pair` (`video_id`, `tag_id`),
  KEY `idx_video_tag_reverse` (`tag_id`, `video_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频与标签关系表';

CREATE TABLE `danmaku` (
  `id` BIGINT NOT NULL COMMENT '弹幕主键 ID，由异步写入事件携带',
  `video_id` BIGINT NOT NULL COMMENT '所属视频 ID',
  `part_id` BIGINT NOT NULL COMMENT '所属分 P ID',
  `user_id` BIGINT NOT NULL COMMENT '弹幕发送者用户 ID',
  `content` VARCHAR(255) NOT NULL COMMENT '弹幕文本内容',
  `time_offset_ms` INT NOT NULL COMMENT '弹幕出现的播放位置，单位为毫秒',
  `mode` TINYINT NOT NULL DEFAULT 1 COMMENT '展示模式（1 滚动，2 顶部固定，3 底部固定）',
  `font_size` TINYINT NOT NULL DEFAULT 25 COMMENT '弹幕字号，单位为像素',
  `color` VARCHAR(16) NOT NULL DEFAULT '#FFFFFF' COMMENT '弹幕的十六进制颜色值',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '弹幕发送时间',
  PRIMARY KEY (`id`),
  KEY `idx_danmaku_part_time` (`video_id`, `part_id`, `time_offset_ms`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频弹幕表';

CREATE TABLE `comment` (
  `id` BIGINT NOT NULL COMMENT '评论主键 ID',
  `video_id` BIGINT NOT NULL COMMENT '所属视频 ID',
  `user_id` BIGINT NOT NULL COMMENT '评论作者用户 ID',
  `root_id` BIGINT NOT NULL DEFAULT 0 COMMENT '所属根评论 ID，根评论自身取 0',
  `parent_id` BIGINT NOT NULL DEFAULT 0 COMMENT '直接回复的评论 ID，根评论取 0',
  `reply_to_user_id` BIGINT NOT NULL DEFAULT 0 COMMENT '被回复的用户 ID，根评论取 0',
  `content` TEXT NOT NULL COMMENT '评论正文',
  `like_count` INT NOT NULL DEFAULT 0 COMMENT '评论点赞数',
  `reply_count` INT NOT NULL DEFAULT 0 COMMENT '根评论下的子回复数',
  `is_pinned` TINYINT NOT NULL DEFAULT 0 COMMENT 'UP 主置顶标记（0 否，1 是）',
  `is_up_liked` TINYINT NOT NULL DEFAULT 0 COMMENT 'UP 主点赞标记（0 否，1 是）',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '评论状态（1 正常，2 已删除）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '评论发表时间',
  PRIMARY KEY (`id`),
  KEY `idx_comment_video_roots` (`video_id`, `root_id`, `status`, `is_pinned` DESC, `created_at` DESC, `id` DESC),
  KEY `idx_comment_root_replies` (`root_id`, `status`, `created_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频评论与回复表';

CREATE TABLE `comment_reaction` (
  `id` BIGINT NOT NULL COMMENT '评论表态记录主键 ID',
  `comment_id` BIGINT NOT NULL COMMENT '目标评论 ID',
  `user_id` BIGINT NOT NULL COMMENT '操作用户 ID',
  `reaction_type` TINYINT NOT NULL COMMENT '表态类型（1 点赞，2 点踩）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次表态时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最近一次变更表态的时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_comment_reaction_pair` (`comment_id`, `user_id`),
  KEY `idx_comment_reaction_type` (`comment_id`, `reaction_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='评论点赞与点踩记录表';

CREATE TABLE `video_interaction` (
  `id` BIGINT NOT NULL COMMENT '视频互动主键 ID',
  `user_id` BIGINT NOT NULL COMMENT '操作用户 ID',
  `video_id` BIGINT NOT NULL COMMENT '目标视频 ID',
  `liked` TINYINT NOT NULL DEFAULT 0 COMMENT '视频点赞状态（0 未点赞，1 已点赞）',
  `coin_count` TINYINT NOT NULL DEFAULT 0 COMMENT '用户对该视频累计投币数量，最大为 2',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_video_interaction_pair` (`user_id`, `video_id`),
  CONSTRAINT `ck_video_interaction_coin_count` CHECK (`coin_count` BETWEEN 0 AND 2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频点赞与投币汇总表';

CREATE TABLE `coin_ledger` (
  `id` BIGINT NOT NULL COMMENT '投币流水主键 ID',
  `user_id` BIGINT NOT NULL COMMENT '支付代币的用户 ID',
  `video_id` BIGINT NOT NULL COMMENT '投币目标视频 ID',
  `request_id` VARCHAR(64) COLLATE utf8mb4_0900_bin NOT NULL COMMENT '投币请求标识，同一请求重试时保持一致',
  `coin_count` TINYINT NOT NULL COMMENT '本次扣除的代币数量（1 或 2）',
  `balance_after` INT NOT NULL COMMENT '本次扣款完成后的代币余额',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '扣款记账时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_coin_ledger_request` (`user_id`, `request_id`),
  KEY `idx_coin_ledger_user_time` (`user_id`, `created_at` DESC, `id` DESC),
  CONSTRAINT `ck_coin_ledger_coin_count` CHECK (`coin_count` BETWEEN 1 AND 2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频投币扣款流水表';

CREATE TABLE `favorite_folder` (
  `id` BIGINT NOT NULL COMMENT '收藏夹主键 ID',
  `user_id` BIGINT NOT NULL COMMENT '收藏夹所属用户 ID',
  `title` VARCHAR(64) NOT NULL COMMENT '收藏夹名称',
  `is_default` TINYINT NOT NULL DEFAULT 0 COMMENT '默认标记（0 普通收藏夹，1 默认收藏夹）',
  `is_public` TINYINT NOT NULL DEFAULT 0 COMMENT '公开状态（0 私密，1 公开）',
  `item_count` INT NOT NULL DEFAULT 0 COMMENT '收藏夹内的视频数量',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_favorite_folder_default` ((CASE WHEN is_default = 1 THEN user_id END)),
  KEY `idx_favorite_folder_user` (`user_id`, `created_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户收藏夹表';

CREATE TABLE `favorite_item` (
  `id` BIGINT NOT NULL COMMENT '收藏记录主键 ID',
  `folder_id` BIGINT NOT NULL COMMENT '所属收藏夹 ID',
  `user_id` BIGINT NOT NULL COMMENT '收藏用户 ID，与收藏夹所属用户一致',
  `video_id` BIGINT NOT NULL COMMENT '收藏的视频 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '收藏时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_favorite_item_pair` (`folder_id`, `video_id`),
  KEY `idx_favorite_item_folder_time` (`folder_id`, `created_at` DESC, `id` DESC),
  KEY `idx_favorite_item_user_video` (`user_id`, `video_id`, `folder_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='收藏夹视频记录表';

CREATE TABLE `watch_history` (
  `id` BIGINT NOT NULL COMMENT '观看历史主键 ID',
  `user_id` BIGINT NOT NULL COMMENT '观看者用户 ID',
  `video_id` BIGINT NOT NULL COMMENT '观看的视频 ID',
  `part_id` BIGINT NOT NULL COMMENT '最后观看的分 P ID',
  `progress_sec` INT NOT NULL DEFAULT 0 COMMENT '最后一次观看进度，单位为秒',
  `watched_at` DATETIME NOT NULL COMMENT '最近一次观看时间，由播放进度事件提供',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_watch_history_pair` (`user_id`, `video_id`),
  KEY `idx_watch_history_user_time` (`user_id`, `watched_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频观看历史表';

CREATE TABLE `user_follow` (
  `id` BIGINT NOT NULL COMMENT '关注记录主键 ID',
  `follower_id` BIGINT NOT NULL COMMENT '关注发起者用户 ID',
  `followee_id` BIGINT NOT NULL COMMENT '被关注者用户 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '建立关注的时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_follow_pair` (`follower_id`, `followee_id`),
  KEY `idx_user_follow_following` (`follower_id`, `created_at` DESC, `id` DESC),
  KEY `idx_user_follow_followers` (`followee_id`, `created_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户关注关系表';

CREATE TABLE `dynamic_post` (
  `id` BIGINT NOT NULL COMMENT '动态主键 ID',
  `user_id` BIGINT NOT NULL COMMENT '动态发布者用户 ID',
  `dynamic_type` TINYINT NOT NULL COMMENT '动态类型（1 视频投稿，2 图文动态）',
  `video_id` BIGINT NULL DEFAULT NULL COMMENT '投稿动态对应的视频 ID，图文动态时为空',
  `content` TEXT NULL DEFAULT NULL COMMENT '动态文案，未填写时为空',
  `image_urls` JSON NULL DEFAULT NULL COMMENT '配图访问地址数组，未添加配图时为空',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '动态发布时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dynamic_post_video` (`video_id`),
  KEY `idx_dynamic_post_user_time` (`user_id`, `created_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户动态内容表';

CREATE TABLE `dynamic_inbox` (
  `receiver_id` BIGINT NOT NULL COMMENT '动态接收者用户 ID',
  `dynamic_id` BIGINT NOT NULL COMMENT '分发的动态 ID',
  `published_at` DATETIME NOT NULL COMMENT '原动态发布时间，取自动态内容记录',
  PRIMARY KEY (`receiver_id`, `dynamic_id`),
  KEY `idx_dynamic_inbox_receiver_time` (`receiver_id`, `published_at` DESC, `dynamic_id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='关注动态收件箱表';

CREATE TABLE `notification` (
  `id` BIGINT NOT NULL COMMENT '通知主键 ID，由异步写入事件携带',
  `receiver_id` BIGINT NOT NULL COMMENT '接收通知的用户 ID',
  `sender_id` BIGINT NOT NULL DEFAULT 0 COMMENT '触发通知的用户 ID，系统通知取 0',
  `notify_type` TINYINT NOT NULL COMMENT '通知分类（1 评论回复，2 收到点赞，3 新增粉丝，4 系统通知）',
  `target_type` TINYINT NOT NULL DEFAULT 0 COMMENT '关联对象类型（0 无目标，1 视频，2 评论）',
  `target_id` BIGINT NULL DEFAULT NULL COMMENT '关联的业务对象 ID，无关联对象时为空',
  `content` VARCHAR(255) NOT NULL COMMENT '通知摘要',
  `is_read` TINYINT NOT NULL DEFAULT 0 COMMENT '已读状态（0 未读，1 已读）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '通知创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_notification_receiver_type_time` (`receiver_id`, `notify_type`, `created_at` DESC, `id` DESC),
  KEY `idx_notification_receiver_unread` (`receiver_id`, `is_read`, `notify_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户站内消息表';

CREATE TABLE `user_video_behavior` (
  `id` BIGINT NOT NULL COMMENT '行为主键 ID，由行为事件携带',
  `user_id` BIGINT NOT NULL COMMENT '触发行为的用户 ID',
  `video_id` BIGINT NOT NULL COMMENT '关联的视频 ID',
  `behavior_type` TINYINT NOT NULL COMMENT '行为类型（1 有效播放，2 点赞，3 收藏，4 投币，5 转发，6 不感兴趣）',
  `weight` INT NOT NULL COMMENT '行为权重（播放加 1，点赞加 3，收藏加 4，投币加 5，转发加 4，不感兴趣减 5）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '行为发生时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_video_behavior_time` (`created_at`, `id`),
  KEY `idx_user_video_behavior_user_time` (`user_id`, `created_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='推荐用户行为记录表';
