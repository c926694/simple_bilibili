package recommend

import "context"

type UserReader interface {
	NicknamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}

type VideoReader interface {
	PublishedIDs(ctx context.Context, ids []int64) ([]int64, error)
}
