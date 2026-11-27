# MySQL 业务表结构

- 表引擎为 `InnoDB`，默认字符集为 `utf8mb4`，默认排序规则为 `utf8mb4_0900_ai_ci`。需区分大小写的标识字段列出各自的排序规则。
- 默认值列中的「无」表示 SQL 未声明默认值，`NULL` 和 `''` 分别表示空值和空字符串。联合索引按列出的顺序建立，未注明方向的索引字段使用升序。
- 主键由写入方提供。跨表通过逻辑外键关联，先取关联 ID，再通过 `sqlx.In` 批量查询并在服务层组装。使用 `0` 或 `NULL` 表示无关联的字段在说明中标明。

## user 模块

### `user` 用户账号与个人资料表

保存登录信息、个人资料、代币余额及关注计数。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 用户主键 ID | — |
| `username` | `VARCHAR(64)` | 无 | 否 | 登录账号名 | — |
| `nickname` | `VARCHAR(64)` | 无 | 否 | 用户展示昵称 | — |
| `email` | `VARCHAR(128)` | `NULL` | 是 | 绑定邮箱，未绑定时为空 | — |
| `phone` | `VARCHAR(32)` | `NULL` | 是 | 绑定手机号，未绑定时为空 | — |
| `password_hash` | `VARCHAR(255)` | 无 | 否 | 使用 bcrypt 生成的密码哈希 | — |
| `avatar_url` | `VARCHAR(512)` | `''` | 否 | 头像访问地址 | — |
| `bio` | `VARCHAR(255)` | `''` | 否 | 个人签名简介 | — |
| `coin_balance` | `INT` | `0` | 否 | 可使用的代币余额 | — |
| `follower_count` | `INT` | `0` | 否 | 粉丝数量 | — |
| `following_count` | `INT` | `0` | 否 | 关注数量 | — |
| `version` | `INT` | `0` | 否 | 代币余额更新使用的乐观锁版本号 | — |
| `status` | `TINYINT` | `1` | 否 | 账号状态（1 正常，2 封禁） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 注册时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 更新时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_user_username` | 唯一索引 | `username` | 账号登录并禁止重名。 |
| `uk_user_email` | 唯一索引 | `email` | 邮箱登录并禁止同一邮箱绑定多次。 |
| `uk_user_phone` | 唯一索引 | `phone` | 手机号登录并禁止同一手机号绑定多次。 |

- 邮箱和手机号均可留空，未绑定的字段使用 `NULL`。注册流程为新用户写入 10 枚代币。

## upload 模块

### `user_checkin` 用户每日签到记录表

保存上海时区的每日签到与奖励。用户 ID 和签到日期联合唯一。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 签到记录主键 ID | — |
| `user_id` | `BIGINT` | 无 | 否 | 签到用户 ID | `user.id` |
| `checkin_date` | `DATE` | 无 | 否 | 上海时区的签到日期 | — |
| `reward_coins` | `TINYINT` | `1` | 否 | 本次签到奖励的代币数量 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 签到记录创建时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_user_checkin_date` | 唯一索引 | `user_id`、`checkin_date` | 限制每日奖励次数并支持按月回填位图。 |

### `upload_task` 分片上传任务表

