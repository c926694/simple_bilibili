package video

import "context"

type UserReader interface {
	NicknamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}
