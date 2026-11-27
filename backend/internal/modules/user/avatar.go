package user

import (
	"context"
	"errors"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/media"
)

func (s *Service) verifyAvatar(ctx context.Context, userID int64, url string) error {
	err := media.VerifyOwnedImage(ctx, s.objects, "avatars", userID, url)
	if errors.Is(err, media.ErrInvalidImage) {
		return errcode.InvalidParams
	}
	return err
}
