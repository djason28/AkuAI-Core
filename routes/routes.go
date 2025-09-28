package routes

import (
	"AkuAI/controllers"
	"AkuAI/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	authRoutes "AkuAI/routes/auth"
	convRoutes "AkuAI/routes/conversation"
	profileRoutes "AkuAI/routes/profile"
)

// RegisterRoutes mendaftarkan semua route aplikasi.
// - r : *gin.Engine
// - db: *gorm.DB (dipass ke controllers melalui route handlers)
func RegisterRoutes(r *gin.Engine, db *gorm.DB) {
	// health / index
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "Go auth + chat backend running"})
	})

	// Public routes (no auth)
	authRoutes.RegisterPublic(r, db)

	// WebSocket chat (self-auth via token query) with handshake rate limit
	r.GET("/ws/chat", middleware.RateLimit(), controllers.ChatWS(db))

	// Protected routes (memakai middleware AuthMiddleware)
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())

	// auth protected (logout)
	authRoutes.RegisterProtected(protected, db)

	// profile
	profileRoutes.Register(protected, db)

	// conversations / chat
	convRoutes.Register(protected, db)

}
