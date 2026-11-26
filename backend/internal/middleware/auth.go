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
	UserIDKey = "user_id"
	JTIKey    = "jti"
	ExpiryKey = "exp"
)

func Authenticate(tokens *jwt.Manager, revoked func(context.Context, string) (bool, error), logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Fail(c, errcode.Unauthorized, "")
			return
		}
		claims, err := tokens.Parse(parts[1], jwt.Access)
		if err != nil {
			response.Fail(c, errcode.Unauthorized, "")
			return
		}
		blocked, err := revoked(c.Request.Context(), claims.ID)
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "读取访问凭证撤销状态", "error", err)
		}
		if err != nil || blocked {
			response.Fail(c, errcode.Unauthorized, "")
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(JTIKey, claims.ID)
		c.Set(ExpiryKey, claims.ExpiresAt.Time)
		c.Next()
	}
}
