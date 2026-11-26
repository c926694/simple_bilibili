package upload

import "simple_bilibili/backend/internal/infra/minio"

type imageInput struct {
	BizType     string `json:"biz_type"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
}

type imageResponse struct {
	UploadURL string            `json:"upload_url"`
	ObjectKey string            `json:"object_key"`
	PublicURL string            `json:"public_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresIn int               `json:"expires_in"`
}

type videoInput struct {
	FileHash    string `json:"file_hash"`
	FileSize    int64  `json:"file_size"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	extension   string
}

type initResponse struct {
	TaskID        int64                `json:"task_id,string"`
	Instant       bool                 `json:"instant"`
	Resumed       bool                 `json:"resumed"`
	ObjectKey     string               `json:"object_key"`
	ChunkSize     int                  `json:"chunk_size"`
	TotalChunks   int                  `json:"total_chunks"`
	UploadedParts []minio.UploadedPart `json:"uploaded_parts"`
}

type partURLInput struct {
	PartNumbers []int `json:"part_numbers"`
}

type partURL struct {
	PartNumber int    `json:"part_number"`
	UploadURL  string `json:"upload_url"`
}

type partURLResponse struct {
	URLs      []partURL `json:"urls"`
	ExpiresIn int       `json:"expires_in"`
}

type submittedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

type completeInput struct {
	Parts []submittedPart `json:"parts"`
}

type completeResponse struct {
	TaskID    int64  `json:"task_id,string"`
	ObjectKey string `json:"object_key"`
	Status    int    `json:"status"`
}

type detailResponse struct {
	TaskID        int64                `json:"task_id,string"`
	FileName      string               `json:"file_name"`
	FileSize      int64                `json:"file_size"`
	ChunkSize     int                  `json:"chunk_size"`
	TotalChunks   int                  `json:"total_chunks"`
	ObjectKey     string               `json:"object_key"`
	Status        int                  `json:"status"`
	UploadedParts []minio.UploadedPart `json:"uploaded_parts"`
}
