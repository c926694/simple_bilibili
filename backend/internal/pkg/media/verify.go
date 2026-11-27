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
	key, _, err := parseOwnedPath(url, fmt.Sprintf("/storage/%s/%d/", namespace, owner), namespace, owner)
	if err != nil {
		return err
	}
	return validateImageObject(ctx, objects, key)
}

func PromoteOwnedImage(ctx context.Context, objects *minio.Client, namespace string, owner int64, url string) (string, error) {
	srcKey, dstKey, err := parseOwnedPath(url, fmt.Sprintf("/storage/tmp/%s/%d/", namespace, owner), namespace, owner)
	if err != nil {
		return "", err
	}
	if err := validateImageObject(ctx, objects, srcKey); err != nil {
		return "", err
	}
	if err := objects.CopyObject(ctx, srcKey, dstKey); err != nil {
		if errors.Is(err, minio.ErrObjectNotFound) {
			return "", ErrInvalidImage
		}
		return "", err
	}
	if err := objects.RemoveObject(ctx, srcKey); err != nil && !errors.Is(err, minio.ErrObjectNotFound) {
		return "", err
	}
	return "/storage/" + dstKey, nil
}

func PublicImageKey(namespace, url string) (string, bool) {
	prefix := fmt.Sprintf("/storage/%s/", namespace)
	rest, ok := strings.CutPrefix(url, prefix)
	if !ok || rest == "" || strings.ContainsAny(rest, "\\?%#") {
		return "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[1] == "." || parts[1] == ".." {
		return "", false
	}
	return fmt.Sprintf("%s/%s/%s", namespace, parts[0], parts[1]), true
}

func parseOwnedPath(url, prefix, namespace string, owner int64) (string, string, error) {
	name, owned := strings.CutPrefix(url, prefix)
	if !owned || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\%?#") {
		return "", "", ErrInvalidImage
	}
	srcKey := strings.TrimPrefix(url, "/storage/")
	dstKey := fmt.Sprintf("%s/%d/%s", namespace, owner, name)
	return srcKey, dstKey, nil
}

func validateImageObject(ctx context.Context, objects *minio.Client, key string) error {
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
