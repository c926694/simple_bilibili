package mq

import "time"

type WatchHistoryEvent struct {
	ID        int64     `db:"id" json:"id,string"`
	UserID    int64     `db:"user_id" json:"user_id,string"`
	VideoID   int64     `db:"video_id" json:"video_id,string"`
	PartID    int64     `db:"part_id" json:"part_id,string"`
	Progress  int       `db:"progress_sec" json:"progress_sec"`
	WatchedAt time.Time `db:"watched_at" json:"watched_at"`
}
