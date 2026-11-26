package interaction

type Service struct {
	videos VideoReader
}

func NewService(videos VideoReader) *Service {
	return &Service{videos: videos}
}
