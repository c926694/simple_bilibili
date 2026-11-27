package interaction

import "github.com/gin-gonic/gin"

func RegisterRoutes(api *gin.RouterGroup, h *Handler, auth, optionalAuth gin.HandlerFunc) {
	videos := api.Group("/videos/:bvid")
	videos.GET("/interaction", auth, h.state)
	videos.POST("/like", auth, h.like(true))
	videos.DELETE("/like", auth, h.like(false))
	videos.POST("/coin", auth, h.coin)
	videos.POST("/triple", auth, h.triple)
	videos.POST("/share", optionalAuth, h.share)
	videos.POST("/play", optionalAuth, h.play)
	videos.PUT("/favorites", auth, h.setFavorites)
	videos.POST("/progress", auth, h.progress)
	folders := api.Group("/favorite-folders")
	folders.GET("", optionalAuth, h.folders)
	folders.POST("", auth, h.createFolder)
	folders.PUT("/:id", auth, h.editFolder)
	folders.DELETE("/:id", auth, h.deleteFolder)
	folders.GET("/:id/videos", optionalAuth, h.folderVideos)
	api.GET("/history", auth, h.history)
	api.DELETE("/history", auth, h.deleteHistory)
	api.DELETE("/history/:bvid", auth, h.deleteHistory)
	api.GET("/users/me/checkin", auth, h.checkinStatus)
	api.POST("/users/me/checkin", auth, h.checkinStatus)
}
