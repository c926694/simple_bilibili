package social

import "context"

type UserReader interface {
	NicknamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}

type VideoReader interface {
	TitlesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}
