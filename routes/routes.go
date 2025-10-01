package routes

import (
	"AkuAI/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	authRoutes "AkuAI/routes/auth"
	convRoutes "AkuAI/routes/conversation"
	profileRoutes "AkuAI/routes/profile"
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
}
