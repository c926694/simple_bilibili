package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/snowflake"
)

type interactionFixture struct {
	id   int64
	part int64
	code string
}

func TestInteractionIntegration(t *testing.T) {
	if os.Getenv("BILIBILI_INTERACTION_INTEGRATION") != "1" {
		t.Skip("设置 BILIBILI_INTERACTION_INTEGRATION=1 以使用真实容器")
	}
	for _, key := range []string{"MYSQL_ADDR", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_DATABASE", "REDIS_ADDR", "REDIS_USERNAME",
		"REDIS_PASSWORD", "REDIS_DB", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_SECURE", "ROCKETMQ_NAME_SERVERS"} {
		if value, ok := os.LookupEnv("BILIBILI_USER_TEST_" + key); ok {
			t.Setenv("BILIBILI_"+key, value)
		}
	}
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.MySQL.Database, "_test") {
		t.Fatal("互动测试要求独立的 MySQL 测试数据库")
	}
	nonce := strings.ToLower(rand.Text())[:16]
	cfg.MinIO.Bucket = "bilibili-interaction-test-" + nonce
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
	c := userIntegration{client: &http.Client{Timeout: 10 * time.Second}, ip: "2001:db8:face::9"}
	var users []string
	var videos []interactionFixture
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	month := time.Now().In(shanghai).Format("200601")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		keys := []string{"rate:auth:register:" + c.ip, "interaction:{play}:buffer", "interaction:{play}:pending",
			"interaction:{play}:generation", "interaction:{play}:events"}
		for _, v := range videos {
			for _, table := range []string{"interaction_stat_outbox", "video_stat_event", "watch_history", "coin_ledger",
				"favorite_item", "video_interaction", "video_part", "video_stat"} {
				if _, err := app.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE video_id = ?", v.id); err != nil {
					t.Error(err)
				}
			}
			if _, err := app.db.ExecContext(ctx, "DELETE FROM video WHERE id = ?", v.id); err != nil {
				t.Error(err)
			}
			cache := "video:detail:{" + v.code + "}"
			keys = append(keys, cache, cache+":generation", fmt.Sprintf("interaction:{play}:seen:%d:ip:%s", v.id, c.ip), fmt.Sprintf("interaction:share:%d:ip:%s", v.id, c.ip))
			for _, userID := range users {
				keys = append(keys, fmt.Sprintf("interaction:{play}:seen:%d:user:%s", v.id, userID), fmt.Sprintf("interaction:share:%d:user:%s", v.id, userID))
			}
		}
		for _, userID := range users {
			for _, table := range []string{"user_checkin", "favorite_folder"} {
				if _, err := app.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE user_id = ?", userID); err != nil {
					t.Error(err)
				}
			}
			if _, err := app.db.ExecContext(ctx, "DELETE FROM user WHERE id = ?", userID); err != nil {
				t.Error(err)
			}
			keys = append(keys, "user:refresh:"+userID, "watch:{"+userID+"}:hot", "watch:{"+userID+"}:deleted", "checkin:{"+userID+"}:"+month)
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
	})
	c.url = "http://" + listener.Addr().String()
	c.call(t, http.MethodGet, "/healthz", "", nil, 0, nil)
	register := func(name string) string {
		t.Helper()
		var result struct {
			userTokens
			User integrationProfile
		}
		c.call(t, http.MethodPost, authURL+"/register", "", map[string]string{"username": name + "_" + nonce, "password": "1"}, 0, &result)
		users = append(users, result.User.ID)
		return result.Access
	}
	author := register("intAuthor")
	viewer := register("intViewer")
	poor := register("intPoor")
	poorID, err := strconv.ParseInt(users[2], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if balance, err := app.Modules.User.ChangeCoins(ctx, poorID, -9); err != nil || balance != 1 {
		t.Fatalf("准备余额为 %d，错误 %v", balance, err)
	}
	ids, err := snowflake.New(1023)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		v := interactionFixture{id: ids.Next(), part: ids.Next(), code: fmt.Sprintf("BV%s%d", nonce, i)}
		videos = append(videos, v)
		if _, err := app.db.ExecContext(ctx, "INSERT INTO video (id,bv_id,author_id,category_id,title,duration_sec,status,published_at) VALUES (?,?,?,1,?,45,3,CURRENT_TIMESTAMP)",
			v.id, v.code, users[0], fmt.Sprintf("互动 %%_ 测试 %d", i)); err != nil {
			t.Fatal(err)
		}
		if _, err := app.db.ExecContext(ctx, "INSERT INTO video_part (id,video_id,part_index,title,duration_sec,raw_object_key,transcode_status) VALUES (?,?,1,'第一段',20,'raw/interaction.mp4',2)", v.part, v.id); err != nil {
			t.Fatal(err)
		}
		if _, err := app.db.ExecContext(ctx, "INSERT INTO video_stat (video_id) VALUES (?)", v.id); err != nil {
			t.Fatal(err)
		}
	}
	first, second, third := videos[0], videos[1], videos[2]
	firstPath := videosURL + "/" + first.code
	c.call(t, http.MethodPost, firstPath+"/coin", author, map[string]any{"count": 1, "request_id": "self"}, 40002, nil)
	checkConcurrentCodes(t, func(index int) (userEnvelope, error) {
		return c.request(ctx, http.MethodPost, firstPath+"/coin", viewer, map[string]any{"count": 2, "request_id": "viewer-" + strconv.Itoa(index)})
	}, 0, 40003)
	var requestID string
	if err := app.db.GetContext(ctx, &requestID, "SELECT request_id FROM coin_ledger WHERE user_id = ? AND video_id = ?", users[1], first.id); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Count   int `json:"coin_count"`
		Added   int `json:"added_coins"`
		Balance int `json:"coin_balance"`
	}
	c.call(t, http.MethodPost, firstPath+"/coin", viewer, map[string]any{"count": 2, "request_id": requestID}, 0, &receipt)
	if receipt.Count != 2 || receipt.Added != 2 || receipt.Balance != 8 {
		t.Fatalf("并发投币回执不符: %+v", receipt)
	}
	waitInteractionStat(t, c, first.code, "coin_count", 2)
	checkConcurrentCodes(t, func(index int) (userEnvelope, error) {
		return c.request(ctx, http.MethodPost, videosURL+"/"+videos[index].code+"/coin", poor, map[string]any{"count": 1, "request_id": "poor-" + strconv.Itoa(index)})
	}, 0, 40001)
	thirdPath := videosURL + "/" + third.code
	var triple struct {
		Liked     bool
		Favorited bool
		Reason    string   `json:"coin_skipped_reason"`
		Balance   int      `json:"coin_balance"`
		Added     int      `json:"added_coins"`
		Folders   []string `json:"favorite_folder_ids"`
	}
	c.call(t, http.MethodPost, thirdPath+"/triple", poor, map[string]string{"request_id": "poor-triple"}, 0, &triple)
	if !triple.Liked || !triple.Favorited || triple.Reason != "insufficient_coins" || triple.Balance != 0 || triple.Added != 0 || len(triple.Folders) != 1 {
		t.Fatalf("余额不足三连结果不符: %+v", triple)
	}
	waitInteractionStat(t, c, third.code, "like_count", 1)
	waitInteractionStat(t, c, third.code, "favorite_count", 1)
	c.call(t, http.MethodPost, thirdPath+"/triple", author, map[string]string{"request_id": "author-triple"}, 0, &triple)
	if !triple.Liked || !triple.Favorited || triple.Reason != "self_video" || triple.Added != 0 {
		t.Fatalf("自己的稿件三连结果不符: %+v", triple)
	}
	var liked struct{ Delta int }
	c.call(t, http.MethodPost, firstPath+"/like", viewer, nil, 0, &liked)
	if liked.Delta != 1 {
		t.Fatalf("点赞增量为 %d", liked.Delta)
	}
	c.call(t, http.MethodPost, firstPath+"/like", viewer, nil, 0, &liked)
	if liked.Delta != 0 {
		t.Fatalf("重复点赞增量为 %d", liked.Delta)
	}
	c.call(t, http.MethodDelete, firstPath+"/like", viewer, nil, 0, &liked)
	if liked.Delta != -1 {
		t.Fatalf("取消点赞增量为 %d", liked.Delta)
	}

	verifyFavoriteFolders(t, c, viewer, poor, users[1], first)
	verifyPlaybackCounts(t, c, app, viewer, users[1], first)
	verifyWatchHistory(t, c, app, ids, viewer, users[1], first, second)
	verifyInteractionCheckin(t, c, app, author, users[0], shanghai)
}

