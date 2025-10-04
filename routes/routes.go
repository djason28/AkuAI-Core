package routes

import (
	"AkuAI/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	authRoutes "AkuAI/routes/auth"
	convRoutes "AkuAI/routes/conversation"
	imageRoutes "AkuAI/routes/images"
	profileRoutes "AkuAI/routes/profile"
	uibRoutes "AkuAI/routes/uib"
	uploadsRoutes "AkuAI/routes/uploads"
	websocketRoutes "AkuAI/routes/websocket"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "Go auth + chat backend running"})
	})

	uploadsRoutes.Register(r, db)
	websocketRoutes.Register(r, db)
	authRoutes.RegisterPublic(r, db)

	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	authRoutes.RegisterProtected(protected, db)
	profileRoutes.Register(protected, db)
	convRoutes.Register(protected, db)

	// UIB routes - accessible to all authenticated users
	uibRoutes.Register(protected, db)

	// Image search routes - accessible to all authenticated users
	imageRoutes.Register(protected, db)
}
