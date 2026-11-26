package comment

import (
	"simple_bilibili/backend/internal/modules/user"
	"simple_bilibili/backend/internal/modules/video"
)

type Service struct {
	users  *user.Service
	videos *video.Service
}

func NewService(users *user.Service, videos *video.Service) *Service {
	return &Service{users: users, videos: videos}
}
