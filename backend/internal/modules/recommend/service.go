package recommend

type Service struct {
	users  UserReader
	videos VideoReader
}

func NewService(users UserReader, videos VideoReader) *Service {
	return &Service{users: users, videos: videos}
}
