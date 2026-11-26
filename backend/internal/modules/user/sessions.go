package user

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/pkg/jwt"
)

var rotateRefreshSession = redis.NewScript(`
local now = tonumber(redis.call('TIME')[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
if ARGV[1] ~= '' and redis.call('ZREM', KEYS[1], ARGV[1]) == 0 then
    return 0
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[2])
local latest = redis.call('ZRANGE', KEYS[1], -1, -1, 'WITHSCORES')
redis.call('EXPIREAT', KEYS[1], latest[2])
return 1
`)

func refreshKey(userID int64) string {
	return "user:refresh:" + strconv.FormatInt(userID, 10)
}

func revokedKey(jti string) string {
	return "user:access:revoked:" + jti
}

func (s *Store) saveSession(ctx context.Context, userID int64, previousJTI string, next jwt.Token) (bool, error) {
	// 旧凭证只允许轮换一次，集合有效期保留到其中最晚的凭证到期时间。
	result, err := rotateRefreshSession.Run(ctx, s.redis, []string{refreshKey(userID)}, previousJTI, next.ID, next.ExpiresAt.Unix()).Int()
	if err != nil {
		return false, fmt.Errorf("保存或轮换刷新会话: %w", err)
	}
	return result == 1, nil
}

func (s *Store) accessRevoked(ctx context.Context, jti string) (bool, error) {
	count, err := s.redis.Exists(ctx, revokedKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("读取访问凭证撤销状态: %w", err)
	}
	return count > 0, nil
}

func (s *Store) revokeSessions(ctx context.Context, access accessSession, refreshJTI string, all bool) error {
	_, err := s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		// 已过期的访问凭证会由 JWT 校验拒绝，只为仍有效的凭证保存撤销标记。
		if ttl := time.Until(access.ExpiresAt); ttl > 0 {
			pipe.Set(ctx, revokedKey(access.JTI), 1, ttl)
		}
		if all {
			pipe.Del(ctx, refreshKey(access.UserID))
		} else if refreshJTI != "" {
			pipe.ZRem(ctx, refreshKey(access.UserID), refreshJTI)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("撤销用户会话: %w", err)
	}
	return nil
}
