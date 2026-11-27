package mq

type VideoTranscodeTask struct {
	VideoID      int64  `json:"video_id,string"`
	PartID       int64  `json:"part_id,string"`
	AuthorID     int64  `json:"author_id,string"`
	RawObjectKey string `json:"raw_object_key"`
	Attempt      int    `json:"attempt"`
	JobID        int64  `json:"job_id,string"`
}