记录上传任务、文件校验信息和对象位置，支持秒传和续传。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 上传任务主键 ID | — |
| `upload_id` | `VARCHAR(128) COLLATE utf8mb4_0900_bin` | `NULL` | 是 | MinIO 分片上传 ID，秒传时为空 | — |
| `user_id` | `BIGINT` | 无 | 否 | 上传者用户 ID | `user.id` |
| `file_hash` | `VARCHAR(64)` | 无 | 否 | 小写十六进制 SHA-256 文件哈希 | — |
| `file_name` | `VARCHAR(255)` | 无 | 否 | 原始文件名 | — |
| `file_size` | `BIGINT` | 无 | 否 | 文件总字节数 | — |
| `chunk_size` | `INT` | `0` | 否 | 单个分片的字节数，秒传时为 0 | — |
| `total_chunks` | `INT` | `0` | 否 | 分片总数，秒传时为 0 | — |
| `object_key` | `VARCHAR(512)` | 无 | 否 | MinIO 目标对象键 | — |
| `status` | `TINYINT` | `0` | 否 | 上传状态（0 上传中，1 已合并或秒传完成，2 已取消或过期） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 更新时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_upload_task_upload_id` | 唯一索引 | `upload_id` | 定位分片上传会话。 |
| `idx_upload_task_resume` | 索引 | `user_id`、`file_hash`、`file_size`、`status`、`updated_at DESC`、`id DESC` | 按用户和文件定位最近一次可续传任务。 |
| `idx_upload_task_dedup` | 索引 | `file_hash`、`file_size`、`status`、`updated_at DESC`、`id DESC` | 查找同一文件已完成的上传记录。 |

- 续传时通过 `upload_id` 查询 MinIO 已接收的分片。

## video 模块

### `category` 视频分区表

定义首页展示和视频投稿使用的分区。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `INT` | 无 | 否 | 分区主键 ID | — |
| `name` | `VARCHAR(64)` | 无 | 否 | 分区名称 | — |
| `sort_order` | `INT` | `0` | 否 | 展示顺序，数值越小越靠前 | — |
| `status` | `TINYINT` | `1` | 否 | 分区状态（0 停用，1 启用） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 更新时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_category_name` | 唯一索引 | `name` | 禁止分区名称重复。 |
| `idx_category_display` | 索引 | `status`、`sort_order`、`id` | 按顺序查询启用的分区。 |

### `home_banner` 首页轮播配置表

管理首页轮播图片、跳转地址、展示时间和顺序。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 首页轮播主键 ID | — |
| `title` | `VARCHAR(128)` | 无 | 否 | 轮播标题 | — |
| `image_url` | `VARCHAR(512)` | 无 | 否 | 轮播图片访问地址 | — |
| `link_url` | `VARCHAR(512)` | 无 | 否 | 点击轮播后的跳转地址 | — |
| `sort_order` | `INT` | `0` | 否 | 展示顺序，数值越小越靠前 | — |
| `status` | `TINYINT` | `1` | 否 | 展示状态（0 停用，1 启用） | — |
| `starts_at` | `DATETIME` | `NULL` | 是 | 展示开始时间，留空表示启用后即可展示 | — |
| `ends_at` | `DATETIME` | `NULL` | 是 | 展示结束时间，留空表示持续展示 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 更新时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `idx_home_banner_display` | 索引 | `status`、`sort_order`、`id` | 按顺序读取启用的轮播，再按展示时段筛选。 |

### `video` 视频稿件主表

