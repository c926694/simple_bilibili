package video

type video struct {
	ID       int64  `db:"id"`
	AuthorID int64  `db:"author_id"`
	Title    string `db:"title"`
	Status   uint8  `db:"status"`
}

const statusPublished = 3
