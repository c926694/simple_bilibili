package danmaku

type sendRequest struct {
	PartID       int64   `json:"part_id,string"`
	Content      string  `json:"content"`
	TimeOffsetMS *int64  `json:"time_offset_ms"`
	Mode         *uint8  `json:"mode"`
	FontSize     *uint8  `json:"font_size"`
	Color        *string `json:"color"`
}

type sendInput struct {
	PartID       int64
	Content      string
	TimeOffsetMS int64
	Mode         uint8
	FontSize     uint8
	Color        string
}

type historyResponse struct {
	Segment           int64     `json:"segment"`
	SegmentDurationMS int       `json:"segment_duration_ms"`
	OnlineCount       int       `json:"online_count"`
	Items             []Message `json:"items"`
}