保存稿件信息、作者、分区和发布状态。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 视频主键 ID | — |
| `bv_id` | `VARCHAR(32) COLLATE utf8mb4_0900_bin` | 无 | 否 | 对外展示的视频唯一编码，区分英文大小写 | — |
| `author_id` | `BIGINT` | 无 | 否 | 稿件作者用户 ID | `user.id` |
| `category_id` | `INT` | `NULL` | 是 | 所属分区 ID，草稿未选择分区时为空 | `category.id` |
| `title` | `VARCHAR(128)` | 无 | 否 | 视频标题 | — |
| `description` | `TEXT` | `NULL` | 是 | 视频简介 | — |
| `cover_url` | `VARCHAR(512)` | `''` | 否 | 视频封面访问地址，尚未设置时为空字符串 | — |
| `duration_sec` | `INT` | `0` | 否 | 视频总时长，单位为秒 | — |
| `status` | `TINYINT` | `0` | 否 | 稿件状态（0 草稿，1 转码中，2 审核中，3 已发布，4 转码失败，5 已下架） | — |
| `published_at` | `DATETIME` | `NULL` | 是 | 正式发布时间，首次发布前为空 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 更新时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_video_bv_id` | 唯一索引 | `bv_id` | 按公开视频编码查询稿件并禁止编码重复。 |
| `idx_video_author_status_published` | 索引 | `author_id`、`status`、`published_at DESC`、`id DESC` | 分页查询指定作者的已发布稿件。 |
| `idx_video_category_status_published` | 索引 | `category_id`、`status`、`published_at DESC`、`id DESC` | 分页查询指定分区的已发布稿件。 |
| `idx_video_status_published` | 索引 | `status`、`published_at DESC`、`id DESC` | 分页查询全站已发布的最新稿件。 |

### `video_part` 视频选集表

保存分 P 顺序、原始文件和各清晰度播放地址。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 视频分 P 主键 ID | — |
| `video_id` | `BIGINT` | 无 | 否 | 所属视频 ID | `video.id` |
| `part_index` | `INT` | 无 | 否 | 分 P 序号，从 1 开始 | — |
| `title` | `VARCHAR(128)` | 无 | 否 | 分 P 标题 | — |
| `duration_sec` | `INT` | `0` | 否 | 当前分 P 的时长，单位为秒 | — |
| `raw_object_key` | `VARCHAR(512)` | 无 | 否 | 原始视频在 MinIO 中的对象键 | — |
| `hls_1080p_url` | `VARCHAR(512)` | `NULL` | 是 | 1080P 的 HLS 播放列表地址，未生成时为空 | — |
| `hls_720p_url` | `VARCHAR(512)` | `NULL` | 是 | 720P 的 HLS 播放列表地址，未生成时为空 | — |
| `hls_480p_url` | `VARCHAR(512)` | `NULL` | 是 | 480P 的 HLS 播放列表地址，未生成时为空 | — |
| `transcode_status` | `TINYINT` | `0` | 否 | 转码状态（0 待转码，1 转码中，2 已成功，3 已失败） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |
| `transcode_job_id` | `BIGINT` | `0` | 否 | 当前转码消息标识，用于识别重复或过期消息 | — |
| `transcode_attempt` | `INT` | `0` | 否 | 本轮自动重试次数，从 0 开始 | — |
| `transcode_dispatch_at` | `DATETIME(3)` | `NULL` | 是 | 待投递任务的计划时间，成功投递后清空 | — |
| `transcode_lease_until` | `DATETIME(3)` | `NULL` | 是 | 转码任务执行期限，进程中断后可重新安排 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_video_part_order` | 唯一索引 | `video_id`、`part_index` | 按顺序读取选集，同一视频内的分 P 序号唯一。 |
| `idx_video_part_reuse` | 普通索引 | `raw_object_key`、`transcode_status`、`id` | 查询可复用的转码结果。 |
| `idx_video_part_dispatch` | 普通索引 | `transcode_status`、`transcode_dispatch_at`、`id` | 查询到期的待投递任务。 |
| `idx_video_part_lease` | 普通索引 | `transcode_status`、`transcode_lease_until`、`id` | 查询执行期限已过的转码任务。 |

### `video_stat` 视频互动统计表

保存视频的累计计数和最近一次计算的热度分值。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `video_id` | `BIGINT` | 无 | 否 | 对应的视频主键 ID | `video.id` |
| `play_count` | `BIGINT` | `0` | 否 | 累计播放量 | — |
| `danmaku_count` | `BIGINT` | `0` | 否 | 累计弹幕数 | — |
| `like_count` | `BIGINT` | `0` | 否 | 累计点赞数 | — |
| `coin_count` | `BIGINT` | `0` | 否 | 累计投币数 | — |
| `favorite_count` | `BIGINT` | `0` | 否 | 累计收藏数 | — |
| `share_count` | `BIGINT` | `0` | 否 | 累计转发数 | — |
| `comment_count` | `BIGINT` | `0` | 否 | 累计评论数 | — |
| `hot_score` | `DOUBLE` | `0` | 否 | 最近一次计算的热度分值 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 最近一次统计同步时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `video_id` | 唯一标识记录。 |
| `idx_video_stat_play` | 索引 | `play_count DESC`、`video_id DESC` | 支持按播放量排序。 |
| `idx_video_stat_danmaku` | 索引 | `danmaku_count DESC`、`video_id DESC` | 支持按弹幕数排序。 |
| `idx_video_stat_hot` | 索引 | `hot_score DESC`、`video_id DESC` | 支持按热度分值排序。 |

### `video_stat_event` 视频统计增量事件处理记录表

保存已应用的统计增量 ID，确保重复投递只更新一次计数。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 已处理的统计增量事件 ID | — |
| `video_id` | `BIGINT` | 无 | 否 | 关联的视频 ID | `video.id` |
| `applied_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 增量写入统计的时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 按事件 ID 去重。 |
| `idx_video_stat_event_video` | 普通索引 | `video_id` | 按视频查询已应用事件。 |

### `tag` 视频标签表

保存供视频关联和推荐使用的标签名称。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 标签主键 ID | — |
| `name` | `VARCHAR(64)` | 无 | 否 | 标签名称 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_tag_name` | 唯一索引 | `name` | 按名称查找标签并禁止标签名称重复。 |

