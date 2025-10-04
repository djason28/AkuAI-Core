package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"AkuAI/models"
)

type UIBEventService struct {
	eventsData *models.UIBEventsData
}

func NewUIBEventService() (*UIBEventService, error) {
	service := &UIBEventService{}
	err := service.loadEventsData()
	if err != nil {
		return nil, fmt.Errorf("failed to load UIB events data: %w", err)
	}
	return service, nil
}

// loadEventsData loads the UIB events from JSON file
func (s *UIBEventService) loadEventsData() error {
	// Get the path to the data directory
	dataPath := filepath.Join("data", "uib_events.json")

	// Read the JSON file
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("error reading UIB events file: %w", err)
	}

	// Parse JSON
	s.eventsData = &models.UIBEventsData{}
	err = json.Unmarshal(data, s.eventsData)
	if err != nil {
		return fmt.Errorf("error parsing UIB events JSON: %w", err)
	}

	return nil
}

// GetAllEvents returns all UIB events
func (s *UIBEventService) GetAllEvents() []models.UIBEvent {
	var allEvents []models.UIBEvent

	allEvents = append(allEvents, s.eventsData.UIBEvents.October2025...)
	allEvents = append(allEvents, s.eventsData.UIBEvents.November2025...)
	allEvents = append(allEvents, s.eventsData.UIBEvents.December2025...)

	return allEvents
}

// SearchEvents searches events based on criteria
func (s *UIBEventService) SearchEvents(criteria models.EventSearchCriteria) []models.UIBEvent {
	allEvents := s.GetAllEvents()
	var filteredEvents []models.UIBEvent

	for _, event := range allEvents {
		// Filter by type
		if criteria.EventType != "" && event.Type != criteria.EventType {
			continue
		}

		// Filter by month
		if criteria.Month != "" {
			eventDate, err := time.Parse("2006-01-02", event.Date)
			if err != nil {
				continue
			}
			eventMonth := strings.ToLower(eventDate.Format("January"))
			if !strings.Contains(eventMonth, criteria.Month) {
				continue
			}
		}

		// Filter by department
		if criteria.Department != "" &&
			!strings.Contains(strings.ToLower(event.Department), strings.ToLower(criteria.Department)) {
			continue
		}

		// Filter by free events only
		if criteria.FreeOnly {
			if !s.isFreeEvent(event) {
				continue
			}
		}

		filteredEvents = append(filteredEvents, event)
	}

	return filteredEvents
}

// GetEventByID returns a specific event by ID
func (s *UIBEventService) GetEventByID(eventID string) (*models.UIBEvent, error) {
	allEvents := s.GetAllEvents()

	for _, event := range allEvents {
		if event.ID == eventID {
			return &event, nil
		}
	}

	return nil, fmt.Errorf("event with ID %s not found", eventID)
}

// GetEventsByMonth returns events for a specific month
func (s *UIBEventService) GetEventsByMonth(month string) []models.UIBEvent {
	month = strings.ToLower(month)

	switch month {
	case "october", "oktober":
		return s.eventsData.UIBEvents.October2025
	case "november":
		return s.eventsData.UIBEvents.November2025
	case "december", "desember":
		return s.eventsData.UIBEvents.December2025
	default:
		return []models.UIBEvent{}
	}
}

