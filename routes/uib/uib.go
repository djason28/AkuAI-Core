package uib

import (
	"log"
	"net/http"

	"AkuAI/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(r *gin.RouterGroup, db *gorm.DB) {
	// Initialize UIB controller
	uibController, err := controllers.NewUIBController()
	if err != nil {
		log.Printf("Failed to initialize UIB controller: %v", err)
		// Register a fallback handler
		r.GET("/api/uib/health", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"message": "UIB service is unavailable: " + err.Error(),
			})
		})
		return
	}

	// UIB API routes group
	uibGroup := r.Group("/api/uib")
	{
		// Health check endpoint
		uibGroup.GET("/health", uibController.HealthCheck)

		// Events endpoints
		uibGroup.GET("/events", uibController.GetAllEvents)
		uibGroup.GET("/events/month/:month", uibController.GetEventsByMonth)
		uibGroup.GET("/events/type/:type", uibController.GetEventsByType)
		uibGroup.GET("/events/upcoming", uibController.GetUpcomingEvents)
		uibGroup.GET("/events/summaries", uibController.GetEventSummaries)
		uibGroup.GET("/events/search", uibController.SearchEvents)
		uibGroup.GET("/events/:id", uibController.GetEventByID)

		// Query endpoints
		uibGroup.POST("/query", uibController.QueryUIBEvents)
		uibGroup.POST("/context", uibController.GetUIBContext)
	}

	log.Printf("UIB routes registered successfully")
}
