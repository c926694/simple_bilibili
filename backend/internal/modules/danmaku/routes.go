package danmaku

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.Engine, h *Handler, auth, optionalAuth, limit gin.HandlerFunc) {
	router.GET("/ws/videos/:bvid", websocketCredentials, optionalAuth, h.connect)
	router.GET("/api/v1/videos/:bvid/danmaku", h.history)
	router.POST("/api/v1/videos/:bvid/danmaku", auth, limit, h.send)
}
