package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"simple_bilibili/backend/internal/infra/minio"
)

var ErrInvalidImage = errors.New("图片对象无效")

func VerifyOwnedImage(ctx context.Context, objects *minio.Client, namespace string, owner int64, url string) error {
	prefix := fmt.Sprintf("/storage/%s/%d/", namespace, owner)
	name, owned := strings.CutPrefix(url, prefix)
	if !owned || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\%?#") {
		return ErrInvalidImage
	}
	key := strings.TrimPrefix(url, "/storage/")
	info, err := objects.StatObject(ctx, key)
	if errors.Is(err, minio.ErrObjectNotFound) {
		return ErrInvalidImage
	}
	if err != nil {
		return err
	}
	if info.Size <= 0 || info.Size > MaxImageSize {
		return ErrInvalidImage
	}
	header, err := objects.ReadObjectHeader(ctx, key, 512)
	if errors.Is(err, minio.ErrObjectNotFound) {
		return ErrInvalidImage
	}
	if err != nil {
		return err
	}
	if ImageExtension(http.DetectContentType(header)) == "" {
		if err := objects.RemoveObject(ctx, key); err != nil {
			return err
		}
		return ErrInvalidImage
	}
	return nil
}
