package conversation

import (
	"AkuAI/controllers"
	"AkuAI/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register registers conversation routes (protected)
func Register(g *gin.RouterGroup, db *gorm.DB) {
	// Basic rate limiting on chat POST endpoints
	g.POST("/conversations", middleware.RateLimit(), controllers.CreateOrAddMessage(db))
	g.POST("/conversations/stream", middleware.RateLimit(), controllers.CreateOrAddMessageStream(db))
	g.GET("/conversations", controllers.ListConversations(db))
	g.GET("/conversations/:conversation_id", controllers.GetConversation(db))
	g.DELETE("/conversations/:conversation_id", controllers.DeleteConversation(db))
	// Delete all conversations for current user
	g.DELETE("/conversations", controllers.DeleteAllConversations(db))
}
