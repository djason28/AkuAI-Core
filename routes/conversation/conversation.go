package conversation

import (
	"AkuAI/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register registers conversation routes (protected)
func Register(g *gin.RouterGroup, db *gorm.DB) {
	g.POST("/conversations", controllers.CreateOrAddMessage(db))
	g.POST("/conversations/stream", controllers.CreateOrAddMessageStream(db))
	g.GET("/conversations", controllers.ListConversations(db))
	g.GET("/conversations/:conversation_id", controllers.GetConversation(db))
	g.DELETE("/conversations/:conversation_id", controllers.DeleteConversation(db))
}
