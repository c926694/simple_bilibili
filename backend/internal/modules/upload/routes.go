package upload

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, h *Handler, auth gin.HandlerFunc) {
	group := router.Group("/uploads", auth)
	group.POST("/image-presign", h.imagePresign)
	group.POST("/video/init", h.initVideo)
	group.POST("/video/:task_id/part-urls", h.partURLs)
	group.POST("/video/:task_id/complete", h.complete)
	group.POST("/video/:task_id/abort", h.abort)
	group.GET("/video/:task_id", h.detail)
}
