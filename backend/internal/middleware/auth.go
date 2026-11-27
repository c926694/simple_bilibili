package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/jwt"
	"simple_bilibili/backend/internal/pkg/response"
)

const (
	UserIDKey        = "user_id"
	JTIKey           = "jti"
	ExpiryKey        = "exp"
	JustCheckedInKey = "just_checked_in"
)

type DailyCheckinHook func(context.Context, int64) (bool, int, error)

func Authenticate(tokens *jwt.Manager, revoked func(context.Context, string) (bool, error), onAuthenticated DailyCheckinHook, logger *slog.Logger) gin.HandlerFunc {
	return authenticate(tokens, revoked, onAuthenticated, logger, false)
}

func AuthenticateOptional(tokens *jwt.Manager, revoked func(context.Context, string) (bool, error), onAuthenticated DailyCheckinHook, logger *slog.Logger) gin.HandlerFunc {
	return authenticate(tokens, revoked, onAuthenticated, logger, true)
}

func authenticate(tokens *jwt.Manager, revoked func(context.Context, string) (bool, error), onAuthenticated DailyCheckinHook, logger *slog.Logger, optional bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthenticated(c, optional)
			return
		}
		claims, err := tokens.Parse(parts[1], jwt.Access)
		if err != nil {
			unauthenticated(c, optional)
			return
		}
		blocked, err := revoked(c.Request.Context(), claims.ID)
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "读取访问凭证撤销状态", "error", err)
		}
		if err != nil || blocked {
			unauthenticated(c, optional)
			return
		}
		justCheckedIn, _, err := onAuthenticated(c.Request.Context(), claims.UserID)
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "执行每日自动签到", "error", err)
			response.Fail(c, errcode.InternalError, "")
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(JTIKey, claims.ID)
		c.Set(ExpiryKey, claims.ExpiresAt.Time)
		c.Set(JustCheckedInKey, justCheckedIn)
		c.Next()
	}
}

func unauthenticated(c *gin.Context, optional bool) {
	if optional {
		c.Next()
		return
	}
	response.Fail(c, errcode.Unauthorized, "")
}