### `video_tag` 视频与标签关系表

保存视频与标签之间的多对多关系。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 视频标签关系主键 ID | — |
| `video_id` | `BIGINT` | 无 | 否 | 关联的视频 ID | `video.id` |
| `tag_id` | `BIGINT` | 无 | 否 | 关联的标签 ID | `tag.id` |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 建立关联的时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_video_tag_pair` | 唯一索引 | `video_id`、`tag_id` | 查询视频标签，并限制同一视频对同一标签只关联一次。 |
| `idx_video_tag_reverse` | 索引 | `tag_id`、`video_id` | 通过标签批量查找视频 ID。 |

## danmaku 模块

### `danmaku` 视频弹幕表

保存弹幕正文、播放位置和展示样式，支持按分 P 和时间窗口加载。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 弹幕主键 ID，由异步写入事件携带 | — |
| `video_id` | `BIGINT` | 无 | 否 | 所属视频 ID | `video.id` |
| `part_id` | `BIGINT` | 无 | 否 | 所属分 P ID | `video_part.id` |
| `user_id` | `BIGINT` | 无 | 否 | 弹幕发送者用户 ID | `user.id` |
| `content` | `VARCHAR(255)` | 无 | 否 | 弹幕文本内容 | — |
| `time_offset_ms` | `INT` | 无 | 否 | 弹幕出现的播放位置，单位为毫秒 | — |
| `mode` | `TINYINT` | `1` | 否 | 展示模式（1 滚动，2 顶部固定，3 底部固定） | — |
| `font_size` | `TINYINT` | `25` | 否 | 弹幕字号，单位为像素 | — |
| `color` | `VARCHAR(16)` | `'#FFFFFF'` | 否 | 弹幕的十六进制颜色值 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 弹幕发送时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `idx_danmaku_part_time` | 索引 | `video_id`、`part_id`、`time_offset_ms`、`id` | 按视频、分 P、播放位置和 ID 顺序加载弹幕。 |

- 异步写入重试沿用同一条弹幕的 `id`。

## comment 模块

### `danmaku_stat_outbox` 弹幕统计待投递记录表

与弹幕入库在同一事务内记录本批实际增量，投递成功后删除待发记录。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 弹幕统计增量事件 ID | — |
| `video_id` | `BIGINT` | 无 | 否 | 关联的视频 ID | `video.id` |
| `danmaku_delta` | `BIGINT` | 无 | 否 | 本批实际新增的弹幕条数 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 待投递记录的创建时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 确定投递顺序并定位待发记录。 |
| `idx_danmaku_stat_outbox_video` | 普通索引 | `video_id` | 按视频查询待投递增量。 |

### `comment` 视频评论与回复表

保存根评论、两级楼中楼回复、展示标记和评论计数。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 评论主键 ID | — |
| `video_id` | `BIGINT` | 无 | 否 | 所属视频 ID | `video.id` |
| `user_id` | `BIGINT` | 无 | 否 | 评论作者用户 ID | `user.id` |
| `root_id` | `BIGINT` | `0` | 否 | 所属根评论 ID，根评论自身取 0 | `comment.id` |
| `parent_id` | `BIGINT` | `0` | 否 | 直接回复的评论 ID，根评论取 0 | `comment.id` |
| `reply_to_user_id` | `BIGINT` | `0` | 否 | 被回复的用户 ID，根评论取 0 | `user.id` |
| `content` | `TEXT` | 无 | 否 | 评论正文 | — |
| `like_count` | `INT` | `0` | 否 | 评论点赞数 | — |
| `reply_count` | `INT` | `0` | 否 | 根评论下的子回复数 | — |
| `is_pinned` | `TINYINT` | `0` | 否 | UP 主置顶标记（0 否，1 是） | — |
| `is_up_liked` | `TINYINT` | `0` | 否 | UP 主点赞标记（0 否，1 是） | — |
| `status` | `TINYINT` | `1` | 否 | 评论状态（1 正常，2 已删除） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 评论发表时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `idx_comment_video_roots` | 索引 | `video_id`、`root_id`、`status`、`is_pinned DESC`、`created_at DESC`、`id DESC` | 查询视频的根评论，置顶优先并按发表时间分页。 |
| `idx_comment_root_replies` | 索引 | `root_id`、`status`、`created_at`、`id` | 按发表顺序分页查询某条根评论的回复。 |

- 热评排序由服务层按点赞数、回复数和发布时间计算，热门结果缓存在 Redis。

### `comment_reaction` 评论点赞与点踩记录表

记录每位用户当前对一条评论的表态。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 评论表态记录主键 ID | — |
| `comment_id` | `BIGINT` | 无 | 否 | 目标评论 ID | `comment.id` |
| `user_id` | `BIGINT` | 无 | 否 | 操作用户 ID | `user.id` |
| `reaction_type` | `TINYINT` | 无 | 否 | 表态类型（1 点赞，2 点踩） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 首次表态时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 最近一次变更表态的时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_comment_reaction_pair` | 唯一索引 | `comment_id`、`user_id` | 每位用户对同一评论只保留一条表态记录。 |
| `idx_comment_reaction_type` | 索引 | `comment_id`、`reaction_type` | 按评论和表态类型统计。 |

