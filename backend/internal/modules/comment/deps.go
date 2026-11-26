package comment

import "context"

type UserReader interface {
	NicknamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}

type VideoReader interface {
	AuthorsByIDs(ctx context.Context, ids []int64) (map[int64]int64, error)
}
