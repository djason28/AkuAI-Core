package services

import (
	"context"
	"log"
)

type UIBImageService struct {
	images []ImageSearchResult
}

// Static UIB images data
func NewUIBImageService() *UIBImageService {
	uibImages := []ImageSearchResult{
		{
			Title:        "Gedung Utama Universitas Internasional Batam",
			ImageURL:     "https://uib.ac.id/wp-content/uploads/2023/01/gedung-utama-uib.jpg",
			ThumbnailURL: "https://uib.ac.id/wp-content/uploads/2023/01/gedung-utama-uib-thumb.jpg",
			SourceURL:    "",
			Width:        800,
			Height:       600,
		},
		{
			Title:        "Logo Resmi Universitas Internasional Batam",
			ImageURL:     "https://uib.ac.id/wp-content/uploads/2023/01/logo-uib-official.png",
			ThumbnailURL: "https://uib.ac.id/wp-content/uploads/2023/01/logo-uib-official-thumb.png",
			SourceURL:    "",
			Width:        512,
			Height:       512,
		},
		{
			Title:        "Ruang Kuliah Modern UIB",
			ImageURL:     "https://uib.ac.id/wp-content/uploads/2023/01/ruang-kuliah-uib.jpg",
			ThumbnailURL: "https://uib.ac.id/wp-content/uploads/2023/01/ruang-kuliah-uib-thumb.jpg",
			SourceURL:    "",
			Width:        800,
			Height:       600,
		},
		{
			Title:        "Laboratorium Komputer UIB",
			ImageURL:     "https://uib.ac.id/wp-content/uploads/2023/01/lab-komputer-uib.jpg",
			ThumbnailURL: "https://uib.ac.id/wp-content/uploads/2023/01/lab-komputer-uib-thumb.jpg",
			SourceURL:    "",
			Width:        800,
			Height:       600,
		},
	}

	log.Printf("[uib-images] ✅ UIB Static Images Service initialized with %d images", len(uibImages))

	return &UIBImageService{
		images: uibImages,
	}
}

func (s *UIBImageService) IsEnabled() bool {
	return true // Always enabled since it's static data
}

// GetUIBImages returns the static UIB images
func (s *UIBImageService) GetUIBImages(ctx context.Context) ([]ImageSearchResult, error) {
	log.Printf("[uib-images] 📸 Returning %d UIB images", len(s.images))
	return s.images, nil
}

// SearchImages placeholder for compatibility - always returns UIB images
func (s *UIBImageService) SearchImages(ctx context.Context, query string, maxResults int) ([]ImageSearchResult, error) {
	if maxResults <= 0 || maxResults > len(s.images) {
		maxResults = len(s.images)
	}

	results := make([]ImageSearchResult, maxResults)
	copy(results, s.images[:maxResults])

	log.Printf("[uib-images] 🔍 Returning %d UIB images for query: %s", len(results), query)
	return results, nil
}

// ExtractSearchTermFromContext - not needed for static images but kept for compatibility
func (s *UIBImageService) ExtractSearchTermFromContext(message string) string {
	// Static service always returns fixed search term regardless of input
	_ = message // acknowledge parameter to avoid unused parameter warning
	return "universitas internasional batam"
}

// SearchImagesForChat - returns UIB images
func (s *UIBImageService) SearchImagesForChat(ctx context.Context, message string) ([]ImageSearchResult, error) {
	return s.GetUIBImages(ctx)
}
