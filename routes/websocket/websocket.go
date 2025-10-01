package websocket

import (
	"AkuAI/controllers"
	"AkuAI/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(r *gin.Engine, db *gorm.DB) {
	r.GET("/ws/chat", middleware.RateLimit(), controllers.ChatWS(db))
}
