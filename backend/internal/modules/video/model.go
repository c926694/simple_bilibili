package video

import "time"

const (
	statusDraft       = 0
	statusTranscoding = 1
	statusReviewing   = 2
	statusPublished   = 3
	statusFailed      = 4
	statusWithdrawn   = 5

	partWaiting   = 0
	partRunning   = 1
	partSucceeded = 2
	partFailed    = 3
)

type video struct {
	ID          int64      `db:"id"`
	BVID        string     `db:"bv_id"`
	AuthorID    int64      `db:"author_id"`
	CategoryID  *int32     `db:"category_id"`
	Title       string     `db:"title"`
	Description string     `db:"description"`
	CoverURL    string     `db:"cover_url"`
	DurationSec int64      `db:"duration_sec"`
	Status      uint8      `db:"status"`
	PublishedAt *time.Time `db:"published_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

type Category struct {
	ID   int32  `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

type statistic struct {
	VideoID       int64   `db:"video_id" json:"-"`
	PlayCount     int64   `db:"play_count" json:"play_count"`
	DanmakuCount  int64   `db:"danmaku_count" json:"danmaku_count"`
	LikeCount     int64   `db:"like_count" json:"like_count"`
	CoinCount     int64   `db:"coin_count" json:"coin_count"`
	FavoriteCount int64   `db:"favorite_count" json:"favorite_count"`
	ShareCount    int64   `db:"share_count" json:"share_count"`
	CommentCount  int64   `db:"comment_count" json:"comment_count"`
	HotScore      float64 `db:"hot_score" json:"-"`
}

type part struct {
	ID         int64      `db:"id" json:"id,string"`
	VideoID    int64      `db:"video_id" json:"-"`
	Index      int        `db:"part_index" json:"part_index"`
	Title      string     `db:"title" json:"title"`
	Duration   int        `db:"duration_sec" json:"duration_sec"`
	RawKey     string     `db:"raw_object_key" json:"-"`
	HLS1080    *string    `db:"hls_1080p_url" json:"hls_1080p_url"`
	HLS720     *string    `db:"hls_720p_url" json:"hls_720p_url"`
	HLS480     *string    `db:"hls_480p_url" json:"hls_480p_url"`
	Status     uint8      `db:"transcode_status" json:"transcode_status"`
	JobID      int64      `db:"transcode_job_id" json:"-"`
	Attempt    int        `db:"transcode_attempt" json:"-"`
	DispatchAt *time.Time `db:"transcode_dispatch_at" json:"-"`
	LeaseUntil *time.Time `db:"transcode_lease_until" json:"-"`
}