func checkConcurrentCodes(t *testing.T, run func(int) (userEnvelope, error), first, second int) {
	t.Helper()
	type result struct {
		code int
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i := range 2 {
		go func() {
			<-start
			res, err := run(i)
			results <- result{res.Code, err}
		}()
	}
	close(start)
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || !((a.code == first && b.code == second) || (a.code == second && b.code == first)) {
		t.Fatalf("并发返回 %v 与 %v，期望错误码 %d 和 %d", a, b, first, second)
	}
}

func verifyFavoriteFolders(t *testing.T, c userIntegration, viewer, other, owner string, v interactionFixture) {
	t.Helper()
	const path = "/api/v1/favorite-folders"
	type folder struct {
		ID      string
		Title   string
		Default bool `json:"is_default"`
		Public  bool `json:"is_public"`
		Count   int  `json:"item_count"`
	}
	var folders []folder
	c.call(t, http.MethodGet, path, viewer, nil, 0, &folders)
	if len(folders) != 1 || !folders[0].Default {
		t.Fatalf("默认收藏夹不符: %+v", folders)
	}
	c.call(t, http.MethodDelete, path+"/"+folders[0].ID, viewer, nil, 40005, nil)
	var private, public folder
	c.call(t, http.MethodPost, path, viewer, map[string]any{"title": "私密", "is_public": false}, 0, &private)
	c.call(t, http.MethodPost, path, viewer, map[string]any{"title": "计划公开", "is_public": false}, 0, &public)
	c.call(t, http.MethodPut, path+"/"+public.ID, viewer, map[string]any{"title": "公开", "is_public": true}, 0, &public)
	if !public.Public || public.Title != "公开" {
		t.Fatalf("更新后的收藏夹不符: %+v", public)
	}
	c.call(t, http.MethodGet, path+"?user_id="+owner, other, nil, 0, &folders)
	if len(folders) != 1 || folders[0].ID != public.ID {
		t.Fatalf("公开收藏夹列表不符: %+v", folders)
	}
	c.call(t, http.MethodGet, path+"/"+private.ID+"/videos", other, nil, 10003, nil)

	p := videosURL + "/" + v.code + "/favorites"
	var favorites struct {
		Delta     int
		FolderIDs []string `json:"favorite_folder_ids"`
	}
	c.call(t, http.MethodPut, p, viewer, map[string]any{"folder_ids": []string{private.ID, public.ID}}, 0, &favorites)
	if favorites.Delta != 1 || len(favorites.FolderIDs) != 2 {
		t.Fatalf("新增多夹收藏返回 %+v", favorites)
	}
	waitInteractionStat(t, c, v.code, "favorite_count", 1)
	var listing struct {
		Folder folder
		Items  []integrationVideo
		Total  int
	}
	c.call(t, http.MethodGet, path+"/"+public.ID+"/videos?page=1", other, nil, 0, &listing)
	if listing.Folder.Count != 1 || listing.Total != 1 || len(listing.Items) != 1 || listing.Items[0].BVID != v.code {
		t.Fatalf("收藏夹内容不符: %+v", listing)
	}
	c.call(t, http.MethodDelete, path+"/"+private.ID, viewer, nil, 0, nil)
	var state struct {
		Favorited bool
		FolderIDs []string `json:"favorite_folder_ids"`
	}
	c.call(t, http.MethodGet, videosURL+"/"+v.code+"/interaction", viewer, nil, 0, &state)
	if !state.Favorited || len(state.FolderIDs) != 1 || state.FolderIDs[0] != public.ID {
		t.Fatalf("删除一个收藏夹后的状态不符: %+v", state)
	}
	waitInteractionStat(t, c, v.code, "favorite_count", 1)
	c.call(t, http.MethodDelete, path+"/"+public.ID, viewer, nil, 0, nil)
	waitInteractionStat(t, c, v.code, "favorite_count", 0)
}

func verifyPlaybackCounts(t *testing.T, c userIntegration, app *App, viewer, viewerID string, v interactionFixture) {
	t.Helper()
	path := videosURL + "/" + v.code
	var result struct{ Counted bool }
	for _, scenario := range []struct {
		suffix, access string
		wanted         bool
	}{
		{"/play", "", true}, {"/play", "", false}, {"/play", viewer, true}, {"/play", viewer, false},
		{"/share", "", true}, {"/share", "", false},
	} {
		c.call(t, http.MethodPost, path+scenario.suffix, scenario.access, nil, 0, &result)
		if result.Counted != scenario.wanted {
			t.Fatalf("%s 去重返回 %+v", scenario.suffix, result)
		}
	}
	key := fmt.Sprintf("interaction:{play}:seen:%d:user:%s", v.id, viewerID)
	ttl, err := app.redis.TTL(t.Context(), key).Result()
	if err != nil || ttl > 135*time.Second || ttl < 130*time.Second {
		t.Fatalf("45 秒视频去重期限为 %s，错误 %v", ttl, err)
	}
	waitInteractionStat(t, c, v.code, "play_count", 2)
	waitInteractionStat(t, c, v.code, "share_count", 1)
}

func verifyWatchHistory(t *testing.T, c userIntegration, app *App, ids *snowflake.Generator, access, userID string, first, second interactionFixture) {
	t.Helper()
	path := videosURL + "/" + first.code
	progress := map[string]any{"part_id": strconv.FormatInt(first.part, 10), "progress_sec": 999}
	var result struct {
		PartID   string `json:"part_id"`
		Progress int    `json:"progress_sec"`
	}
	tx, err := app.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var locked int64
	if err := tx.QueryRowContext(t.Context(), "SELECT id FROM user WHERE id = ? FOR UPDATE", userID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	c.call(t, http.MethodPost, path+"/progress", access, progress, 0, &result)
	if result.PartID != strconv.FormatInt(first.part, 10) || result.Progress != 20 {
		t.Fatalf("尾部进度夹紧返回 %+v", result)
	}
	var state struct {
		Progress struct {
			Seconds   int `json:"progress_sec"`
			PartIndex int `json:"part_index"`
		} `json:"watch_progress"`
	}
	c.call(t, http.MethodGet, path+"/interaction", access, nil, 0, &state)
	if state.Progress.Seconds != 20 || state.Progress.PartIndex != 1 {
		t.Fatalf("热进度返回 %+v", state)
	}
	var stored int64
	if err := app.db.GetContext(t.Context(), &stored, "SELECT COUNT(*) FROM watch_history WHERE user_id = ?", userID); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatal("读取热进度前，历史应仍未写入 MySQL")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	waitInteraction(t, func() bool {
		if err := app.db.GetContext(t.Context(), &stored, "SELECT COUNT(*) FROM watch_history WHERE user_id = ?", userID); err != nil {
			t.Fatal(err)
		}
		return stored == 1
	})
	var old mq.WatchHistoryEvent
	if err := app.db.GetContext(t.Context(), &old, "SELECT id,user_id,video_id,part_id,progress_sec,watched_at FROM watch_history WHERE user_id = ? AND video_id = ?", userID, first.id); err != nil {
		t.Fatal(err)
	}
	newer, older := old, old
	newer.ID, newer.Progress, newer.WatchedAt = ids.Next(), 12, time.Now().UTC().Truncate(time.Microsecond)
	older.ID, older.Progress, older.WatchedAt = ids.Next(), 4, newer.WatchedAt.Add(-time.Microsecond)
	if err := app.producer.PublishBatchJSON(t.Context(), mq.WatchHistorySyncTopic, []any{newer, older}); err != nil {
		t.Fatal(err)
	}
	waitInteraction(t, func() bool {
		var saved mq.WatchHistoryEvent
		if err := app.db.GetContext(t.Context(), &saved, "SELECT id,user_id,video_id,part_id,progress_sec,watched_at FROM watch_history WHERE user_id = ? AND video_id = ?", userID, first.id); err != nil {
			t.Fatal(err)
		}
		old = saved
		return saved.ID == newer.ID && saved.Progress == 12
	})
	var list struct {
		Items []struct {
			Video integrationVideo
			Title string `json:"part_title"`
		}
		Total int
	}
	c.call(t, http.MethodGet, "/api/v1/history?keyword="+url.QueryEscape("%_"), access, nil, 0, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Video.BVID != first.code || list.Items[0].Title != "第一段" {
		t.Fatalf("历史标题字面匹配或分 P 组装不符: %+v", list)
	}
	c.call(t, http.MethodDelete, "/api/v1/history/"+first.code, access, nil, 0, nil)
	c.call(t, http.MethodDelete, "/api/v1/history", access, nil, 0, nil)
	if _, err := app.producer.PublishJSON(t.Context(), mq.WatchHistorySyncTopic, old); err != nil {
		t.Fatal(err)
	}
	c.call(t, http.MethodPost, videosURL+"/"+second.code+"/progress", access, map[string]any{
		"part_id": strconv.FormatInt(second.part, 10), "progress_sec": 7,
	}, 0, nil)
	waitInteraction(t, func() bool {
		if err := app.db.GetContext(t.Context(), &stored, "SELECT COUNT(*) FROM watch_history WHERE user_id = ? AND video_id = ?", userID, second.id); err != nil {
			t.Fatal(err)
		}
		return stored == 1
	})
	if err := app.db.GetContext(t.Context(), &stored, "SELECT COUNT(*) FROM watch_history WHERE user_id = ? AND video_id = ?", userID, first.id); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatal("迟到事件恢复了已删除的观看历史")
	}
	c.call(t, http.MethodGet, "/api/v1/history", access, nil, 0, &list)
	if list.Total != 1 || list.Items[0].Video.BVID != second.code {
		t.Fatalf("清空后观看历史不符: %+v", list)
	}
}

func verifyInteractionCheckin(t *testing.T, c userIntegration, app *App, access, userID string, zone *time.Location) {
	t.Helper()
	const path = "/api/v1/users/me/checkin"
	checkConcurrentCodes(t, func(_ int) (userEnvelope, error) { return c.request(t.Context(), http.MethodPost, path, access, nil) }, 0, 40006)
	var result struct {
		Today   bool `json:"checked_in_today"`
		Month   string
		Dates   []int `json:"checked_dates"`
		Streak  int   `json:"streak_days"`
		Balance int   `json:"coin_balance"`
	}
	c.call(t, http.MethodGet, path, access, nil, 0, &result)
	now := time.Now().In(zone)
	if !result.Today || result.Month != now.Format("2006-01") || len(result.Dates) != 1 || result.Dates[0] != now.Day() || result.Streak != 1 || result.Balance != 11 {
		t.Fatalf("签到位图或奖励不符: %+v", result)
	}
	key := "checkin:{" + userID + "}:" + now.Format("200601")
	if err := app.redis.Del(t.Context(), key).Err(); err != nil {
		t.Fatal(err)
	}
	c.call(t, http.MethodGet, path, access, nil, 0, &result)
	bit, err := app.redis.GetBit(t.Context(), key, int64(now.Day()-1)).Result()
	if err != nil || bit != 1 || !result.Today || result.Balance != 11 {
		t.Fatalf("从 MySQL 回填位图返回 %+v，位值 %d，错误 %v", result, bit, err)
	}
	c.call(t, http.MethodPost, path, access, nil, 40006, nil)
}

func waitInteractionStat(t *testing.T, c userIntegration, bvid, field string, wanted int64) {
	t.Helper()
	var last int64
	waitInteraction(t, func() bool {
		response, err := c.request(t.Context(), http.MethodGet, videosURL+"/"+bvid, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if response.Code != 0 {
			t.Fatalf("读取视频详情返回 %d", response.Code)
		}
		var payload struct{ Stat map[string]int64 }
		if err := json.Unmarshal(response.Data, &payload); err != nil {
			t.Fatal(err)
		}
		last = payload.Stat[field]
		return last == wanted
	})
}

func waitInteraction(t *testing.T, ready func() bool) {
	t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for !ready() {
		select {
		case <-timer.C:
			t.Fatal("等待异步互动结果超时")
		case <-ticker.C:
		}
	}
}
