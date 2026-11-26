package minio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
)

var (
	ErrObjectNotFound = errors.New("对象不存在")
	ErrUploadNotFound = errors.New("分片上传会话不存在")
)

type ObjectInfo = minio.ObjectInfo
type CompletePart = minio.CompletePart

type UploadedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

func (c *Client) PresignPutObject(ctx context.Context, objectKey, contentType string, expiry time.Duration) (string, error) {
	headers := http.Header{"Content-Type": {contentType}}
	u, err := c.signer.PresignHeader(ctx, http.MethodPut, c.bucket, objectKey, expiry, nil, headers)
	if err != nil {
		return "", fmt.Errorf("签发图片上传地址: %w", err)
	}
	return u.String(), nil
}

func (c *Client) PresignUploadPart(ctx context.Context, objectKey, uploadID string, partNumber int, expiry time.Duration) (string, error) {
	params := url.Values{"uploadId": {uploadID}, "partNumber": {strconv.Itoa(partNumber)}}
	u, err := c.signer.Presign(ctx, http.MethodPut, c.bucket, objectKey, expiry, params)
	if err != nil {
		return "", fmt.Errorf("签发视频分片地址: %w", err)
	}
	return u.String(), nil
}

func (c *Client) NewMultipartUpload(ctx context.Context, objectKey, contentType string) (string, error) {
	id, err := c.core.NewMultipartUpload(ctx, c.bucket, objectKey, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("初始化视频分片上传: %w", err)
	}
	return id, nil
}

func (c *Client) ListObjectParts(ctx context.Context, objectKey, uploadID string) ([]UploadedPart, error) {
	parts := make([]UploadedPart, 0)
	marker := 0
	for {
		page, err := c.core.ListObjectParts(ctx, c.bucket, objectKey, uploadID, marker, 1000)
		if err != nil {
			return nil, fmt.Errorf("读取已上传分片: %w", storageError(err))
		}
		for _, part := range page.ObjectParts {
			parts = append(parts, UploadedPart{part.PartNumber, part.ETag, part.Size})
		}
		if !page.IsTruncated {
			return parts, nil
		}
		marker = page.NextPartNumberMarker
	}
}

func (c *Client) CompleteMultipartUpload(ctx context.Context, objectKey, uploadID string, parts []CompletePart) error {
	_, err := c.core.CompleteMultipartUpload(ctx, c.bucket, objectKey, uploadID, parts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("合并视频分片: %w", storageError(err))
	}
	return nil
}

func (c *Client) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	if err := c.core.AbortMultipartUpload(ctx, c.bucket, objectKey, uploadID); err != nil {
		return fmt.Errorf("取消分片上传: %w", storageError(err))
	}
	return nil
}

func (c *Client) StatObject(ctx context.Context, objectKey string) (ObjectInfo, error) {
	info, err := c.client.StatObject(ctx, c.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("读取对象属性 %q: %w", objectKey, storageError(err))
	}
	return info, nil
}

func (c *Client) ReadObjectHeader(ctx context.Context, objectKey string, limit int64) ([]byte, error) {
	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(0, limit-1); err != nil {
		return nil, err
	}
	body, _, _, err := c.core.GetObject(ctx, c.bucket, objectKey, opts)
	if err != nil {
		return nil, fmt.Errorf("读取对象开头 %q: %w", objectKey, storageError(err))
	}
	defer body.Close()
	return io.ReadAll(io.LimitReader(body, limit))
}

func storageError(err error) error {
	switch minio.ToErrorResponse(err).Code {
	case "NoSuchKey", "NotFound":
		return fmt.Errorf("%w: %v", ErrObjectNotFound, err)
	case "NoSuchUpload":
		return fmt.Errorf("%w: %v", ErrUploadNotFound, err)
	}
	return err
}
