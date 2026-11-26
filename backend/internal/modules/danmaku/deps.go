package danmaku

import "context"

type VideoReader interface {
	PublishedIDs(ctx context.Context, ids []int64) ([]int64, error)
}
