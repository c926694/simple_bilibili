package bootstrap

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/modules/danmaku"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/snowflake"
	"simple_bilibili/backend/internal/ws"
)

func TestDanmakuIntegration(t *testing.T) {
	if os.Getenv("BILIBILI_DANMAKU_INTEGRATION") != "1" {
		t.Skip("设置 BILIBILI_DANMAKU_INTEGRATION=1，以连接真实数据库、Redis、MinIO 与 RocketMQ")
	}
	for _, key := range []string{"MYSQL_ADDR", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_DATABASE", "REDIS_ADDR",
		"REDIS_USERNAME", "REDIS_PASSWORD", "REDIS_DB", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_SECURE", "ROCKETMQ_NAME_SERVERS"} {
		if value, ok := os.LookupEnv("BILIBILI_USER_TEST_" + key); ok {
			t.Setenv("BILIBILI_"+key, value)
		}
	}
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.MySQL.Database, "_test") {
		t.Fatal("弹幕集成测试要求使用测试数据库")
	}
	nonce := strings.ToLower(rand.Text())[:16]
	cfg.MinIO.Bucket = "bilibili-danmaku-test-" + nonce
	cfg.FFmpeg.WorkDir = filepath.Join(t.TempDir(), "transcode")
	cfg.App.TrustedProxies = []string{"127.0.0.1/32"}
	app, err := New(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Error(err)
		}
	})
	cleanAvatarTestBucket(t, cfg.MinIO)
	ids, err := snowflake.New(1023)
	if err != nil {
		t.Fatal(err)
	}
	videoID, partID, pendingID := ids.Next(), ids.Next(), ids.Next()
	bvid, pendingCode := "BV"+nonce, "BV"+nonce+"X"
	c := userIntegration{client: &http.Client{Timeout: 10 * time.Second}, ip: "2001:db8:ca::7"}
	var users []string
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, table := range []string{"danmaku", "video_stat_event", "danmaku_stat_outbox", "video_part", "video_stat"} {
			if _, err := app.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE video_id = ?", videoID); err != nil {
				t.Error(err)
			}
		}
		if _, err := app.db.ExecContext(ctx, "DELETE FROM video WHERE id IN (?, ?)", videoID, pendingID); err != nil {
			t.Error(err)
		}
		keys := []string{"rate:auth:register:" + c.ip}
		for _, id := range users {
			if _, err := app.db.ExecContext(ctx, "DELETE FROM user WHERE id = ?", id); err != nil {
				t.Error(err)
			}
			keys = append(keys, "user:refresh:"+id, "rate:user:danmaku:"+id)
		}
		cacheKey := "video:detail:{" + bvid + "}"
		keys = append(keys, cacheKey, cacheKey+":generation")
		for i := 0; i < 2; i++ {
			key := fmt.Sprintf("danmaku:{%d:%d}", partID, i)
			keys = append(keys, key+":history", key+":hot")
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
	serverDone := make(chan error, 1)
	go func() { serverDone <- app.serveAPI(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		if err := <-serverDone; err != nil {
			t.Error(err)
		}
	})
	c.url = "http://" + listener.Addr().String()
	c.call(t, http.MethodGet, "/healthz", "", nil, 0, nil)
	register := func(username string) userTokens {
		t.Helper()
		var result struct {
			userTokens
			User integrationProfile
		}
		c.call(t, http.MethodPost, authURL+"/register", "", map[string]string{"username": username, "password": "1"}, 0, &result)
		users = append(users, result.User.ID)
		return result.userTokens
	}
	first := register("dm1_" + nonce)
	second := register("dm2_" + nonce)
	authorID, err := strconv.ParseInt(users[0], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	// 真实数据库中的已发布稿件用于验证跨五分钟分段的查询。
	if _, err := app.db.ExecContext(ctx, "INSERT INTO video (id,bv_id,author_id,category_id,title,duration_sec,status,published_at) VALUES (?,?,?,?,?,310,3,CURRENT_TIMESTAMP), (?,?,?,?,?,0,1,NULL)",
		videoID, bvid, authorID, 1, "弹幕集成", pendingID, pendingCode, authorID, 1, "尚未发布"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(ctx, "INSERT INTO video_part (id,video_id,part_index,title,duration_sec,raw_object_key,transcode_status) VALUES (?,?,1,?,310,?,2)",
		partID, videoID, "第一集", "raw/danmaku-integration.mp4"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(ctx, "INSERT INTO video_stat (video_id) VALUES (?)", videoID); err != nil {
		t.Fatal(err)
	}

	wsURL := "ws" + strings.TrimPrefix(c.url, "http") + "/ws/videos/"
	for _, code := range []string{"BVunknown", pendingCode} {
		_, reply, err := websocket.DefaultDialer.DialContext(ctx, wsURL+code, nil)
		if reply != nil {
			reply.Body.Close()
		}
		if err == nil || reply == nil || reply.StatusCode != http.StatusNotFound {
			t.Fatalf("房间 %s 握手返回 %v，%v", code, reply, err)
		}
	}
	guest, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL+bvid, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer guest.Close()
	waitOnlineCount(t, guest, 1)

	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{ws.Protocol, "bearer." + first.Access}
	headers := http.Header{"Origin": []string{c.url}}
	viewer, _, err := dialer.DialContext(ctx, wsURL+bvid, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	if viewer.Subprotocol() != ws.Protocol {
		t.Fatalf("确认的子协议不符: %q", viewer.Subprotocol())
	}
	waitOnlineCount(t, viewer, 2)
	waitOnlineCount(t, guest, 2)
	if err := app.hub.SendUser(authorID, "user_notice", struct{}{}); err != nil {
		t.Fatal(err)
	}
	readDanmakuFrame(t, viewer, "user_notice", nil)
	if err := viewer.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		t.Fatal(err)
	}
	readDanmakuFrame(t, viewer, "pong", nil)

	path := videosURL + "/" + bvid + "/danmaku"
	part := strconv.FormatInt(partID, 10)
	historyPath := path + "?part_id=" + part
	var history struct {
		Segment  int               `json:"segment"`
		Duration int               `json:"segment_duration_ms"`
		Online   int               `json:"online_count"`
		Items    []danmaku.Message `json:"items"`
	}
	c.call(t, http.MethodGet, historyPath, "", nil, 0, &history)
	if history.Segment != 0 || history.Duration != 300000 || history.Online != 2 || history.Items == nil || len(history.Items) != 0 {
		t.Fatalf("初始历史段不符: %+v", history)
	}
	c.call(t, http.MethodGet, path+"?part_id=1", "", nil, 20007, nil)
	c.call(t, http.MethodGet, historyPath+"&segment=-1", "", nil, 10001, nil)
	var before struct {
		Stat struct {
			Danmaku int64 `json:"danmaku_count"`
		} `json:"stat"`
	}
	c.call(t, http.MethodGet, videosURL+"/"+bvid, "", nil, 0, &before)
	if before.Stat.Danmaku != 0 {
		t.Fatal("初始视频统计应为零")
	}

	input := map[string]any{"part_id": part, "content": "  即时弹幕  ", "time_offset_ms": 0}
	c.call(t, http.MethodPost, path, "", input, 10002, nil)
	var posted danmaku.Message
	withDanmakuReadLock(t, app.db.DB, func(conn *sql.Conn) {
		c.call(t, http.MethodPost, path, first.Access, input, 0, &posted)
		var count int64
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM danmaku WHERE id = ?", posted.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("此时弹幕尚未写入 MySQL")
		}
		c.call(t, http.MethodGet, historyPath, "", nil, 0, &history)
		if len(history.Items) != 1 || history.Items[0].ID != posted.ID || history.Items[0].Content != "即时弹幕" {
			t.Fatalf("发送后立即拉取失败: %+v", history)
		}
	})
	if posted.VideoID != videoID || posted.PartID != partID || posted.UserID != authorID || posted.Mode != 1 || posted.FontSize != 25 ||
		posted.Color != "#FFFFFF" || posted.CreatedAt.IsZero() {
		t.Fatalf("弹幕默认字段不符: %+v", posted)
	}
	for _, socket := range []*websocket.Conn{guest, viewer} {
		var received danmaku.Message
		readDanmakuFrame(t, socket, "danmaku", &received)
		if received.ID != posted.ID {
			t.Fatalf("广播收到其他弹幕: %+v", received)
		}
	}
	c.call(t, http.MethodPost, path, first.Access, map[string]any{"part_id": part, "content": "段末", "time_offset_ms": 299999, "mode": 2, "font_size": 18, "color": "#aabbcc"}, 0, nil)
	var segmentOne danmaku.Message
	c.call(t, http.MethodPost, path, first.Access, map[string]any{"part_id": part, "content": "下一段", "time_offset_ms": 300000}, 0, &segmentOne)
	c.call(t, http.MethodPost, path, first.Access, input, 10005, nil)
	c.call(t, http.MethodPost, path, second.Access, input, 0, nil)

	c.call(t, http.MethodGet, historyPath+"&segment=1", "", nil, 0, &history)
	if len(history.Items) != 1 || history.Items[0].ID != segmentOne.ID {
		t.Fatalf("分段边界不符: %+v", history)
	}
	c.call(t, http.MethodGet, historyPath, "", nil, 0, &history)
	if len(history.Items) != 3 || history.Items[0].ID != posted.ID || history.Items[1].TimeOffsetMS != 0 ||
		history.Items[2].TimeOffsetMS != 299999 || history.Items[2].Color != "#AABBCC" {
		t.Fatalf("排序或颜色标准化不符: %+v", history)
	}
	waitVideoStat(t, c, bvid, 4, 0, 0)
	if _, err := app.producer.PublishJSON(ctx, mq.DanmakuPersistTopic, posted); err != nil {
		t.Fatal(err)
	}
	c.call(t, http.MethodPost, path, second.Access, map[string]any{"part_id": part, "content": "重复消息之后", "time_offset_ms": 312000}, 0, nil)
	c.call(t, http.MethodPost, path, second.Access, map[string]any{"part_id": part, "content": "超过尾帧", "time_offset_ms": 312001}, 10001, nil)
	waitVideoStat(t, c, bvid, 5, 0, 0)
	var count int64
	if err := app.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM danmaku WHERE video_id = ?", videoID); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("弹幕入库数量为 %d，期望 5", count)
	}

	delta := mq.VideoStatDelta{EventID: ids.Next(), VideoID: videoID, LikeDelta: 2}
	for range 2 {
		if _, err := app.producer.PublishJSON(ctx, mq.VideoStatSyncTopic, delta); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.producer.PublishJSON(ctx, mq.VideoStatSyncTopic, mq.VideoStatDelta{EventID: ids.Next(), VideoID: videoID, ShareDelta: 1}); err != nil {
		t.Fatal(err)
	}
	waitVideoStat(t, c, bvid, 5, 2, 1)
	if err := viewer.Close(); err != nil {
		t.Fatal(err)
	}
	waitOnlineCount(t, guest, 1)
	c.call(t, http.MethodGet, historyPath, "", nil, 0, &history)
	if history.Online != 1 {
		t.Fatalf("断开后在线人数为 %d", history.Online)
	}
	cancel()
	if err := guest.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		if _, _, err := guest.ReadMessage(); err != nil {
			break
		}
	}
	if count := app.hub.OnlineCount(videoID); count != 0 {
		t.Fatalf("停机后在线人数为 %d", count)
	}
}

func withDanmakuReadLock(t *testing.T, db *sql.DB, run func(*sql.Conn)) {
	t.Helper()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), "LOCK TABLES danmaku READ"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(ctx, "UNLOCK TABLES"); err != nil {
			t.Error(err)
		}
	}()
	run(conn)
}

