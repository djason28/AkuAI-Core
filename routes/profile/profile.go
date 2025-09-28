package profile

import (
	"AkuAI/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register registers protected profile routes on supplied router group
// expects the group to already have AuthMiddleware applied
func Register(g *gin.RouterGroup, db *gorm.DB) {
	g.GET("/profile", controllers.Profile(db))
	g.PUT("/profile", controllers.Profile(db))
}
