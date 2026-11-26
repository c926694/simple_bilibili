package user

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, h *Handler, auth, registerLimit, loginLimit gin.HandlerFunc) {
	authRoutes := router.Group("/auth")
	authRoutes.POST("/register", registerLimit, h.register)
	authRoutes.POST("/login", loginLimit, h.login)
	authRoutes.POST("/refresh", h.refresh)
	authRoutes.POST("/logout", auth, h.logout)

	users := router.Group("/users")
	users.GET("/me", auth, h.me)
	users.PUT("/me", auth, h.updateProfile)
	users.POST("/me/avatar", auth, h.uploadAvatar)
	users.PUT("/me/password", auth, h.changePassword)
	users.GET("/:id", h.publicProfile)
}