- 取消表态时删除相应记录。

## interaction 模块

### `video_interaction` 视频点赞与投币汇总表

保存用户对单个视频的点赞状态和累计投币数量。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 视频互动主键 ID | — |
| `user_id` | `BIGINT` | 无 | 否 | 操作用户 ID | `user.id` |
| `video_id` | `BIGINT` | 无 | 否 | 目标视频 ID | `video.id` |
| `liked` | `TINYINT` | `0` | 否 | 视频点赞状态（0 未点赞，1 已点赞） | — |
| `coin_count` | `TINYINT` | `0` | 否 | 用户对该视频累计投币数量，最大为 2 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |
| `updated_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 更新时间。更新记录时自动写入当前时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_video_interaction_pair` | 唯一索引 | `user_id`、`video_id` | 查询用户对视频的互动状态，同一组合只保留一条记录。 |

- `ck_video_interaction_coin_count` 检查约束。累计投币数范围为 0 至 2。

### `interaction_stat_outbox` 互动统计增量待投递记录表

与互动变更同一事务写入。RocketMQ 确认收到消息后删除待发记录。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 互动统计事件 ID | — |
| `video_id` | `BIGINT` | 无 | 否 | 关联的视频 ID | `video.id` |
| `like_delta` | `BIGINT` | `0` | 否 | 视频点赞数增量 | — |
| `coin_delta` | `BIGINT` | `0` | 否 | 视频投币数增量 | — |
| `favorite_delta` | `BIGINT` | `0` | 否 | 视频收藏人数增量 | — |
| `share_delta` | `BIGINT` | `0` | 否 | 视频分享数增量 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 待投递记录创建时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 定位待发事件并保持投递顺序。 |
| `idx_interaction_stat_outbox_video` | 普通索引 | `video_id` | 查询指定视频的待投递增量。 |

### `coin_ledger` 视频投币扣款流水表

保存每次成功的投币扣款、扣款后余额和防重复记账标识。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 投币流水主键 ID | — |
| `user_id` | `BIGINT` | 无 | 否 | 支付代币的用户 ID | `user.id` |
| `video_id` | `BIGINT` | 无 | 否 | 投币目标视频 ID | `video.id` |
| `request_id` | `VARCHAR(64) COLLATE utf8mb4_0900_bin` | 无 | 否 | 投币请求标识，同一请求重试时保持一致 | — |
| `coin_count` | `TINYINT` | 无 | 否 | 本次扣除的代币数量（1 或 2） | — |
| `balance_after` | `INT` | 无 | 否 | 本次扣款完成后的代币余额 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 扣款记账时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_coin_ledger_request` | 唯一索引 | `user_id`、`request_id` | 同一用户的一次投币请求只记录一笔流水。 |
| `idx_coin_ledger_user_time` | 索引 | `user_id`、`created_at DESC`、`id DESC` | 按时间分页查看用户的扣款流水。 |

- `ck_coin_ledger_coin_count` 检查约束。单次投币数范围为 1 至 2。
- 同一次投币的用户余额、视频互动汇总和流水记录在同一事务内写入。

### `favorite_folder` 用户收藏夹表

保存收藏夹名称、公开状态、默认标记和收藏数量。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 收藏夹主键 ID | — |
| `user_id` | `BIGINT` | 无 | 否 | 收藏夹所属用户 ID | `user.id` |
| `title` | `VARCHAR(64)` | 无 | 否 | 收藏夹名称 | — |
| `is_default` | `TINYINT` | `0` | 否 | 默认标记（0 普通收藏夹，1 默认收藏夹） | — |
| `is_public` | `TINYINT` | `0` | 否 | 公开状态（0 私密，1 公开） | — |
| `item_count` | `INT` | `0` | 否 | 收藏夹内的视频数量 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 创建时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_favorite_folder_default` | 唯一索引 | `(CASE WHEN is_default = 1 THEN user_id END)` | 同一用户最多拥有一个默认收藏夹。 |
| `idx_favorite_folder_user` | 索引 | `user_id`、`created_at DESC`、`id DESC` | 按时间读取用户的收藏夹。 |

