package mq

const (
	VideoTranscodeTopic   = "video_transcode_topic"
	VideoStatSyncTopic    = "video_stat_sync_topic"
	DanmakuPersistTopic   = "danmaku_persist_topic"
	UserBehaviorTopic     = "user_behavior_topic"
	WatchHistorySyncTopic = "watch_history_sync_topic"
	DynamicFanoutTopic    = "dynamic_fanout_topic"
	NotificationTopic     = "notification_topic"
)

const (
	VideoTranscodeProducerGroup   = "simple_bilibili_transcode_producer"
	VideoTranscodeConsumerGroup   = "simple_bilibili_transcode_consumer"
	VideoStatSyncProducerGroup    = "simple_bilibili_video_stat_sync_producer"
	VideoStatSyncConsumerGroup    = "simple_bilibili_video_stat_sync_consumer"
	DanmakuPersistProducerGroup   = "simple_bilibili_danmaku_persist_producer"
	DanmakuPersistConsumerGroup   = "simple_bilibili_danmaku_persist_consumer"
	UserBehaviorProducerGroup     = "simple_bilibili_user_behavior_producer"
	UserBehaviorConsumerGroup     = "simple_bilibili_user_behavior_consumer"
	WatchHistorySyncProducerGroup = "simple_bilibili_watch_history_sync_producer"
	WatchHistorySyncConsumerGroup = "simple_bilibili_watch_history_sync_consumer"
	DynamicFanoutProducerGroup    = "simple_bilibili_dynamic_fanout_producer"
	DynamicFanoutConsumerGroup    = "simple_bilibili_dynamic_fanout_consumer"
	NotificationProducerGroup     = "simple_bilibili_notification_producer"
	NotificationConsumerGroup     = "simple_bilibili_notification_consumer"
)
