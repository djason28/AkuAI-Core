package conversation

import (
	"AkuAI/controllers"
	"AkuAI/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(g *gin.RouterGroup, db *gorm.DB) {
	g.POST("/conversations", middleware.RateLimit(), controllers.CreateOrAddMessage(db))
	g.POST("/conversations/stream", middleware.RateLimit(), controllers.CreateOrAddMessageStream(db))
	g.POST("/conversations/compare", middleware.RateLimit(), controllers.ComparePromptModes())
	g.GET("/conversations", controllers.ListConversations(db))
	g.GET("/conversations/:conversation_id", controllers.GetConversation(db))
	g.DELETE("/conversations/:conversation_id", controllers.DeleteConversation(db))
	g.DELETE("/conversations", controllers.DeleteAllConversations(db))
}
