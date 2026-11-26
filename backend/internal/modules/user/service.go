package user

import "context"

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) NicknamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := s.store.profilesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	nicknames := make(map[int64]string, len(rows))
	for _, row := range rows {
		nicknames[row.ID] = row.Nickname
	}
	return nicknames, nil
}
