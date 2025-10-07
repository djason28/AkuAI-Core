package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"AkuAI/pkg/config"
)

var mockImageCatalog = map[string][]string{
	"universitas internasional batam": {
		"https://images.unsplash.com/photo-1588072432836-e10032774350?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1509391366360-2e959784a276?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1498050108023-c5249f4df085?auto=format&fit=crop&w=1200&q=80",
	},
	"universitas indonesia": {
		"https://images.unsplash.com/photo-1523580846011-d3a5bc25702b?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1503676260728-1c00da094a0b?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1521587760476-6c12a4b040da?auto=format&fit=crop&w=1200&q=80",
	},
	"universitas gadjah mada": {
		"https://images.unsplash.com/photo-1498050108023-c5249f4df085?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1521737604893-d14cc237f11d?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1521737604893-0fe0f5f48f8b?auto=format&fit=crop&w=1200&q=80",
	},
	"institut teknologi bandung": {
		"https://images.unsplash.com/photo-1523050854058-8df90110c9f1?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1509099955925-930d1a1ae353?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1498050108023-c5249f4df085?auto=format&fit=crop&w=1200&q=80",
	},
	"kampus": {
		"https://images.unsplash.com/photo-1523580846011-d3a5bc25702b?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1503676260728-1c00da094a0b?auto=format&fit=crop&w=1200&q=80",
		"https://images.unsplash.com/photo-1521587760476-6c12a4b040da?auto=format&fit=crop&w=1200&q=80",
	},
}