func readDanmakuFrame(t *testing.T, conn *websocket.Conn, kind string, data any) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		var packet struct {
			Type string
			Data json.RawMessage
		}
		if err := conn.ReadJSON(&packet); err != nil {
			t.Fatalf("等待 %s 帧: %v", kind, err)
		}
		if packet.Type != kind {
			continue
		}
		if data != nil {
			if err := json.Unmarshal(packet.Data, data); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
}

func waitOnlineCount(t *testing.T, conn *websocket.Conn, wanted int) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		var packet struct {
			Type string
			Data struct{ Count int }
		}
		if err := conn.ReadJSON(&packet); err != nil {
			t.Fatalf("等待在线人数 %d: %v", wanted, err)
		}
		if packet.Type == "online_count" && packet.Data.Count == wanted {
			return
		}
	}
}

func waitVideoStat(t *testing.T, c userIntegration, bvid string, danmakuCount, likes, shares int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		res, err := c.request(ctx, http.MethodGet, videosURL+"/"+bvid, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.Code != 0 {
			t.Fatalf("视频详情返回 %d，%s", res.Code, res.Msg)
		}
		var value struct {
			Stat struct {
				Danmaku int64 `json:"danmaku_count"`
				Likes   int64 `json:"like_count"`
				Shares  int64 `json:"share_count"`
			} `json:"stat"`
		}
		if err := json.Unmarshal(res.Data, &value); err != nil {
			t.Fatal(err)
		}
		if value.Stat.Danmaku == danmakuCount && value.Stat.Likes == likes && value.Stat.Shares == shares {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("视频统计为 %+v，期望弹幕 %d、点赞 %d、分享 %d", value.Stat, danmakuCount, likes, shares)
		case <-ticker.C:
		}
	}
}
