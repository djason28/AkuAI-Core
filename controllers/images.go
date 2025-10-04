package controllers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"AkuAI/pkg/services"

	"github.com/gin-gonic/gin"
)

type ImageController struct {
	imageService *services.GoogleImageService
}

func NewImageController() *ImageController {
	return &ImageController{
		imageService: services.NewGoogleImageService(),
	}
}

type ImageSearchRequest struct {
	Query      string `json:"query" binding:"required"`
	MaxResults int    `json:"max_results,omitempty"`
}

type ImageSearchResponse struct {
	Success bool                         `json:"success"`
	Message string                       `json:"message,omitempty"`
	Images  []services.ImageSearchResult `json:"images,omitempty"`
	Query   string                       `json:"query"`
	Count   int                          `json:"count"`
}

// SearchImages handles POST /api/images/search
func (ctrl *ImageController) SearchImages(c *gin.Context) {
	var req ImageSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ImageSearchResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}

	if !ctrl.imageService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, ImageSearchResponse{
			Success: false,
			Message: "Image search service is not available. Please configure Google Custom Search API.",
		})
		return
	}

	// Set default max results if not provided
	if req.MaxResults <= 0 || req.MaxResults > 10 {
		req.MaxResults = 4
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Search for images
	images, err := ctrl.imageService.SearchImages(ctx, req.Query, req.MaxResults)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ImageSearchResponse{
			Success: false,
			Message: "Failed to search images: " + err.Error(),
			Query:   req.Query,
		})
		return
	}

	c.JSON(http.StatusOK, ImageSearchResponse{
		Success: true,
		Images:  images,
		Query:   req.Query,
		Count:   len(images),
	})
}

// SearchImagesFromChat handles GET /api/images/chat?q=query&max=4
func (ctrl *ImageController) SearchImagesFromChat(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, ImageSearchResponse{
			Success: false,
			Message: "Query parameter 'q' is required",
		})
		return
	}

	maxResults := 4
	if maxStr := c.Query("max"); maxStr != "" {
		if parsed, err := strconv.Atoi(maxStr); err == nil && parsed > 0 && parsed <= 10 {
			maxResults = parsed
		}
	}

	if !ctrl.imageService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, ImageSearchResponse{
			Success: false,
			Message: "Image search service is not available",
		})
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Use the chat-specific search method that extracts keywords
	images, err := ctrl.imageService.SearchImages(ctx, ctrl.imageService.ExtractSearchTermFromContext(query), maxResults)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ImageSearchResponse{
			Success: false,
			Message: "Failed to search images: " + err.Error(),
			Query:   query,
		})
		return
	}

	c.JSON(http.StatusOK, ImageSearchResponse{
		Success: true,
		Images:  images,
		Query:   query,
		Count:   len(images),
	})
}

// HealthCheck handles GET /api/images/health
func (ctrl *ImageController) HealthCheck(c *gin.Context) {
	status := "disabled"
	if ctrl.imageService.IsEnabled() {
		status = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"service": "google-images",
		"status":  status,
		"message": "Image search service status",
	})
}
