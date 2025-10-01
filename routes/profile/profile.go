package profile

import (
	"AkuAI/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(g *gin.RouterGroup, db *gorm.DB) {
	g.GET("/profile", controllers.Profile(db))
	g.PUT("/profile", controllers.Profile(db))
	g.POST("/profile/image/token", controllers.ProfileImageUploadToken(db))
	g.POST("/profile/image/upload", controllers.ProfileImageUpload(db))
	g.GET("/profile/image", controllers.ProfileImageURL(db))
	g.DELETE("/profile/image", controllers.DeleteProfileImage(db))
}
