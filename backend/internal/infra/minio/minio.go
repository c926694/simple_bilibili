package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"simple_bilibili/backend/internal/config"
)

type Client struct {
	client *minio.Client
	core   minio.Core
	signer *minio.Client
	bucket string
}

func Connect(ctx context.Context, cfg config.MinIO) (*Client, error) {
	creds := credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 MinIO 客户端: %w", err)
	}
	endpoint := cfg.PublicEndpoint
	if endpoint == "" {
		endpoint = cfg.Endpoint
	}
	signer, err := minio.New(endpoint, &minio.Options{
		Creds: creds, Secure: cfg.PublicSecure, BucketLookup: minio.BucketLookupPath, Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 MinIO 签名客户端: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("检查 MinIO 存储桶 %q: %w", cfg.Bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil &&
			minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
			return nil, fmt.Errorf("创建 MinIO 存储桶 %q: %w", cfg.Bucket, err)
		}
	}
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": [
				"arn:aws:s3:::%[1]s/avatars/*",
				"arn:aws:s3:::%[1]s/covers/*",
				"arn:aws:s3:::%[1]s/dynamics/*",
				"arn:aws:s3:::%[1]s/hls/*"
			]
		}]
	}`, cfg.Bucket)
	if err := client.SetBucketPolicy(ctx, cfg.Bucket, policy); err != nil {
		return nil, fmt.Errorf("设置 MinIO 存储桶 %q 的访问策略: %w", cfg.Bucket, err)
	}
	return &Client{client: client, core: minio.Core{Client: client}, signer: signer, bucket: cfg.Bucket}, nil
}

func (c *Client) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	if _, err := c.client.PutObject(ctx, c.bucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("上传 MinIO 对象 %q: %w", objectKey, err)
	}
	return nil
}

func (c *Client) RemoveObject(ctx context.Context, objectKey string) error {
	if err := c.client.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除 MinIO 对象 %q: %w", objectKey, err)
	}
	return nil
}