- 默认收藏夹唯一索引仅对默认记录使用用户 ID，普通收藏夹的索引表达式值为 `NULL`。

### `favorite_item` 收藏夹视频记录表

保存每个收藏夹内的视频及收藏时间。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 收藏记录主键 ID | — |
| `folder_id` | `BIGINT` | 无 | 否 | 所属收藏夹 ID | `favorite_folder.id` |
| `user_id` | `BIGINT` | 无 | 否 | 收藏用户 ID，与收藏夹所属用户一致 | `user.id` |
| `video_id` | `BIGINT` | 无 | 否 | 收藏的视频 ID | `video.id` |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 收藏时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_favorite_item_pair` | 唯一索引 | `folder_id`、`video_id` | 同一视频在同一收藏夹内只保留一条记录。 |
| `idx_favorite_item_folder_time` | 索引 | `folder_id`、`created_at DESC`、`id DESC` | 分页查询收藏夹内的视频。 |
| `idx_favorite_item_user_video` | 索引 | `user_id`、`video_id`、`folder_id` | 查询用户将指定视频加入的收藏夹。 |

### `watch_history` 视频观看历史表

保存用户对每个视频最近观看的选集和进度，支持续播。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 观看历史主键 ID | — |
| `user_id` | `BIGINT` | 无 | 否 | 观看者用户 ID | `user.id` |
| `video_id` | `BIGINT` | 无 | 否 | 观看的视频 ID | `video.id` |
| `part_id` | `BIGINT` | 无 | 否 | 最后观看的分 P ID | `video_part.id` |
| `progress_sec` | `INT` | `0` | 否 | 最后一次观看进度，单位为秒 | — |
| `watched_at` | `DATETIME(6)` | 无 | 否 | 最近一次观看时间，精确到微秒以比较更新与删除顺序 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_watch_history_pair` | 唯一索引 | `user_id`、`video_id` | 每位用户对每个视频只保留一条观看历史。 |
| `idx_watch_history_user_time` | 索引 | `user_id`、`watched_at DESC`、`id DESC` | 按观看时间分页读取用户的观看历史。 |

## social 模块

### `user_follow` 用户关注关系表

