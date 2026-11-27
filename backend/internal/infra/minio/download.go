package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func (c *Client) DownloadFile(ctx context.Context, objectKey, destPath string) error {
	if err := c.client.FGetObject(ctx, c.bucket, objectKey, destPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("下载 MinIO 对象 %q: %w", objectKey, storageError(err))
	}
	return nil
}
