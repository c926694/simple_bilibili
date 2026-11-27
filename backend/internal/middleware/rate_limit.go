package middleware

import (
	"crypto/rand"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

var slidingWindow = redis.NewScript(`
local clock = redis.call('TIME')
local now = clock[1] * 1000 + math.floor(clock[2] / 1000)
local window = tonumber(ARGV[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[2]) then
    return 0
end
redis.call('ZADD', KEYS[1], now, ARGV[3])
redis.call('PEXPIRE', KEYS[1], window)
return 1
`)

func RateLimit(client *redis.Client, logger *slog.Logger, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return rateLimit(client, logger, limit, window, func(c *gin.Context) string { return "rate:auth:" + scope + ":" + c.ClientIP() })
}

func RateLimitByUser(client *redis.Client, logger *slog.Logger, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return rateLimit(client, logger, limit, window, func(c *gin.Context) string {
		return "rate:user:" + scope + ":" + strconv.FormatInt(c.GetInt64(UserIDKey), 10)
	})
}

func rateLimit(client *redis.Client, logger *slog.Logger, limit int, window time.Duration, keyFor func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := keyFor(c)
		allowed, err := slidingWindow.Run(ctx, client, []string{key}, window.Milliseconds(), limit, rand.Text()).Int()
		if err != nil {
			logger.ErrorContext(ctx, "检查请求限流", "error", err)
			response.Fail(c, errcode.InternalError, "")
			return
		}
		if allowed == 0 {
			response.Fail(c, errcode.TooManyRequests, "")
			return
		}
		c.Next()
	}
}
