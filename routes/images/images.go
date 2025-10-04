package images

import (
	"AkuAI/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(r *gin.RouterGroup, db *gorm.DB) {
	imageController := controllers.NewImageController()

	// Image API routes
	apiGroup := r.Group("/api/images")
	{
		apiGroup.GET("/health", imageController.HealthCheck)
		apiGroup.POST("/search", imageController.SearchImages)
		apiGroup.GET("/chat", imageController.SearchImagesFromChat)
	}
}
