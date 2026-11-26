package user

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/media"
)

func (s *Service) verifyAvatar(ctx context.Context, userID int64, url string) error {
	prefix := fmt.Sprintf("/storage/avatars/%d/", userID)
	filename, owned := strings.CutPrefix(url, prefix)
	if !owned || filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\%?#") {
		return errcode.InvalidParams
	}
	key := strings.TrimPrefix(url, "/storage/")
	info, err := s.objects.StatObject(ctx, key)
	if errors.Is(err, minio.ErrObjectNotFound) {
		return errcode.InvalidParams
	}
	if err != nil {
		return err
	}
	if info.Size <= 0 || info.Size > media.MaxImageSize {
		return errcode.InvalidParams
	}
	header, err := s.objects.ReadObjectHeader(ctx, key, 512)
	if errors.Is(err, minio.ErrObjectNotFound) {
		return errcode.InvalidParams
	}
	if err != nil {
		return err
	}
	if media.ImageExtension(http.DetectContentType(header)) == "" {
		if err := s.objects.RemoveObject(ctx, key); err != nil {
			return err
		}
		return errcode.InvalidParams
	}
	return nil
}