保存用户之间的关注关系，支持关注列表、粉丝列表和互关判定。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 关注记录主键 ID | — |
| `follower_id` | `BIGINT` | 无 | 否 | 关注发起者用户 ID | `user.id` |
| `followee_id` | `BIGINT` | 无 | 否 | 被关注者用户 ID | `user.id` |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 建立关注的时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_user_follow_pair` | 唯一索引 | `follower_id`、`followee_id` | 限制同一用户对同一对象的关注关系只保存一次，并支持查询关注状态。 |
| `idx_user_follow_following` | 索引 | `follower_id`、`created_at DESC`、`id DESC` | 按时间分页查询关注列表。 |
| `idx_user_follow_followers` | 索引 | `followee_id`、`created_at DESC`、`id DESC` | 按时间分页查询粉丝列表。 |

### `dynamic_post` 用户动态内容表

保存视频投稿动态和用户手动发布的图文动态。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 动态主键 ID | — |
| `user_id` | `BIGINT` | 无 | 否 | 动态发布者用户 ID | `user.id` |
| `dynamic_type` | `TINYINT` | 无 | 否 | 动态类型（1 视频投稿，2 图文动态） | — |
| `video_id` | `BIGINT` | `NULL` | 是 | 投稿动态对应的视频 ID，图文动态时为空 | `video.id` |
| `content` | `TEXT` | `NULL` | 是 | 动态文案，未填写时为空 | — |
| `image_urls` | `JSON` | `NULL` | 是 | 配图访问地址数组，未添加配图时为空 | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 动态发布时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `uk_dynamic_post_video` | 唯一索引 | `video_id` | 每个视频仅生成一条投稿动态，多条图文动态可以同时留空。 |
| `idx_dynamic_post_user_time` | 索引 | `user_id`、`created_at DESC`、`id DESC` | 分页读取指定用户的动态，供读扩散和个人空间使用。 |

### `dynamic_inbox` 关注动态收件箱表

保存需要写扩散的动态与接收者之间的关系。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `receiver_id` | `BIGINT` | 无 | 否 | 动态接收者用户 ID | `user.id` |
| `dynamic_id` | `BIGINT` | 无 | 否 | 分发的动态 ID | `dynamic_post.id` |
| `published_at` | `DATETIME` | 无 | 否 | 原动态发布时间，取自动态内容记录 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `receiver_id`、`dynamic_id` | 唯一标识记录。 |
| `idx_dynamic_inbox_receiver_time` | 索引 | `receiver_id`、`published_at DESC`、`dynamic_id DESC` | 按原动态发布时间和动态 ID 分页，与读扩散结果归并。 |

- 联合主键保证同一动态只进入同一用户的收件箱一次。

### `notification` 用户站内消息表

保存评论回复、点赞、关注和系统通知及其已读状态。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 通知主键 ID，由异步写入事件携带 | — |
| `receiver_id` | `BIGINT` | 无 | 否 | 接收通知的用户 ID | `user.id` |
| `sender_id` | `BIGINT` | `0` | 否 | 触发通知的用户 ID，系统通知取 0 | `user.id` |
| `notify_type` | `TINYINT` | 无 | 否 | 通知分类（1 评论回复，2 收到点赞，3 新增粉丝，4 系统通知） | — |
| `target_type` | `TINYINT` | `0` | 否 | 关联对象类型（0 无目标，1 视频，2 评论） | — |
| `target_id` | `BIGINT` | `NULL` | 是 | 关联的业务对象 ID，无关联对象时为空 | `video.id` 或 `comment.id`，取决于 `target_type` |
| `content` | `VARCHAR(255)` | 无 | 否 | 通知摘要 | — |
| `is_read` | `TINYINT` | `0` | 否 | 已读状态（0 未读，1 已读） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 通知创建时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `idx_notification_receiver_type_time` | 索引 | `receiver_id`、`notify_type`、`created_at DESC`、`id DESC` | 按通知分类分页读取用户消息。 |
| `idx_notification_receiver_unread` | 索引 | `receiver_id`、`is_read`、`notify_type` | 按分类统计未读通知，支持批量标记已读。 |

- 异步写入重试沿用同一条通知的 `id`。

## recommend 模块

### `user_video_behavior` 推荐用户行为记录表

保存推荐画像和协同过滤所需的用户视频行为。

| 字段 | 类型 | 默认值 | 可为空 | 业务说明 | 逻辑外键 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGINT` | 无 | 否 | 行为主键 ID，由行为事件携带 | — |
| `user_id` | `BIGINT` | 无 | 否 | 触发行为的用户 ID | `user.id` |
| `video_id` | `BIGINT` | 无 | 否 | 关联的视频 ID | `video.id` |
| `behavior_type` | `TINYINT` | 无 | 否 | 行为类型（1 有效播放，2 点赞，3 收藏，4 投币，5 转发，6 不感兴趣） | — |
| `weight` | `INT` | 无 | 否 | 行为权重（播放加 1，点赞加 3，收藏加 4，投币加 5，转发加 4，不感兴趣减 5） | — |
| `created_at` | `DATETIME` | `CURRENT_TIMESTAMP` | 否 | 行为发生时间 | — |

| 索引名称 | 类型 | 字段及顺序 | 用途 |
| --- | --- | --- | --- |
| `PRIMARY` | 主键 | `id` | 唯一标识记录。 |
| `idx_user_video_behavior_time` | 索引 | `created_at`、`id` | 扫描指定时间范围内的行为，供协同过滤计算使用。 |
| `idx_user_video_behavior_user_time` | 索引 | `user_id`、`created_at DESC`、`id DESC` | 按用户和时间范围读取行为，供用户画像重建。 |

- 独立发生的每次行为使用各自的 `id`，同一行为事件重试时继续使用原 `id`。