func validateImageURL(client *http.Client, imageURL string) bool {
	if !(strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://")) {
		return false
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	res, err := client.Head(imageURL)
	if err == nil {
		defer res.Body.Close()
		if res.StatusCode == http.StatusOK {
			if strings.HasPrefix(res.Header.Get("Content-Type"), "image/") {
				return true
			}
		}
	}
	resp, err := client.Get(imageURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()
	buf := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, buf)
	return strings.HasPrefix(http.DetectContentType(buf[:n]), "image/")
}

func fetchRawGoogleImages(query, apiKey, cx string, numToFetch, startIndex int) ([]struct {
	Link string `json:"link"`
}, error) {
	if numToFetch <= 0 {
		return []struct {
			Link string `json:"link"`
		}{}, nil
	}
	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?q=%s&key=%s&cx=%s&searchType=image&num=%d&start=%d",
		url.QueryEscape(query), apiKey, cx, numToFetch, startIndex)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("gagal melakukan permintaan ke Google API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gagal mendapatkan gambar dari Google: status %d, body: %s", resp.StatusCode, string(errorBody))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca body respons: %w", err)
	}

	var googleResp struct {
		Items []struct {
			Link string `json:"link"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &googleResp); err != nil {
		return nil, fmt.Errorf("gagal unmarshal JSON respons: %w. Respons body: %s", err, string(body))
	}
	return googleResp.Items, nil
}

// return 1 image URL
func GetGoogleImages(query string) ([]string, error) {
	apiKey := config.GoogleAPIKey
	cx := config.GoogleAPI_CX

	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("google API Key atau CX tidak ditemukan")
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
	}
	validImageURLs := []string{}
	const (
		targetCount = 1
		maxAttempts = 5
		baseFetch   = 2
	)

	for attempt := 0; attempt < maxAttempts && len(validImageURLs) < targetCount; attempt++ {
		numToFetch := baseFetch + attempt
		if numToFetch > 5 {
			numToFetch = 5
		}

		items, err := fetchRawGoogleImages(query, apiKey, cx, numToFetch, 1)
		if err != nil || len(items) < 1 {
			break
		}

		links := make([]string, len(items))
		for i, itm := range items {
			links[i] = itm.Link
		}
		fmt.Printf("[Attempt %d] fetched links: %v\n", attempt+1, links)

		start := 0
		if len(items) > 1 {
			start = len(items) - 1
		}
		tailItems := items[start:]

		for _, item := range tailItems {
			if validateImageURL(client, item.Link) {
				dup := false
				for _, u := range validImageURLs {
					if u == item.Link {
						dup = true
						break
					}
				}
				if !dup {
					validImageURLs = append(validImageURLs, item.Link)
					if len(validImageURLs) == targetCount {
						break
					}
				}
			}
		}
	}

	if len(validImageURLs) < targetCount {
		return nil, fmt.Errorf("gagal menemukan %d gambar valid untuk '%s' setelah %d percobaan",
			targetCount, query, maxAttempts)
	}
	return validImageURLs, nil
}

// return 2 image URL
func GetGoogleImagesPlaces(query string) ([]string, error) {
	apiKey := config.GoogleAPIKey
	cx := config.GoogleAPI_CX

	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("google API Key atau CX tidak ditemukan")
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
	}
	validImageURLs := []string{}
	const (
		targetCount = 2
		maxAttempts = 5
		baseFetch   = 3
	)

	for attempt := 0; attempt < maxAttempts && len(validImageURLs) < targetCount; attempt++ {
		numToFetch := baseFetch + attempt
		if numToFetch > 5 {
			numToFetch = 5
		}

		items, err := fetchRawGoogleImages(query, apiKey, cx, numToFetch, 1)
		if err != nil || len(items) < 1 {
			break
		}

		links := make([]string, len(items))
		for i, itm := range items {
			links[i] = itm.Link
		}
		fmt.Printf("[Attempt %d] fetched links: %v\n", attempt+1, links)

		start := 0
		if len(items) > 2 {
			start = len(items) - 2
		}
		tailItems := items[start:]

		for _, item := range tailItems {
			if validateImageURL(client, item.Link) {
				dup := false
				for _, u := range validImageURLs {
					if u == item.Link {
						dup = true
						break
					}
				}
				if !dup {
					validImageURLs = append(validImageURLs, item.Link)
					if len(validImageURLs) == targetCount {
						break
					}
				}
			}
		}
	}

	if len(validImageURLs) < targetCount {
		return nil, fmt.Errorf("gagal menemukan %d gambar valid untuk '%s' setelah %d percobaan",
			targetCount, query, maxAttempts)
	}
	return validImageURLs, nil
}

// return 4 image URLs dengan retry mechanism yang agresif
func GetGoogleImages4(query string) ([]string, error) {
	apiKey := config.GoogleAPIKey
	cx := config.GoogleAPI_CX

	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("google API Key atau CX tidak ditemukan")
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
	}
	validImageURLs := []string{}
	const (
		targetCount = 4
		maxAttempts = 10 // Increase attempts untuk dapat 4 gambar
		baseFetch   = 5  // Fetch more images per attempt
	)

	for attempt := 0; attempt < maxAttempts && len(validImageURLs) < targetCount; attempt++ {
		numToFetch := baseFetch + attempt
		if numToFetch > 10 {
			numToFetch = 10
		}

		// Use different start index to get variety
		startIndex := 1 + (attempt * 3)
		if startIndex > 90 {
			startIndex = 1
		}

		items, err := fetchRawGoogleImages(query, apiKey, cx, numToFetch, startIndex)
		if err != nil || len(items) < 1 {
			continue // Try next attempt
		}

		links := make([]string, len(items))
		for i, itm := range items {
			links[i] = itm.Link
		}
		fmt.Printf("[Attempt %d] fetched %d links from start %d: %v\n", attempt+1, len(links), startIndex, links)

		// Process all items, not just tail
		for _, item := range items {
			if validateImageURL(client, item.Link) {
				dup := false
				for _, u := range validImageURLs {
					if u == item.Link {
						dup = true
						break
					}
				}
				if !dup {
					validImageURLs = append(validImageURLs, item.Link)
					fmt.Printf("✅ Valid image %d: %s\n", len(validImageURLs), item.Link)
					if len(validImageURLs) == targetCount {
						break
					}
				}
			} else {
				fmt.Printf("❌ Invalid image rejected: %s\n", item.Link)
			}
		}
	}

	if len(validImageURLs) < targetCount {
		fmt.Printf("⚠️ Only found %d valid images out of target %d after %d attempts\n", len(validImageURLs), targetCount, maxAttempts)
		// Return what we have instead of error
		if len(validImageURLs) == 0 {
			return nil, fmt.Errorf("gagal menemukan gambar valid untuk '%s' setelah %d percobaan", query, maxAttempts)
		}
	}
	return validImageURLs, nil
}

// return 3 image URLs dengan retry mechanism yang agresif
func GetGoogleImages3(query string) ([]string, error) {
	// Mock logic: always mock if staging, or if production but disabled
	if config.IsStaging || (config.IsProduction && !config.IsGoogleAPIEnabled) {
		key := strings.ToLower(strings.TrimSpace(query))
		if key == "" {
			key = "kampus"
		}
		if images, ok := mockImageCatalog[key]; ok {
			return images, nil
		}
		if images, ok := mockImageCatalog["kampus"]; ok {
			return images, nil
		}
		return []string{}, nil
	}

	apiKey := config.GoogleAPIKey
	cx := config.GoogleAPI_CX

	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("google API Key atau CX tidak ditemukan")
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
	}
	validImageURLs := []string{}
	const (
		targetCount = 3
		maxAttempts = 10 // Increase attempts untuk dapat 3 gambar
		baseFetch   = 5  // Fetch more images per attempt
	)

	for attempt := 0; attempt < maxAttempts && len(validImageURLs) < targetCount; attempt++ {
		numToFetch := baseFetch + attempt
		if numToFetch > 10 {
			numToFetch = 10
		}

		// Use different start index to get variety
		startIndex := 1 + (attempt * 3)
		if startIndex > 90 {
			startIndex = 1
		}

		items, err := fetchRawGoogleImages(query, apiKey, cx, numToFetch, startIndex)
		if err != nil || len(items) < 1 {
			continue // Try next attempt
		}

		links := make([]string, len(items))
		for i, itm := range items {
			links[i] = itm.Link
		}
		fmt.Printf("[Attempt %d] fetched %d links from start %d: %v\n", attempt+1, len(links), startIndex, links)

		// Process all items, not just tail
		for _, item := range items {
			if validateImageURL(client, item.Link) {
				dup := false
				for _, u := range validImageURLs {
					if u == item.Link {
						dup = true
						break
					}
				}
				if !dup {
					validImageURLs = append(validImageURLs, item.Link)
					fmt.Printf("✅ Valid image %d: %s\n", len(validImageURLs), item.Link)
					if len(validImageURLs) == targetCount {
						break
					}
				}
			} else {
				fmt.Printf("❌ Invalid image rejected: %s\n", item.Link)
			}
		}
	}

	if len(validImageURLs) < targetCount {
		fmt.Printf("⚠️ Only found %d valid images out of target %d after %d attempts\n", len(validImageURLs), targetCount, maxAttempts)
		// Return what we have instead of error
		if len(validImageURLs) == 0 {
			return nil, fmt.Errorf("gagal menemukan gambar valid untuk '%s' setelah %d percobaan", query, maxAttempts)
		}
	}
	return validImageURLs, nil
}

// Struct yang diperlukan untuk compatibility dengan system yang ada
type ImageSearchResult struct {
	Title        string `json:"title"`
	ImageURL     string `json:"image_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	SourceURL    string `json:"source_url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// Service wrapper untuk compatibility dengan existing controller
type GoogleImageService struct {
	enabled bool
	client  *http.Client
}

func NewGoogleImageService() *GoogleImageService {
	apiKey := config.GoogleAPIKey
	cx := config.GoogleAPI_CX
	enabled := apiKey != "" && cx != ""

	return &GoogleImageService{
		enabled: enabled,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (s *GoogleImageService) IsEnabled() bool {
	return s.enabled
}

// SearchImagesForChat untuk ws.go dengan context - return 3 gambar berdasarkan kata kunci
func (s *GoogleImageService) SearchImagesForChat(ctx interface{}, searchTerm string) ([]ImageSearchResult, error) {
	if !s.enabled {
		return nil, fmt.Errorf("google image service not enabled")
	}

	term := strings.TrimSpace(searchTerm)
	if term == "" {
		term = "kampus"
	}

	// Menggunakan GetGoogleImages3 untuk mendapatkan 3 gambar sesuai query
	urls, err := GetGoogleImages3(term)
	if err != nil {
		return nil, err
	}

	// Convert ke format ImageSearchResult
	results := make([]ImageSearchResult, len(urls))
	for i, url := range urls {
		results[i] = ImageSearchResult{
			Title:        fmt.Sprintf("%s #%d", humanizeQuery(term), i+1),
			ImageURL:     url,
			ThumbnailURL: url,
			SourceURL:    url,
			Width:        800,
			Height:       600,
		}
	}

	return results, nil
}

// SearchImages method untuk images controller - return 3 gambar
func (s *GoogleImageService) SearchImages(ctx interface{}, query string, maxResults int) ([]ImageSearchResult, error) {
	if !s.enabled {
		return nil, fmt.Errorf("google image service not enabled")
	}

	term := strings.TrimSpace(query)
	if term == "" {
		term = "kampus"
	}

	// Menggunakan GetGoogleImages3 untuk mendapatkan gambar sesuai query
	urls, err := GetGoogleImages3(term)
	if err != nil {
		return nil, err
	}

	// Limit results jika diminta
	if maxResults > 0 && len(urls) > maxResults {
		urls = urls[:maxResults]
	}

	// Convert ke format ImageSearchResult
	results := make([]ImageSearchResult, len(urls))
	for i, url := range urls {
		results[i] = ImageSearchResult{
			Title:        fmt.Sprintf("%s #%d", humanizeQuery(term), i+1),
			ImageURL:     url,
			ThumbnailURL: url,
			SourceURL:    url,
			Width:        800,
			Height:       600,
		}
	}

	return results, nil
}

func humanizeQuery(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "Kampus"
	}
	words := strings.Fields(q)
	for i, w := range words {
		upper := strings.ToUpper(w)
		if len(w) <= 5 && w == upper {
			words[i] = upper
			continue
		}
		r := []rune(strings.ToLower(w))
		if len(r) == 0 {
			continue
		}
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// ExtractSearchTermFromContext method untuk images controller
func (s *GoogleImageService) ExtractSearchTermFromContext(message string) string {
	// Selalu return UIB query untuk konsistensi dengan example project
	return "University International Batam"
}
