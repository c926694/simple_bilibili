package bootstrap

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"testing"

	miniogo "github.com/minio/minio-go/v7"

	"simple_bilibili/backend/internal/config"
)

const uploadsURL = "/api/v1/uploads"

type signedImage struct {
	UploadURL string            `json:"upload_url"`
	ObjectKey string            `json:"object_key"`
	PublicURL string            `json:"public_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresIn int               `json:"expires_in"`
}

type videoTask struct {
	ID            string `json:"task_id"`
	Instant       bool   `json:"instant"`
	Resumed       bool   `json:"resumed"`
	ObjectKey     string `json:"object_key"`
	ChunkSize     int    `json:"chunk_size"`
	TotalChunks   int    `json:"total_chunks"`
	Status        int    `json:"status"`
	UploadedParts []struct {
		PartNumber int    `json:"part_number"`
		ETag       string `json:"etag"`
		Size       int    `json:"size"`
	} `json:"uploaded_parts"`
}

type signedParts struct {
	URLs []struct {
		Number int    `json:"part_number"`
		URL    string `json:"upload_url"`
	} `json:"urls"`
	ExpiresIn int `json:"expires_in"`
}

func testUploadProxy(t *testing.T, cfg config.MinIO) string {
	t.Helper()
	scheme := "http"
	if cfg.Secure {
		scheme = "https"
	}
	target := &url.URL{Scheme: scheme, Host: cfg.Endpoint}
	// 透传浏览器请求的 Host，和部署中的预签名转发保持一致。
	proxy := httptest.NewServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	return strings.TrimPrefix(proxy.URL, "http://")
}

func (c userIntegration) verifyAvatarUpload(t *testing.T, cfg config.MinIO, objects *miniogo.Client, access, otherAccess, userID string) {
	t.Helper()
	var pngFile bytes.Buffer
	if err := png.Encode(&pngFile, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path := uploadsURL + "/image-presign"
	input := map[string]any{"biz_type": "avatar", "content_type": "image/png", "file_size": pngFile.Len()}
	c.call(t, http.MethodPost, path, "", input, 10002, nil)
	c.call(t, http.MethodPost, path, access, map[string]any{
		"biz_type": "avatar", "content_type": "image/png", "file_size": (5 << 20) + 1,
	}, 10001, nil)

	var signed signedImage
	c.call(t, http.MethodPost, path, access, input, 0, &signed)
	if signed.ExpiresIn != 300 || signed.Headers["Content-Type"] != "image/png" ||
		!strings.HasPrefix(signed.UploadURL, "http://"+cfg.PublicEndpoint+"/"+cfg.Bucket+"/") ||
		!strings.HasPrefix(signed.ObjectKey, "avatars/"+userID+"/") || signed.PublicURL != "/storage/"+signed.ObjectKey {
		t.Fatalf("头像预签名响应不符: %+v", signed)
	}
	update := map[string]string{"nickname": "新昵称", "avatar_url": signed.PublicURL}
	c.call(t, http.MethodPut, usersURL+"/me", access, update, 10001, nil)
	c.putPresigned(t, signed.UploadURL, signed.Headers, pngFile.Bytes())
	c.call(t, http.MethodPut, usersURL+"/me", otherAccess, update, 10001, nil)

	var saved integrationProfile
	c.call(t, http.MethodPut, usersURL+"/me", access, update, 0, &saved)
	if saved.AvatarURL != signed.PublicURL || saved.ID != userID {
		t.Fatalf("保存头像后的用户资料不符: %+v", saved)
	}
	for biz, prefix := range map[string]string{"cover": "covers", "dynamic": "dynamics"} {
		var extra signedImage
		c.call(t, http.MethodPost, path, access, map[string]any{
			"biz_type": biz, "content_type": "image/png", "file_size": pngFile.Len(),
		}, 0, &extra)
		if !strings.HasPrefix(extra.ObjectKey, prefix+"/"+userID+"/") {
			t.Fatalf("图片目标目录不符: %s", extra.ObjectKey)
		}
		c.putPresigned(t, extra.UploadURL, extra.Headers, pngFile.Bytes())
	}

	text := []byte("文件内容为文本")
	var forged signedImage
	c.call(t, http.MethodPost, path, access, map[string]any{
		"biz_type": "avatar", "content_type": "image/png", "file_size": len(text),
	}, 0, &forged)
	c.putPresigned(t, forged.UploadURL, forged.Headers, text)
	update["avatar_url"] = forged.PublicURL
	c.call(t, http.MethodPut, usersURL+"/me", access, update, 10001, nil)
	if _, err := objects.StatObject(t.Context(), cfg.Bucket, forged.ObjectKey, miniogo.StatObjectOptions{}); miniogo.ToErrorResponse(err).Code != "NoSuchKey" {
		t.Fatalf("核验失败的对象应已删除，查询返回 %v", err)
	}
	c.call(t, http.MethodGet, usersURL+"/me", access, nil, 0, &saved)
	if saved.AvatarURL != signed.PublicURL {
		t.Fatalf("数据库头像地址为 %q，期望 %q", saved.AvatarURL, signed.PublicURL)
	}
}

func (c userIntegration) verifyVideoUpload(t *testing.T, cfg config.Config, objects *miniogo.Client, access, otherAccess, userID string) {
	t.Helper()
	const chunkSize = 8 << 20
	payload := append(bytes.Repeat([]byte{0x61}, chunkSize), bytes.Repeat([]byte{0x62}, 1<<20)...)
	initPath := uploadsURL + "/video/init"
	input := map[string]any{
		"file_name": "direct.mp4", "content_type": "video/mp4",
		"file_hash": fmt.Sprintf("%x", sha256.Sum256(payload)), "file_size": len(payload),
	}
	var initialized videoTask
	c.call(t, http.MethodPost, initPath, access, input, 0, &initialized)
	id, err := strconv.ParseInt(initialized.ID, 10, 64)
	if err != nil || id <= 0 || initialized.Instant || initialized.Resumed || initialized.TotalChunks != 2 ||
		initialized.ChunkSize != chunkSize || initialized.UploadedParts == nil || len(initialized.UploadedParts) != 0 ||
		initialized.ObjectKey != "raw/"+userID+"/"+initialized.ID+".mp4" {
		t.Fatalf("视频初始化响应不符: %+v", initialized)
	}

	taskPath := uploadsURL + "/video/" + initialized.ID
	c.call(t, http.MethodGet, taskPath, otherAccess, nil, 10003, nil)
	c.call(t, http.MethodGet, uploadsURL+"/video/1", access, nil, 20001, nil)
	c.call(t, http.MethodPost, taskPath+"/part-urls", access, map[string]any{"part_numbers": []int{3}}, 10001, nil)
	c.call(t, http.MethodPost, taskPath+"/complete", access, nil, 20003, nil)
	var signed signedParts
	c.call(t, http.MethodPost, taskPath+"/part-urls", access, map[string]any{"part_numbers": []int{1, 2}}, 0, &signed)
	if signed.ExpiresIn != 900 || len(signed.URLs) != 2 || signed.URLs[0].Number != 1 || signed.URLs[1].Number != 2 {
		t.Fatalf("分片预签名响应不符: %+v", signed)
	}
	firstETag := c.putPresigned(t, signed.URLs[0].URL, nil, payload[:chunkSize])

	var resumed videoTask
	c.call(t, http.MethodPost, initPath, access, input, 0, &resumed)
	if !resumed.Resumed || resumed.Instant || resumed.ID != initialized.ID || len(resumed.UploadedParts) != 1 ||
		resumed.UploadedParts[0].PartNumber != 1 || resumed.UploadedParts[0].Size != chunkSize ||
		strings.Trim(resumed.UploadedParts[0].ETag, "\"") != strings.Trim(firstETag, "\"") {
		t.Fatalf("断点续传返回分片不符: %+v", resumed)
	}
	var detail videoTask
	c.call(t, http.MethodGet, taskPath, access, nil, 0, &detail)
	if detail.Status != 0 || len(detail.UploadedParts) != 1 {
		t.Fatalf("上传中任务详情不符: %+v", detail)
	}
	secondETag := c.putPresigned(t, signed.URLs[1].URL, nil, payload[chunkSize:])
	parts := []map[string]any{{"part_number": 2, "etag": secondETag}, {"part_number": 1, "etag": firstETag}}
	var complete videoTask
	c.call(t, http.MethodPost, taskPath+"/complete", access, map[string]any{"parts": parts}, 0, &complete)
	if complete.Status != 1 || complete.ObjectKey != initialized.ObjectKey || complete.ID != initialized.ID {
		t.Fatalf("视频合片响应不符: %+v", complete)
	}
	merged, err := objects.GetObject(t.Context(), cfg.MinIO.Bucket, initialized.ObjectKey, miniogo.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer merged.Close()
	got, err := io.ReadAll(merged)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("合并后的对象内容不符，大小 %d", len(got))
	}

	var instant videoTask
	c.call(t, http.MethodPost, initPath, otherAccess, input, 0, &instant)
	if !instant.Instant || instant.Resumed || instant.ID == initialized.ID || instant.ObjectKey != initialized.ObjectKey ||
		instant.TotalChunks != 0 || instant.ChunkSize != 0 || instant.UploadedParts == nil || len(instant.UploadedParts) != 0 {
		t.Fatalf("跨用户秒传响应不符: %+v", instant)
	}

	input["file_hash"] = fmt.Sprintf("%x", sha256.Sum256([]byte("abort-video")))
	var aborted videoTask
	c.call(t, http.MethodPost, initPath, access, input, 0, &aborted)
	abortPath := uploadsURL + "/video/" + aborted.ID
	c.call(t, http.MethodPost, abortPath+"/abort", access, nil, 0, nil)
	c.call(t, http.MethodGet, abortPath, access, nil, 0, &aborted)
	if aborted.Status != 2 {
		t.Fatalf("取消后的任务状态为 %d", aborted.Status)
	}
	c.call(t, http.MethodPost, abortPath+"/abort", access, nil, 20002, nil)
	input["file_name"] = "unsupported.bin"
	c.call(t, http.MethodPost, initPath, access, input, 20004, nil)
}

func (c userIntegration) putPresigned(t *testing.T, endpoint string, headers map[string]string, body []byte) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := c.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		message, err := io.ReadAll(res.Body)
		t.Fatalf("MinIO 直传返回 HTTP %d，响应 %q，读取错误 %v", res.StatusCode, message, err)
	}
	return res.Header.Get("ETag")
}
