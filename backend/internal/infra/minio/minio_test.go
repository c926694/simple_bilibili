package minio

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"simple_bilibili/backend/internal/config"
)

func TestMinIOIntegration(t *testing.T) {
	endpoint := os.Getenv("BILIBILI_MINIO_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("设置 BILIBILI_MINIO_TEST_ENDPOINT 以连接真实 MinIO")
	}
	cfg := config.MinIO{
		Endpoint: endpoint, AccessKey: os.Getenv("BILIBILI_MINIO_TEST_ACCESS_KEY"),
		SecretKey: os.Getenv("BILIBILI_MINIO_TEST_SECRET_KEY"),
		Bucket:    "bilibili-test-" + strings.ToLower(rand.Text()),
	}
	client, err := Connect(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	var uploaded []string
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, key := range uploaded {
			if err := client.RemoveObject(ctx, key); err != nil {
				t.Error(err)
			}
		}
		if err := client.client.RemoveBucket(ctx, cfg.Bucket); err != nil {
			t.Error(err)
		}
	})
	if _, err := Connect(t.Context(), cfg); err != nil {
		t.Fatalf("重复初始化存储桶: %v", err)
	}

	const contents = "验证对象内容"
	base := "http://" + cfg.Endpoint + "/" + cfg.Bucket + "/"
	httpClient := &http.Client{Timeout: 5 * time.Second}
	for _, prefix := range []string{"avatars", "covers", "dynamics", "hls", "raw", "tmp"} {
		key := prefix + "/test.txt"
		if err := client.PutObject(t.Context(), key, strings.NewReader(contents), int64(len(contents)), "text/plain"); err != nil {
			t.Fatal(err)
		}
		uploaded = append(uploaded, key)
		res, err := httpClient.Get(base + key)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if prefix == "raw" || prefix == "tmp" {
			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("匿名读取私有对象 %s 返回 HTTP %d", prefix, res.StatusCode)
			}
		} else if res.StatusCode != http.StatusOK || string(body) != contents || res.Header.Get("Content-Type") != "text/plain" {
			t.Fatalf("%s 的匿名读取结果不符，HTTP %d", prefix, res.StatusCode)
		}
	}

	lc, err := client.client.GetBucketLifecycle(t.Context(), cfg.Bucket)
	if err != nil {
		t.Fatalf("读取存储桶生命周期规则: %v", err)
	}
	if len(lc.Rules) != 1 || lc.Rules[0].ID != "expire-tmp-uploads" {
		t.Fatalf("生命周期规则不符: %+v", lc.Rules)
	}

	if err := client.CopyObject(t.Context(), "tmp/test.txt", "covers/copied.txt"); err != nil {
		t.Fatalf("桶内复制对象失败: %v", err)
	}
	uploaded = append(uploaded, "covers/copied.txt")
	copiedRes, err := httpClient.Get(base + "covers/copied.txt")
	if err != nil {
		t.Fatal(err)
	}
	copiedBody, err := io.ReadAll(copiedRes.Body)
	copiedRes.Body.Close()
	if err != nil || copiedRes.StatusCode != http.StatusOK || string(copiedBody) != contents {
		t.Fatalf("复制后的对象读取结果不符，HTTP %d，内容 %q，错误 %v", copiedRes.StatusCode, copiedBody, err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, base+"avatars/test.txt", strings.NewReader("拒绝写入"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("匿名写入返回 HTTP %d", res.StatusCode)
	}

	if err := client.RemoveObject(t.Context(), uploaded[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := client.client.StatObject(t.Context(), cfg.Bucket, uploaded[0], minio.StatObjectOptions{}); minio.ToErrorResponse(err).Code != "NoSuchKey" {
		t.Fatalf("验证对象删除时返回 %v", err)
	}
}
