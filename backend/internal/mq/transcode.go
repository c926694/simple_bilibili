package mq

type VideoTranscodeTask struct {
	TaskID    int64  `json:"task_id,string"`
	UserID    int64  `json:"user_id,string"`
	ObjectKey string `json:"object_key"`
	FileHash  string `json:"file_hash"`
	FileSize  int64  `json:"file_size"`
}
