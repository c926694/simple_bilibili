package video

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, h *Handler, auth, optionalAuth gin.HandlerFunc) {
	router.GET("/categories", h.categories)
	router.GET("/users/:id/videos", h.byAuthor)
	videos := router.Group("/videos")
	videos.GET("", h.list)
	videos.GET("/search", h.search)
	videos.GET("/mine", auth, h.mine)
	videos.GET("/:bvid", optionalAuth, h.detail)
	videos.POST("", auth, h.create)
	videos.PUT("/:bvid", auth, h.edit)
	videos.DELETE("/:bvid", auth, h.withdraw)
	videos.POST("/:bvid/parts/:part_id/retry-transcode", auth, h.retry)
}
