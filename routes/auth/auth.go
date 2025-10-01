package auth

import (
	"AkuAI/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterPublic(r *gin.Engine, db *gorm.DB) {
	r.POST("/register", controllers.Register(db))
	r.POST("/login", controllers.Login(db))
}

func RegisterProtected(g *gin.RouterGroup, db *gorm.DB) {
	g.POST("/logout", controllers.Logout())
}
