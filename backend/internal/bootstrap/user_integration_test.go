package bootstrap

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	mathrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/pkg/jwt"
)

type userIntegration struct {
	url    string
	client *http.Client
	ip     string
}

type userEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

type userTokens struct {
	Access           string `json:"access_token"`
	Refresh          string `json:"refresh_token"`
	AccessExpiresIn  int64  `json:"access_expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

type integrationProfile struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	AvatarURL   string  `json:"avatar_url"`
	CoinBalance int     `json:"coin_balance"`
	CreatedAt   string  `json:"created_at"`
}

const (
	authURL       = "/api/v1/auth"
	usersURL      = "/api/v1/users"
	defaultAvatar = "/avatars/default.webp"
)

func TestUserAuthenticationIntegration(t *testing.T) {
	if os.Getenv("BILIBILI_USER_INTEGRATION") != "1" {
		t.Skip("设置 BILIBILI_USER_INTEGRATION=1，并使用 BILIBILI_USER_TEST_* 指定已执行迁移的 MySQL 测试数据库和 Redis")
	}
	for _, key := range []string{"MYSQL_ADDR", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_DATABASE", "REDIS_ADDR", "REDIS_USERNAME", "REDIS_PASSWORD", "REDIS_DB"} {
		if value, ok := os.LookupEnv("BILIBILI_USER_TEST_" + key); ok {
			t.Setenv("BILIBILI_"+key, value)
		}
	}
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.MySQL.Database, "_test") {
		t.Fatal("集成测试要求使用名称以 _test 结尾的 MySQL 数据库")
	}
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
	router, err := app.router()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	base := "2001:db8:" + hex.EncodeToString(seed[:2]) + ":" + hex.EncodeToString(seed[2:4])
	main := userIntegration{url: server.URL, client: server.Client(), ip: base + "::1"}
	duplicate := main
	duplicate.ip = base + "::2"
	limited := main
	limited.ip = base + "::3"
	prefix := "it_" + hex.EncodeToString(seed[:])
	primaryPhone := fmt.Sprintf("13%09d", mathrand.Uint64N(1_000_000_000))
	secondaryPhone := "14" + primaryPhone[2:]
	primaryName, secondaryName, optionalName := prefix+"_one", primaryPhone, prefix+"_optional"
	var testUserIDs []string
	var redisKeys []string
	for _, ip := range []string{main.ip, duplicate.ip, limited.ip} {
		redisKeys = append(redisKeys, "rate:auth:register:"+ip, "rate:auth:login:"+ip)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range testUserIDs {
			if _, err := app.db.ExecContext(ctx, "DELETE FROM user WHERE id = ?", id); err != nil {
				t.Error(err)
			}
		}
		if err := app.redis.Del(ctx, redisKeys...).Err(); err != nil {
			t.Error(err)
		}
	})
	revocationKey := func(token string) string {
		t.Helper()
		claim, err := app.tokens.Parse(token, jwt.Access)
		if err != nil {
			t.Fatal(err)
		}
		key := "user:access:revoked:" + claim.ID
		redisKeys = append(redisKeys, key)
		return key
	}
	trackUser := func(id string) {
		testUserIDs = append(testUserIDs, id)
		redisKeys = append(redisKeys, "user:refresh:"+id)
	}

	primaryEmail, secondaryEmail := primaryName+"@example.test", secondaryName+"@example.test"
	initialPassword, secondaryPassword := "1", "another-password"
	longPassword := strings.Repeat("密码-", 256)
	primary := map[string]string{"username": primaryName, "password": initialPassword, "email": primaryEmail, "phone": primaryPhone}
	secondary := map[string]string{"username": secondaryName, "password": secondaryPassword, "email": secondaryEmail, "phone": secondaryPhone}

	main.call(t, http.MethodGet, "/healthz", "", nil, 0, nil)
	main.call(t, http.MethodPost, authURL+"/register", "", map[string]string{"username": "!"}, 10001, nil)
	main.call(t, http.MethodPost, authURL+"/register", "", map[string]string{"username": primaryName, "password": ""}, 10001, nil)
	var first, second, optional struct {
		userTokens
		User integrationProfile `json:"user"`
	}
	main.call(t, http.MethodPost, authURL+"/register", "", primary, 0, &first)
	trackUser(first.User.ID)
	main.call(t, http.MethodPost, authURL+"/register", "", secondary, 0, &second)
	trackUser(second.User.ID)
	main.call(t, http.MethodPost, authURL+"/register", "", map[string]string{
		"username": optionalName, "password": longPassword, "email": "", "phone": "",
	}, 0, &optional)
	trackUser(optional.User.ID)
	if optional.User.Email != nil || optional.User.Phone != nil {
		t.Fatal("未绑定的联系方式应为空")
	}
	parsedID, err := strconv.ParseInt(first.User.ID, 10, 64)
	if err != nil || parsedID <= 1<<53 || first.User.Username != primaryName || first.User.Nickname != primaryName ||
		first.User.CoinBalance != 10 || first.User.Email == nil || *first.User.Email != primaryEmail ||
		first.User.Phone == nil || *first.User.Phone != primaryPhone || first.User.AvatarURL != defaultAvatar {
		t.Fatalf("注册后的用户资料不符: %+v", first.User)
	}
	if _, err := time.Parse(time.RFC3339, first.User.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if first.AccessExpiresIn != int64(cfg.JWT.AccessTTL/time.Second) || first.RefreshExpiresIn != int64(cfg.JWT.RefreshTTL/time.Second) {
		t.Fatal("凭证时效与配置不符")
	}
	refreshClaim, err := app.tokens.Parse(first.Refresh, jwt.Refresh)
	if err != nil || refreshClaim.UserID != parsedID {
		t.Fatalf("刷新凭证声明不符: %v", err)
	}
	expiry, err := app.redis.ZScore(t.Context(), "user:refresh:"+first.User.ID, refreshClaim.ID).Result()
	if err != nil || expiry != float64(refreshClaim.ExpiresAt.Unix()) {
		t.Fatalf("Redis 未记录正确的刷新会话期限: %v", err)
	}

	duplicate.call(t, http.MethodPost, authURL+"/register", "", primary, 11001, nil)
	duplicate.call(t, http.MethodPost, authURL+"/register", "", map[string]string{
		"username": prefix + "_free", "password": initialPassword, "email": primaryEmail,
	}, 11002, nil)
	duplicate.call(t, http.MethodPost, authURL+"/register", "", map[string]string{
		"username": prefix + "_free", "password": initialPassword, "phone": primaryPhone,
	}, 11003, nil)

	var logins []userTokens
	for _, account := range []string{primaryName, primaryEmail, primaryPhone} {
		var login struct {
			userTokens
			User integrationProfile `json:"user"`
		}
		main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": account, "password": initialPassword}, 0, &login)
		if login.User.ID != first.User.ID {
			t.Fatal("登录返回了其他用户")
		}
		logins = append(logins, login.userTokens)
	}
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": primaryName, "password": "incorrect"}, 11004, nil)
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": primaryName, "password": ""}, 10001, nil)
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": optionalName, "password": longPassword}, 0, nil)
	if _, err := app.db.ExecContext(t.Context(), "UPDATE user SET status = 2 WHERE id = ?", second.User.ID); err != nil {
		t.Fatal(err)
	}
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": secondaryName, "password": secondaryPassword}, 11005, nil)

	main.call(t, http.MethodGet, usersURL+"/me", "", nil, 10002, nil)
	main.call(t, http.MethodGet, usersURL+"/me", first.Refresh, nil, 10002, nil)
	var me integrationProfile
	main.call(t, http.MethodGet, usersURL+"/me", first.Access, nil, 0, &me)
	if me.ID != first.User.ID || me.Username != primaryName || me.AvatarURL != defaultAvatar {
		t.Fatalf("个人资料不符: %+v", me)
	}
	var public map[string]any
	main.call(t, http.MethodGet, usersURL+"/"+first.User.ID, "", nil, 0, &public)
	for _, field := range []string{"id", "nickname", "avatar_url", "bio", "follower_count", "following_count", "created_at"} {
		if _, ok := public[field]; !ok {
			t.Errorf("公开主页缺少 %s", field)
		}
	}
	if len(public) != 7 || public["id"] != first.User.ID {
		t.Fatalf("公开主页字段不符: %v", public)
	}
	main.call(t, http.MethodGet, usersURL+"/invalid", "", nil, 10001, nil)
	main.call(t, http.MethodGet, usersURL+"/"+strconv.FormatInt(math.MaxInt64, 10), "", nil, 11006, nil)
	main.call(t, http.MethodPut, usersURL+"/me", first.Access, map[string]string{"nickname": "新昵称", "avatar_url": "ftp://site/image"}, 10001, nil)
	main.call(t, http.MethodPut, usersURL+"/me", first.Access, map[string]string{"nickname": "新昵称", "email": secondaryEmail}, 11002, nil)
	main.call(t, http.MethodPut, usersURL+"/me", first.Access, map[string]string{"nickname": "新昵称", "phone": secondaryPhone}, 11003, nil)
	var updated map[string]any
	main.call(t, http.MethodPut, usersURL+"/me", first.Access, map[string]string{
		"nickname": "新昵称", "bio": "新简介", "avatar_url": "/avatars/test", "email": "", "phone": "",
	}, 0, &updated)
	if updated["nickname"] != "新昵称" || updated["bio"] != "新简介" || updated["avatar_url"] != "/avatars/test" {
		t.Fatalf("更新后的个人资料不符: %v", updated)
	}
	for _, field := range []string{"email", "phone"} {
		if value, ok := updated[field]; !ok || value != nil {
			t.Fatalf("%s 应为 JSON null", field)
		}
	}
	var reset integrationProfile
	main.call(t, http.MethodPut, usersURL+"/me", first.Access, map[string]string{
		"nickname": "新昵称", "avatar_url": "",
	}, 0, &reset)
	if reset.AvatarURL != defaultAvatar {
		t.Fatalf("清空头像后返回 %q，期望 %q", reset.AvatarURL, defaultAvatar)
	}

	var rotated userTokens
	main.call(t, http.MethodPost, authURL+"/refresh", "", refreshBody(first.Refresh), 0, &rotated)
	main.call(t, http.MethodPost, authURL+"/refresh", "", refreshBody(first.Refresh), 11008, nil)
	main.call(t, http.MethodPost, authURL+"/refresh", "", refreshBody(first.Access), 11008, nil)
	concurrent := main.rotateConcurrently(t, rotated.Refresh)

	concurrentKey := revocationKey(concurrent.Access)
	main.call(t, http.MethodPost, authURL+"/logout", concurrent.Access, refreshBody(concurrent.Refresh), 0, nil)
	main.call(t, http.MethodGet, usersURL+"/me", concurrent.Access, nil, 10002, nil)
	main.call(t, http.MethodPost, authURL+"/refresh", "", refreshBody(concurrent.Refresh), 11008, nil)
	ttl, err := app.redis.PTTL(t.Context(), concurrentKey).Result()
	if err != nil || ttl <= 0 || ttl > cfg.JWT.AccessTTL {
		t.Fatalf("访问凭证撤销标记的期限不符: %v，%v", ttl, err)
	}

	revocationKey(logins[0].Access)
	main.call(t, http.MethodPost, authURL+"/logout", logins[0].Access, refreshBody(second.Refresh), 0, nil)
	var secondRotated userTokens
	main.call(t, http.MethodPost, authURL+"/refresh", "", refreshBody(second.Refresh), 0, &secondRotated)
	revocationKey(logins[1].Access)
	main.call(t, http.MethodPost, authURL+"/logout", logins[1].Access, map[string]string{}, 0, nil)
	main.call(t, http.MethodGet, usersURL+"/me", logins[1].Access, nil, 10002, nil)

	revocationKey(first.Access)
	main.call(t, http.MethodPut, usersURL+"/me/password", first.Access, map[string]string{
		"old_password": initialPassword, "new_password": "",
	}, 10001, nil)
	main.call(t, http.MethodPut, usersURL+"/me/password", first.Access, map[string]string{
		"old_password": initialPassword, "new_password": initialPassword,
	}, 10001, nil)
	main.call(t, http.MethodPut, usersURL+"/me/password", first.Access, map[string]string{
		"old_password": "incorrect", "new_password": longPassword,
	}, 11007, nil)
	main.call(t, http.MethodPut, usersURL+"/me/password", first.Access, map[string]string{
		"old_password": initialPassword, "new_password": longPassword,
	}, 0, nil)
	main.call(t, http.MethodGet, usersURL+"/me", first.Access, nil, 10002, nil)
	for _, login := range logins {
		main.call(t, http.MethodPost, authURL+"/refresh", "", refreshBody(login.Refresh), 11008, nil)
	}
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": primaryName, "password": initialPassword}, 11004, nil)
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": primaryName, "password": longPassword}, 0, nil)

	revocationKey(optional.Access)
	main.call(t, http.MethodPut, usersURL+"/me/password", optional.Access, map[string]string{
		"old_password": longPassword, "new_password": "123",
	}, 0, nil)
	main.call(t, http.MethodPost, authURL+"/login", "", map[string]string{"account": optionalName, "password": "123"}, 0, nil)

	for _, path := range []struct {
		url   string
		limit int
	}{{authURL + "/register", 5}, {authURL + "/login", 20}} {
		for range path.limit {
			limited.call(t, http.MethodPost, path.url, "", map[string]string{}, 10001, nil)
		}
		limited.call(t, http.MethodPost, path.url, "", map[string]string{}, 10005, nil)
	}
}

func refreshBody(value string) map[string]string {
	return map[string]string{"refresh_token": value}
}

func (c userIntegration) request(ctx context.Context, method, path, access string, input any) (userEnvelope, error) {
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			return userEnvelope{}, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, bytes.NewReader(body))
	if err != nil {
		return userEnvelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", c.ip)
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return userEnvelope{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return userEnvelope{}, fmt.Errorf("HTTP 状态码为 %d，期望 %d", res.StatusCode, http.StatusOK)
	}
	var envelope userEnvelope
	err = json.NewDecoder(res.Body).Decode(&envelope)
	return envelope, err
}

func (c userIntegration) call(t *testing.T, method, path, access string, input any, code int, data any) {
	t.Helper()
	res, err := c.request(t.Context(), method, path, access, input)
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != code || res.Msg == "" || (code != 0 && string(res.Data) != "null") {
		t.Fatalf("%s %s 返回 %d 和 %q，期望错误码 %d", method, path, res.Code, res.Msg, code)
	}
	if data != nil {
		if err := json.Unmarshal(res.Data, data); err != nil {
			t.Fatal(err)
		}
	}
}

func (c userIntegration) rotateConcurrently(t *testing.T, refresh string) userTokens {
	t.Helper()
	type result struct {
		res userEnvelope
		err error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			res, err := c.request(t.Context(), http.MethodPost, authURL+"/refresh", "", refreshBody(refresh))
			results <- result{res: res, err: err}
		}()
	}
	var accepted, rejected int
	var tokens userTokens
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		switch result.res.Code {
		case 0:
			accepted++
			if err := json.Unmarshal(result.res.Data, &tokens); err != nil {
				t.Fatal(err)
			}
		case 11008:
			rejected++
		default:
			t.Fatalf("并发刷新返回意外错误码 %d", result.res.Code)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("同一凭证并发轮换收到 %d 次成功和 %d 次拒绝", accepted, rejected)
	}
	return tokens
}
