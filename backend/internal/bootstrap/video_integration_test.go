package bootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	miniogo "github.com/minio/minio-go/v7"

	"simple_bilibili/backend/internal/config"
)

const videosURL = "/api/v1/videos"

type integrationVideo struct {
	ID          string     `json:"id"`
	BVID        string     `json:"bv_id"`
	Title       string     `json:"title"`
	CoverURL    string     `json:"cover_url"`
	Status      int        `json:"status"`
	Duration    int        `json:"duration_sec"`
	PublishedAt *time.Time `json:"published_at"`
	Tags        []string   `json:"tags"`
	Author      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"author"`
	Parts []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Index   int    `json:"part_index"`
		Status  int    `json:"transcode_status"`
		HLS1080 string `json:"hls_1080p_url"`
		HLS720  string `json:"hls_720p_url"`
		HLS480  string `json:"hls_480p_url"`
	} `json:"parts"`
}

type integrationVideoPage struct {
	Items []integrationVideo `json:"items"`
	Total int                `json:"total"`
}

func TestVideoIntegration(t *testing.T) {
	if os.Getenv("BILIBILI_VIDEO_INTEGRATION") != "1" {
		t.Skip("设置 BILIBILI_VIDEO_INTEGRATION=1 以使用真实 FFmpeg 与容器")
	}
	for _, key := range []string{"MYSQL_ADDR", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_DATABASE", "REDIS_ADDR", "REDIS_USERNAME",
		"REDIS_PASSWORD", "REDIS_DB", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_SECURE", "ROCKETMQ_NAME_SERVERS", "FFMPEG_BINARY", "FFMPEG_PROBE_BINARY"} {
		if value, ok := os.LookupEnv("BILIBILI_USER_TEST_" + key); ok {
			t.Setenv("BILIBILI_"+key, value)
		}
	}
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.MySQL.Database, "_test") {
		t.Fatal("视频集成测试要求使用测试数据库")
	}
	nonce := strings.ToLower(rand.Text())
	cfg.App.TrustedProxies = []string{"127.0.0.1/32"}
	cfg.MinIO.Bucket = "bilibili-video-test-" + nonce
	cfg.MinIO.PublicEndpoint = testUploadProxy(t, cfg.MinIO)
	cfg.MinIO.PublicSecure = false
	cfg.FFmpeg.WorkDir = filepath.Join(t.TempDir(), "transcode")
	cfg.FFmpeg.TaskTimeout = 40 * time.Second
	probe, err := exec.LookPath(cfg.FFmpeg.ProbeBinary)
	if err != nil {
		t.Fatal(err)
	}
	cfg.FFmpeg.ProbeBinary = filepath.Join(t.TempDir(), "ffprobe")
	if err := os.Symlink(probe, cfg.FFmpeg.ProbeBinary); err != nil {
		t.Fatal(err)
	}
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Error(err)
		}
	})
	objects := cleanAvatarTestBucket(t, cfg.MinIO)
	var userID string
	var codes []string
	c := userIntegration{client: &http.Client{Timeout: 10 * time.Second}, ip: "2001:db8:face::4"}
	tagName := "tag-" + nonce
	t.Cleanup(func() {
		if userID == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cleanTestCheckin(t, ctx, app, userID)
		type cleanupVideo struct {
			ID   int64
			BVID string `db:"bv_id"`
		}
		var rows []cleanupVideo
		if err := app.db.SelectContext(ctx, &rows, "SELECT id, bv_id FROM video WHERE author_id = ?", userID); err != nil {
			t.Error(err)
		}
		for _, row := range rows {
			for _, table := range []string{"video_part", "video_stat", "video_tag"} {
				if _, err := app.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE video_id = ?", row.ID); err != nil {
					t.Error(err)
				}
			}
			if _, err := app.db.ExecContext(ctx, "DELETE FROM video WHERE id = ?", row.ID); err != nil {
				t.Error(err)
			}
			codes = append(codes, row.BVID)
		}
		for _, statement := range []struct {
			query string
			arg   any
		}{
			{"DELETE FROM upload_task WHERE user_id = ?", userID},
			{"DELETE FROM tag WHERE name = ?", tagName},
			{"DELETE FROM user WHERE id = ?", userID},
		} {
			if _, err := app.db.ExecContext(ctx, statement.query, statement.arg); err != nil {
				t.Error(err)
			}
		}
		keys := []string{"user:refresh:" + userID, "rate:auth:register:" + c.ip}
		for _, code := range codes {
			key := "video:detail:{" + code + "}"
			keys = append(keys, key, key+":generation")
		}
		if err := app.redis.Del(ctx, keys...).Err(); err != nil {
			t.Error(err)
		}
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- app.serveAPI(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
		if dirs, err := os.ReadDir(cfg.FFmpeg.WorkDir); err != nil || len(dirs) != 0 {
			t.Errorf("停机后转码目录仍有内容: %v，%v", dirs, err)
		}
	})
	c.url = "http://" + listener.Addr().String()
	c.call(t, http.MethodGet, "/healthz", "", nil, 0, nil)

	username := "video_" + nonce[:16]
	var registered struct {
		userTokens
		User integrationProfile `json:"user"`
	}
	c.call(t, http.MethodPost, authURL+"/register", "", map[string]string{"username": username, "password": "1", "nickname": "稿件 %_!"}, 0, &registered)
	userID = registered.User.ID
	access := registered.Access
	if userID == "" {
		t.Fatal("注册未返回用户 ID")
	}
	var categories []struct {
		ID        int
		Name      string
		SortOrder int `json:"sort_order"`
	}
	c.call(t, http.MethodGet, "/api/v1/categories", "", nil, 0, &categories)
	if len(categories) != 15 || categories[0].Name != "动画" || categories[14].ID != 15 || categories[14].SortOrder != 15 {
		t.Fatalf("分区种子数据不符: %v", categories)
	}

	low := videoSample(t, cfg.FFmpeg.Binary, "320x180", false)
	uploaded := c.uploadVideoSample(t, access, "sample.mp4", low)
	title := "测试稿件 %_! " + nonce
	create := map[string]any{"title": title, "description": "用于验证同进程转码", "category_id": 1,
		"tags": []string{tagName, tagName}, "upload_task_id": uploaded}
	c.call(t, http.MethodPost, videosURL, access, map[string]any{
		"title": title, "category_id": 1, "upload_task_id": uploaded, "parts": []map[string]string{{"upload_task_id": uploaded, "title": title}},
	}, 10001, nil)
	var created integrationVideo
	c.call(t, http.MethodPost, videosURL, access, create, 0, &created)
	if created.Status != 1 || len(created.Parts) != 1 || created.Parts[0].Title != title || !strings.HasPrefix(created.BVID, "BV") {
		t.Fatalf("投稿响应不符: %+v", created)
	}
	full := c.waitVideoState(t, created.BVID, "", 3)
	first := full.Parts[0]
	if first.HLS480 == "" || first.HLS720 != "" || first.HLS1080 != "" || full.Duration < 1 || full.PublishedAt == nil ||
		full.Author.ID != userID || full.Author.Username != username || len(full.Tags) != 1 || full.Tags[0] != tagName {
		t.Fatalf("已发布稿件详情不符: %+v", full)
	}
	checkVideoObject(t, cfg.MinIO, objects, full.CoverURL, "image/jpeg")
	playlist := checkVideoObject(t, cfg.MinIO, objects, first.HLS480, "application/vnd.apple.mpegurl")
	if !strings.Contains(playlist, "#EXTM3U") || !strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Fatal("HLS 播放列表不完整")
	}
	checkVideoObject(t, cfg.MinIO, objects, strings.TrimSuffix(first.HLS480, "index.m3u8")+"seg_00000.ts", "video/mp2t")
	if err := app.redis.Get(t.Context(), "video:detail:{"+full.BVID+"}").Err(); err != nil {
		t.Fatalf("已发布详情未写入缓存: %v", err)
	}
	c.call(t, http.MethodGet, videosURL+"/"+full.BVID, "", nil, 0, nil)

	instantID := c.uploadVideoSample(t, access, "same.mp4", low)
	delete(create, "upload_task_id")
	create["parts"] = []map[string]string{{"upload_task_id": instantID, "title": "复用选集"}}
	var reused integrationVideo
	create["title"] = "复用 " + nonce
	c.call(t, http.MethodPost, videosURL, access, create, 0, &reused)
	if reused.Status != 3 || reused.Parts[0].HLS480 != first.HLS480 || reused.Duration != full.Duration {
		t.Fatalf("同文件未复用转码结果: %+v", reused)
	}
	var found struct {
		Videos integrationVideoPage
		Users  struct{ Items []integrationProfile }
	}
	c.call(t, http.MethodGet, videosURL+"/search?keyword="+url.QueryEscape("%_!"), "", nil, 0, &found)
	if found.Videos.Total != 1 || found.Videos.Items[0].BVID != full.BVID || len(found.Users.Items) != 1 ||
		found.Users.Items[0].ID != userID || found.Users.Items[0].CreatedAt == "" {
		t.Fatalf("搜索未按字面匹配关键词: %+v", found)
	}
	var authored integrationVideoPage
	c.call(t, http.MethodGet, "/api/v1/users/"+userID+"/videos?sort=play&page_size=1", "", nil, 0, &authored)
	if authored.Total != 2 || len(authored.Items) != 1 {
		t.Fatalf("作者视频分页不符: %+v", authored)
	}

	high := videoSample(t, cfg.FFmpeg.Binary, "1280x720", true)
	highID := c.uploadVideoSample(t, access, "second.mp4", high)
	if err := os.Remove(cfg.FFmpeg.ProbeBinary); err != nil {
		t.Fatal(err)
	}
	var edit integrationVideo
	c.call(t, http.MethodPut, videosURL+"/"+full.BVID, access, map[string]any{"title": "已编辑 " + nonce,
		"parts": []map[string]string{{"upload_task_id": highID, "title": "追加到首位"}, {"part_id": first.ID, "title": "移到次位"}}}, 0, &edit)
	if edit.Status != 1 || len(edit.Parts) != 2 || edit.Parts[1].ID != first.ID {
		t.Fatalf("追加并调整分 P 不符: %+v", edit)
	}
	c.call(t, http.MethodGet, videosURL+"/"+full.BVID, "", nil, 20005, nil)
	waitTranscodeAttempt(t, app, edit.Parts[0].ID, 1)
	if err := os.Symlink(probe, cfg.FFmpeg.ProbeBinary); err != nil {
		t.Fatal(err)
	}
	full = c.waitVideoState(t, full.BVID, "", 3)
	if full.Title != "已编辑 "+nonce || full.Parts[0].HLS720 == "" || full.Parts[0].HLS480 == "" || full.Parts[0].HLS1080 != "" || full.Parts[1].HLS480 != first.HLS480 {
		t.Fatalf("追加分 P 后的稿件不符: %+v", full)
	}
	c.call(t, http.MethodPut, videosURL+"/"+full.BVID, access, map[string]any{"parts": []map[string]string{{"id": first.ID, "title": "只保留原视频"}}}, 0, &edit)
	if len(edit.Parts) != 1 || edit.Parts[0].Index != 1 || edit.Status != 3 {
		t.Fatalf("删除分 P 后的稿件不符: %+v", edit)
	}

	badID := c.uploadVideoSample(t, access, "broken.mp4", []byte("不可解码的源视频数据"))
	create["title"] = "失败测试 " + nonce
	create["parts"] = []map[string]string{{"upload_task_id": badID, "title": "损坏源文件"}}
	var broken integrationVideo
	c.call(t, http.MethodPost, videosURL, access, create, 0, &broken)
	broken = c.waitVideoState(t, broken.BVID, access, 4)
	codes = append(codes, broken.BVID)
	c.call(t, http.MethodGet, videosURL+"/"+broken.BVID, "", nil, 20005, nil)
	if len(broken.Parts) != 1 || broken.Parts[0].Status != 3 {
		t.Fatalf("损坏视频未标记失败: %+v", broken)
	}
	var oldJob, newJob int64
	if err := app.db.GetContext(t.Context(), &oldJob, "SELECT transcode_job_id FROM video_part WHERE id = ?", broken.Parts[0].ID); err != nil {
		t.Fatal(err)
	}
	c.call(t, http.MethodPost, videosURL+"/"+broken.BVID+"/parts/"+broken.Parts[0].ID+"/retry-transcode", access, nil, 0, &edit)
	if edit.Status != 1 {
		t.Fatalf("重试后稿件没有进入转码状态: %+v", edit)
	}
	broken = c.waitVideoState(t, broken.BVID, access, 4)
	if err := app.db.GetContext(t.Context(), &newJob, "SELECT transcode_job_id FROM video_part WHERE id = ?", broken.Parts[0].ID); err != nil {
		t.Fatal(err)
	}
	if newJob == oldJob || broken.Parts[0].Status != 3 {
		t.Fatal("失败分 P 未使用新任务重新处理")
	}

	c.call(t, http.MethodDelete, videosURL+"/"+full.BVID, access, nil, 0, nil)
	c.call(t, http.MethodGet, videosURL+"/"+full.BVID, "", nil, 20005, nil)
	var mine integrationVideoPage
	c.call(t, http.MethodGet, videosURL+"/mine", access, nil, 0, &mine)
	if mine.Total != 2 {
		t.Fatalf("本人列表仍含已下架稿件: %+v", mine)
	}
	for _, item := range mine.Items {
		if len(item.Tags) != 1 || item.Tags[0] != tagName || len(item.Parts) != 1 {
			t.Fatalf("本人稿件列表未返回编辑所需字段: %+v", item)
		}
	}
	c.call(t, http.MethodGet, videosURL+"/mine?status=5", access, nil, 0, &mine)
	if mine.Total != 1 || mine.Items[0].BVID != full.BVID {
		t.Fatalf("按状态查询稿件不符: %+v", mine)
	}
	// 验证投稿封面从 tmp/ 转正、更换封面清理旧封面以及定时清理过期上传任务。
	verifyCoverPromotionAndUploadCleanup(t, cfg, app, objects, c, access, userID, low)
}

func waitTranscodeAttempt(t *testing.T, app *App, partID string, want int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		var attempt int
		if err := app.db.GetContext(ctx, &attempt, "SELECT transcode_attempt FROM video_part WHERE id = ?", partID); err != nil {
			t.Fatal(err)
		}
		if attempt == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("分 P 重试次数为 %d，期望 %d", attempt, want)
		case <-ticker.C:
		}
	}
}

func videoSample(t *testing.T, binary, size string, audio bool) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.mp4")
	args := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=" + size + ":rate=12"}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-c:a", "aac")
	}
	args = append(args, "-t", "2", "-c:v", "libx264", "-threads", "2", "-pix_fmt", "yuv420p", path)
	if output, err := exec.CommandContext(t.Context(), binary, args...).CombinedOutput(); err != nil {
		t.Fatalf("生成测试视频: %v，%s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (c userIntegration) uploadVideoSample(t *testing.T, access, name string, data []byte) string {
	t.Helper()
	var upload videoTask
	c.call(t, http.MethodPost, uploadsURL+"/video/init", access, map[string]any{"file_name": name, "file_hash": fmt.Sprintf("%x", sha256.Sum256(data)), "file_size": len(data)}, 0, &upload)
	if !upload.Instant {
		path := uploadsURL + "/video/" + upload.ID
		var parts signedParts
		c.call(t, http.MethodPost, path+"/part-urls", access, map[string]any{"part_numbers": []int{1}}, 0, &parts)
		if len(parts.URLs) != 1 {
			t.Fatal("测试视频应只需要一个上传分片")
		}
		c.putPresigned(t, parts.URLs[0].URL, nil, data)
		c.call(t, http.MethodPost, path+"/complete", access, nil, 0, nil)
	}
	return upload.ID
}

func (c userIntegration) waitVideoState(t *testing.T, bvid, access string, status int) integrationVideo {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		resp, err := c.request(ctx, http.MethodGet, videosURL+"/"+bvid, access, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Code == 0 {
			var result integrationVideo
			if err := json.Unmarshal(resp.Data, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status == status {
				return result
			}
		} else if resp.Code != 20005 {
			t.Fatalf("读取转码状态返回 %d，%s", resp.Code, resp.Msg)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("稿件 %s 未进入状态 %d", bvid, status)
		case <-ticker.C:
		}
	}
}

func checkVideoObject(t *testing.T, cfg config.MinIO, objects *miniogo.Client, publicURL, contentType string) string {
	t.Helper()
	key, ok := strings.CutPrefix(publicURL, "/storage/")
	if !ok {
		t.Fatalf("媒体访问路径不符: %q", publicURL)
	}
	info, err := objects.StatObject(t.Context(), cfg.Bucket, key, miniogo.StatObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Size <= 0 || info.ContentType != contentType {
		t.Fatalf("对象属性不符: %+v", info)
	}
	object, err := objects.GetObject(t.Context(), cfg.Bucket, key, miniogo.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	var body bytes.Buffer
	if _, err := io.Copy(&body, object); err != nil {
		t.Fatal(err)
	}
	return body.String()
}

func verifyCoverPromotionAndUploadCleanup(t *testing.T, cfg config.Config, app *App, objects *miniogo.Client, c userIntegration, access, userID string, sample []byte) {
	t.Helper()
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	var cover1 signedImage
	c.call(t, http.MethodPost, uploadsURL+"/image-presign", access, map[string]any{
		"biz_type": "cover", "content_type": "image/png", "file_size": pngBuf.Len(),
	}, 0, &cover1)
	c.putPresigned(t, cover1.UploadURL, cover1.Headers, pngBuf.Bytes())

	taskID := c.uploadVideoSample(t, access, "cover-test.mp4", sample)
	var created integrationVideo
	c.call(t, http.MethodPost, videosURL, access, map[string]any{
		"title":          "封面转正验证",
		"category_id":    1,
		"cover_url":      cover1.PublicURL,
		"upload_task_id": taskID,
	}, 0, &created)
	wantCover1 := strings.Replace(cover1.PublicURL, "/storage/tmp/", "/storage/", 1)
	if created.CoverURL != wantCover1 {
		t.Fatalf("封面未转正: got %q, want %q", created.CoverURL, wantCover1)
	}
	if _, err := objects.StatObject(t.Context(), cfg.MinIO.Bucket, cover1.ObjectKey, miniogo.StatObjectOptions{}); miniogo.ToErrorResponse(err).Code != "NoSuchKey" {
		t.Fatalf("临时封面对象未删除: %v", err)
	}
	checkVideoObject(t, cfg.MinIO, objects, created.CoverURL, "image/png")

	var cover2 signedImage
	c.call(t, http.MethodPost, uploadsURL+"/image-presign", access, map[string]any{
		"biz_type": "cover", "content_type": "image/png", "file_size": pngBuf.Len(),
	}, 0, &cover2)
	c.putPresigned(t, cover2.UploadURL, cover2.Headers, pngBuf.Bytes())
	var edited integrationVideo
	c.call(t, http.MethodPut, videosURL+"/"+created.BVID, access, map[string]any{
		"cover_url": cover2.PublicURL,
	}, 0, &edited)
	wantCover2 := strings.Replace(cover2.PublicURL, "/storage/tmp/", "/storage/", 1)
	if edited.CoverURL != wantCover2 {
		t.Fatalf("编辑封面未转正: got %q, want %q", edited.CoverURL, wantCover2)
	}
	if _, err := objects.StatObject(t.Context(), cfg.MinIO.Bucket, strings.TrimPrefix(wantCover1, "/storage/"), miniogo.StatObjectOptions{}); miniogo.ToErrorResponse(err).Code != "NoSuchKey" {
		t.Fatalf("更换封面后旧封面未删除: %v", err)
	}

	// 准备三个过期任务：1) 超期未完成 2) 超期未投稿原文件 3) 已有稿件引用的超期任务（不可删除）。
	var staleUploading videoTask
	c.call(t, http.MethodPost, uploadsURL+"/video/init", access, map[string]any{
		"file_name": "stale-uploading.mp4", "file_hash": fmt.Sprintf("%x", sha256.Sum256([]byte("stale-uploading-data"))), "file_size": 1024,
	}, 0, &staleUploading)

	orphanBytes := []byte("orphan-raw-video-content")
	orphanID := c.uploadVideoSample(t, access, "orphan.mp4", orphanBytes)
	var orphanKey string
	if err := app.db.GetContext(t.Context(), &orphanKey, "SELECT object_key FROM upload_task WHERE id = ?", orphanID); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().UTC().Add(-48 * time.Hour)
	for _, tid := range []string{staleUploading.ID, orphanID, taskID} {
		if _, err := app.db.ExecContext(t.Context(), "UPDATE upload_task SET updated_at = ? WHERE id = ?", oldTime, tid); err != nil {
			t.Fatal(err)
		}
	}

	if err := app.Modules.Upload.CleanupOnce(t.Context(), app.Modules.Video.ReferencedRawKeys); err != nil {
		t.Fatalf("执行定时清理失败: %v", err)
	}

	var staleStatus, orphanStatus, referencedStatus int
	if err := app.db.GetContext(t.Context(), &staleStatus, "SELECT status FROM upload_task WHERE id = ?", staleUploading.ID); err != nil {
		t.Fatal(err)
	}
	if err := app.db.GetContext(t.Context(), &orphanStatus, "SELECT status FROM upload_task WHERE id = ?", orphanID); err != nil {
		t.Fatal(err)
	}
	if err := app.db.GetContext(t.Context(), &referencedStatus, "SELECT status FROM upload_task WHERE id = ?", taskID); err != nil {
		t.Fatal(err)
	}
	if staleStatus != 2 || orphanStatus != 2 || referencedStatus != 1 {
		t.Fatalf("清理后任务状态不符: stale=%d orphan=%d referenced=%d", staleStatus, orphanStatus, referencedStatus)
	}
	if _, err := objects.StatObject(t.Context(), cfg.MinIO.Bucket, orphanKey, miniogo.StatObjectOptions{}); miniogo.ToErrorResponse(err).Code != "NoSuchKey" {
		t.Fatalf("未投稿的过期原视频未被删除: %v", err)
	}
	var referencedKey string
	if err := app.db.GetContext(t.Context(), &referencedKey, "SELECT object_key FROM upload_task WHERE id = ?", taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := objects.StatObject(t.Context(), cfg.MinIO.Bucket, referencedKey, miniogo.StatObjectOptions{}); err != nil {
		t.Fatalf("已被稿件引用的原始视频不应被删除: %v", err)
	}
}
