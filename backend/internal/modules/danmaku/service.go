package danmaku

import "simple_bilibili/backend/internal/modules/video"

type Service struct {
	videos *video.Service
}

func NewService(videos *video.Service) *Service {
	return &Service{videos: videos}
}
