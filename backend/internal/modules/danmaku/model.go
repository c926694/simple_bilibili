package danmaku

import "time"

const (
	segmentDurationMS = 300000
	hotRetention      = 5 * time.Minute
)

type Message struct {
	ID           int64     `db:"id" json:"id,string"`
	VideoID      int64     `db:"video_id" json:"video_id,string"`
	PartID       int64     `db:"part_id" json:"part_id,string"`
	UserID       int64     `db:"user_id" json:"user_id,string"`
	Content      string    `db:"content" json:"content"`
	TimeOffsetMS int64     `db:"time_offset_ms" json:"time_offset_ms"`
	Mode         uint8     `db:"mode" json:"mode"`
	FontSize     uint8     `db:"font_size" json:"font_size"`
	Color        string    `db:"color" json:"color"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}