// GetEventsByType returns events of a specific type
func (s *UIBEventService) GetEventsByType(eventType string) []models.UIBEvent {
	allEvents := s.GetAllEvents()
	var filteredEvents []models.UIBEvent

	for _, event := range allEvents {
		if event.Type == eventType {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return filteredEvents
}

// GetUpcomingEvents returns events from today onwards
func (s *UIBEventService) GetUpcomingEvents() []models.UIBEvent {
	allEvents := s.GetAllEvents()
	var upcomingEvents []models.UIBEvent
	now := time.Now()

	for _, event := range allEvents {
		eventDate, err := time.Parse("2006-01-02", event.Date)
		if err != nil {
			continue
		}

		if eventDate.After(now) || eventDate.Equal(now.Truncate(24*time.Hour)) {
			upcomingEvents = append(upcomingEvents, event)
		}
	}

	return upcomingEvents
}

// GetEventSummaries returns summary of all events
func (s *UIBEventService) GetEventSummaries() []models.EventSummary {
	allEvents := s.GetAllEvents()
	var summaries []models.EventSummary

	for _, event := range allEvents {
		summary := models.EventSummary{
			ID:         event.ID,
			Type:       event.Type,
			Title:      event.Title,
			Date:       event.Date,
			Time:       event.Time,
			Department: event.Department,
			IsFree:     s.isFreeEvent(event),
			Mark:       event.Mark,
		}
		summaries = append(summaries, summary)
	}

	return summaries
}

// FormatEventsForGemini formats events data for Gemini AI context
func (s *UIBEventService) FormatEventsForGemini(events []models.UIBEvent) string {
	if len(events) == 0 {
		return "Tidak ada data acara UIB yang tersedia untuk periode yang diminta."
	}

	var formatted strings.Builder
	formatted.WriteString("=== DATA RESMI UNIVERSITAS INTERNASIONAL BATAM (UIB) ===\n")
	formatted.WriteString("MARK: UIB_OFFICIAL - Data akurat dan terpercaya\n")
	formatted.WriteString("TANGGAL SEKARANG: 4 Oktober 2025\n")
	formatted.WriteString("INSTRUKSI: Langsung berikan SEMUA data yang tersedia, jangan tanya balik\n\n")

	// Group by month
	monthEvents := make(map[string][]models.UIBEvent)
	for _, event := range events {
		eventDate, err := time.Parse("2006-01-02", event.Date)
		if err != nil {
			continue
		}
		monthName := strings.ToUpper(eventDate.Format("January 2006"))
		monthEvents[monthName] = append(monthEvents[monthName], event)
	}

	for month, monthEventsList := range monthEvents {
		formatted.WriteString(fmt.Sprintf("📅 %s:\n", month))

		for _, event := range monthEventsList {
			formatted.WriteString(fmt.Sprintf("\n🎯 %s - %s\n", strings.ToUpper(event.Type), event.Title))
			formatted.WriteString(fmt.Sprintf("   📍 Tanggal: %s", event.Date))
			if event.Time != "" {
				formatted.WriteString(fmt.Sprintf(" | ⏰ Waktu: %s", event.Time))
			}
			formatted.WriteString("\n")

			if event.Location != "" {
				formatted.WriteString(fmt.Sprintf("   🏢 Lokasi: %s\n", event.Location))
			}
			if event.Platform != "" {
				formatted.WriteString(fmt.Sprintf("   💻 Platform: %s\n", event.Platform))
			}

			formatted.WriteString(fmt.Sprintf("   🏛️  Departemen: %s\n", event.Department))
			formatted.WriteString(fmt.Sprintf("   📋 Deskripsi: %s\n", event.Description))

			if event.Speaker != "" {
				formatted.WriteString(fmt.Sprintf("   🎤 Pembicara: %s\n", event.Speaker))
			}
			if event.Requirements != "" {
				formatted.WriteString(fmt.Sprintf("   📋 Persyaratan: %s\n", event.Requirements))
			}
			if event.RegistrationFee != "" {
				formatted.WriteString(fmt.Sprintf("   💰 Biaya: %s\n", event.RegistrationFee))
			}
			if event.Contact != "" {
				formatted.WriteString(fmt.Sprintf("   📞 Kontak: %s\n", event.Contact))
			}

			formatted.WriteString("   ✅ STATUS: UIB_OFFICIAL (Data Resmi UIB)\n")
		}
		formatted.WriteString("\n")
	}

	formatted.WriteString("📞 Kontak Umum UIB: info@uib.ac.id\n")
	formatted.WriteString("🌐 Website: https://uib.ac.id\n")
	formatted.WriteString("\n=== CONTOH FORMAT JAWABAN YANG DIINGINKAN ===\n")
	formatted.WriteString("Contoh: Jika ditanya 'sertifikasi November 2025 UIB?'\n")
	formatted.WriteString("Jawab: 'Berikut sertifikasi UIB untuk November 2025 (Data resmi UIB_OFFICIAL):'\n")
	formatted.WriteString("1. [Nama Sertifikasi] - [Tanggal] - [Biaya] - [Kontak]'\n")
	formatted.WriteString("2. [dst...] - LANGSUNG berikan semua, jangan tanya balik!\n")
	formatted.WriteString("\n=== AKHIR DATA UIB ===\n")

	return formatted.String()
}

// AnalyzeQueryForUIB analyzes if a query is related to UIB
func (s *UIBEventService) AnalyzeQueryForUIB(query string) bool {
	uibKeywords := []string{
		"uib", "universitas internasional batam", "batam",
		"sertifikasi uib", "webinar uib", "acara uib",
		"oktober uib", "november uib", "desember uib",
		"pendaftaran uib", "event uib", "kegiatan uib",
	}

	queryLower := strings.ToLower(query)
	for _, keyword := range uibKeywords {
		if strings.Contains(queryLower, keyword) {
			return true
		}
	}

	return false
}

// GetRelevantEventsForQuery returns events relevant to a specific query
func (s *UIBEventService) GetRelevantEventsForQuery(query string) []models.UIBEvent {
	queryLower := strings.ToLower(query)
	allEvents := s.GetAllEvents()
	var relevantEvents []models.UIBEvent

	for _, event := range allEvents {
		// Check if query matches event content
		if s.isEventRelevantToQuery(event, queryLower) {
			relevantEvents = append(relevantEvents, event)
		}
	}

	// If no specific matches, return upcoming events
	if len(relevantEvents) == 0 && s.AnalyzeQueryForUIB(query) {
		return s.GetUpcomingEvents()
	}

	return relevantEvents
}

// Helper function to check if event is free
func (s *UIBEventService) isFreeEvent(event models.UIBEvent) bool {
	if event.RegistrationFee == "" {
		return true
	}

	fee := strings.ToLower(event.RegistrationFee)
	return strings.Contains(fee, "gratis") || strings.Contains(fee, "free") || fee == "0" || fee == "rp 0"
}

// Helper function to check if event is relevant to query
func (s *UIBEventService) isEventRelevantToQuery(event models.UIBEvent, queryLower string) bool {
	searchableFields := []string{
		strings.ToLower(event.Title),
		strings.ToLower(event.Description),
		strings.ToLower(event.Department),
		strings.ToLower(event.Type),
		strings.ToLower(event.Speaker),
		event.Date,
	}

	for _, field := range searchableFields {
		if strings.Contains(field, queryLower) {
			return true
		}
	}

	// Check for month names
	if strings.Contains(queryLower, "oktober") && strings.Contains(event.Date, "2025-10") {
		return true
	}
	if strings.Contains(queryLower, "november") && strings.Contains(event.Date, "2025-11") {
		return true
	}
	if strings.Contains(queryLower, "desember") && strings.Contains(event.Date, "2025-12") {
		return true
	}

	return false
}
