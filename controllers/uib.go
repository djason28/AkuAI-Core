package controllers

import (
	"net/http"

	"AkuAI/models"
	"AkuAI/pkg/services"

	"github.com/gin-gonic/gin"
)

type UIBController struct {
	uibService *services.UIBEventService
}

func NewUIBController() (*UIBController, error) {
	uibService, err := services.NewUIBEventService()
	if err != nil {
		return nil, err
	}

	return &UIBController{
		uibService: uibService,
	}, nil
}

// GetAllEvents returns all UIB events
func (ctrl *UIBController) GetAllEvents(c *gin.Context) {
	events := ctrl.uibService.GetAllEvents()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   len(events),
		"message": "UIB events retrieved successfully",
	})
}

// GetEventsByMonth returns events for specific month
func (ctrl *UIBController) GetEventsByMonth(c *gin.Context) {
	month := c.Param("month")
	if month == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Month parameter is required",
		})
		return
	}

	events := ctrl.uibService.GetEventsByMonth(month)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"month":   month,
		"total":   len(events),
		"message": "UIB events for " + month + " retrieved successfully",
	})
}

// GetEventsByType returns events by type (certification or webinar)
func (ctrl *UIBController) GetEventsByType(c *gin.Context) {
	eventType := c.Param("type")
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Event type parameter is required",
		})
		return
	}

	// Validate event type
	if eventType != "certification" && eventType != "webinar" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Event type must be 'certification' or 'webinar'",
		})
		return
	}

	events := ctrl.uibService.GetEventsByType(eventType)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"type":    eventType,
		"total":   len(events),
		"message": "UIB " + eventType + " events retrieved successfully",
	})
}

// GetEventByID returns specific event by ID
func (ctrl *UIBController) GetEventByID(c *gin.Context) {
	eventID := c.Param("id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Event ID parameter is required",
		})
		return
	}

	event, err := ctrl.uibService.GetEventByID(eventID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Event not found: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    event,
		"message": "UIB event retrieved successfully",
	})
}

// GetUpcomingEvents returns upcoming events
func (ctrl *UIBController) GetUpcomingEvents(c *gin.Context) {
	events := ctrl.uibService.GetUpcomingEvents()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   len(events),
		"message": "Upcoming UIB events retrieved successfully",
	})
}

// SearchEvents searches events based on query parameters
func (ctrl *UIBController) SearchEvents(c *gin.Context) {
	// Get search parameters
	eventType := c.Query("type")          // certification, webinar, or empty for all
	month := c.Query("month")             // october, november, december, or empty for all
	department := c.Query("department")   // department filter
	freeOnly := c.Query("free") == "true" // filter for free events only

	criteria := models.EventSearchCriteria{
		EventType:  eventType,
		Month:      month,
		Department: department,
		FreeOnly:   freeOnly,
	}

	events := ctrl.uibService.SearchEvents(criteria)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     events,
		"total":    len(events),
		"criteria": criteria,
		"message":  "UIB events search completed successfully",
	})
}

// GetEventSummaries returns summarized view of all events
func (ctrl *UIBController) GetEventSummaries(c *gin.Context) {
	summaries := ctrl.uibService.GetEventSummaries()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summaries,
		"total":   len(summaries),
		"message": "UIB event summaries retrieved successfully",
	})
}

// QueryUIBEvents searches events based on natural language query
func (ctrl *UIBController) QueryUIBEvents(c *gin.Context) {
	var request struct {
		Query string `json:"query" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Check if query is UIB-related
	isUIBRelated := ctrl.uibService.AnalyzeQueryForUIB(request.Query)

	if !isUIBRelated {
		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"data":           []interface{}{},
			"total":          0,
			"message":        "Query tidak terkait dengan UIB. Silakan tanyakan tentang sertifikasi, webinar, atau acara UIB.",
			"is_uib_related": false,
		})
		return
	}

	// Get relevant events
	events := ctrl.uibService.GetRelevantEventsForQuery(request.Query)

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"data":           events,
		"total":          len(events),
		"query":          request.Query,
		"message":        "Relevant UIB events found",
		"is_uib_related": true,
	})
}

// GetUIBContext returns formatted UIB context for AI
func (ctrl *UIBController) GetUIBContext(c *gin.Context) {
	var request struct {
		Query string `json:"query"`
	}

	// Try to bind JSON, but it's optional
	c.ShouldBindJSON(&request)

	var events []models.UIBEvent

	if request.Query != "" {
		// Get events relevant to query
		events = ctrl.uibService.GetRelevantEventsForQuery(request.Query)
	} else {
		// Get all upcoming events
		events = ctrl.uibService.GetUpcomingEvents()
	}

	// Format for AI context
	formattedContext := ctrl.uibService.FormatEventsForGemini(events)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"raw_events":        events,
			"formatted_context": formattedContext,
			"total_events":      len(events),
			"query":             request.Query,
		},
		"message": "UIB context generated successfully",
	})
}

// HealthCheck returns service health status
func (ctrl *UIBController) HealthCheck(c *gin.Context) {
	allEvents := ctrl.uibService.GetAllEvents()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"service": "UIB Event Service",
		"status":  "healthy",
		"data": gin.H{
			"total_events": len(allEvents),
			"data_source":  "uib_events.json",
			"last_updated": "2025-10-04",
			"institution":  "Universitas Internasional Batam (UIB)",
		},
		"endpoints": []string{
			"GET /api/uib/events",
			"GET /api/uib/events/month/:month",
			"GET /api/uib/events/type/:type",
			"GET /api/uib/events/:id",
			"GET /api/uib/events/upcoming",
			"GET /api/uib/events/search",
			"GET /api/uib/events/summaries",
			"POST /api/uib/query",
			"POST /api/uib/context",
		},
		"message": "UIB Event Service is running properly",
	})
}
