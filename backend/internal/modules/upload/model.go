package upload

const (
	statusUploading = 0
	statusCompleted = 1
	statusCanceled  = 2
	chunkSize       = 8 << 20
	maxVideoSize    = 4 << 30
	maxVideoParts   = maxVideoSize / chunkSize
)

type task struct {
	ID          int64   `db:"id"`
	UploadID    *string `db:"upload_id"`
	UserID      int64   `db:"user_id"`
	FileHash    string  `db:"file_hash"`
	FileName    string  `db:"file_name"`
	FileSize    int64   `db:"file_size"`
	ChunkSize   int     `db:"chunk_size"`
	TotalChunks int     `db:"total_chunks"`
	ObjectKey   string  `db:"object_key"`
	Status      int     `db:"status"`
}
