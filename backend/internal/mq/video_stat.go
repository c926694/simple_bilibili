package mq

type VideoStatDelta struct {
	EventID       int64 `json:"event_id,string" db:"id"`
	VideoID       int64 `json:"video_id,string" db:"video_id"`
	PlayDelta     int64 `json:"play_delta"`
	DanmakuDelta  int64 `json:"danmaku_delta" db:"danmaku_delta"`
	LikeDelta     int64 `json:"like_delta"`
	CoinDelta     int64 `json:"coin_delta"`
	FavoriteDelta int64 `json:"favorite_delta"`
	ShareDelta    int64 `json:"share_delta"`
	CommentDelta  int64 `json:"comment_delta"`
}
