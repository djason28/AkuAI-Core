package services

import (
	"encoding/json"
	"fmt"
	"log"
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

// AnalyzeQueryForUIB analyzes if a query is related to UIB EVENTS/ACTIVITIES (not general info like jurusan)
func (s *UIBEventService) AnalyzeQueryForUIB(query string) bool {
	queryLower := strings.ToLower(query)

	// If query is about jurusan/fakultas/program studi, use pure Gemini instead
	jurusanKeywords := []string{
		"jurusan", "fakultas", "program studi", "prodi",
		"teknik informatika", "sistem informasi", "manajemen",
		"akuntansi", "hukum", "psikologi", "komunikasi",
		"apa saja jurusan", "daftar jurusan", "berikan jurusan",
	}

	for _, keyword := range jurusanKeywords {
		if strings.Contains(queryLower, keyword) {
			log.Printf("[uib-service] 🎓 Jurusan query detected - using pure Gemini: %s", query)
			return false // Use pure Gemini for academic program info
		}
	}

	// Check for UIB-specific keywords
	uibDirectKeywords := []string{
		"sertifikasi uib", "webinar uib", "acara uib",
		"oktober uib", "november uib", "desember uib",
		"pendaftaran uib", "event uib", "kegiatan uib",
		"seminar uib", "workshop uib", "pelatihan uib",
		"universitas internasional batam",
	}

	for _, keyword := range uibDirectKeywords {
		if strings.Contains(queryLower, keyword) {
			return true
		}
	}

	// UIB general info (kontak/website) – still use UIB metadata context
	if strings.Contains(queryLower, "uib") {
		contactKeys := []string{"kontak", "email", "website", "situs", "alamat", "telepon", "hubungi"}
		for _, k := range contactKeys {
			if strings.Contains(queryLower, k) {
				return true
			}
		}
	}

	// Also detect general webinar/sertifikasi queries for Oct–Dec 2025
	// since we have UIB data for that period
	monthEventPatterns := []string{
		"webinar november", "sertifikasi november",
		"acara november", "event november",
		"seminar november", "workshop november", "nov ",
		"webinar oktober", "sertifikasi oktober",
		"webinar desember", "sertifikasi desember", "okt ", "des ",
		"november 2025", "oktober 2025", "desember 2025",
	}

	for _, pattern := range monthEventPatterns {
		if strings.Contains(queryLower, pattern) {
			return true
		}
	}

	// Numeric month detection (10,11,12) optionally with year 2025
	// e.g., "bulan 11", "bln 10", "11 2025", "nov 2025" handled above; here we catch pure numeric
	if containsNumericMonth(queryLower) {
		return true
	}

	// Heuristic: default to UIB if query talks about events and no other university is explicitly mentioned
	genericEventKeys := []string{"acara", "event", "seminar", "webinar", "sertifikasi", "pelatihan", "workshop"}
	otherCampusHints := []string{"universitas indonesia", "ui ", "ugm", "gadjah mada", "itb", "ipb", "airlangga", "binus"}
	hasEventWord := false
	for _, k := range genericEventKeys {
		if strings.Contains(queryLower, k) {
			hasEventWord = true
			break
		}
	}
	mentionsOther := false
	for _, o := range otherCampusHints {
		if strings.Contains(queryLower, o) {
			mentionsOther = true
			break
		}
	}
	if hasEventWord && !mentionsOther {
		return true
	}

	// Event title detection: if user mentions an event title (or a strong substring), treat as UIB-related
	// This helps queries like "Sertifikasi Digital Marketing for Business" without saying UIB
	for _, ev := range s.GetAllEvents() {
		titleLower := strings.ToLower(strings.TrimSpace(ev.Title))
		if titleLower == "" {
			continue
		}
		if strings.Contains(queryLower, titleLower) || strings.Contains(titleLower, strings.TrimSpace(queryLower)) {
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

	monthPrefixes := detectMonthPrefixes(queryLower)
	requiredType := detectEventType(queryLower)

	// Relative range detection (e.g., minggu depan)
	if start, end, ok := detectRelativeRange(queryLower, time.Now()); ok {
		// prefilter by date range
		ranged := make([]models.UIBEvent, 0)
		for _, ev := range allEvents {
			evDate, err := time.Parse("2006-01-02", ev.Date)
			if err != nil {
				continue
			}
			if (evDate.Equal(start) || evDate.After(start)) && (evDate.Equal(end) || evDate.Before(end)) {
				ranged = append(ranged, ev)
			}
		}
		allEvents = ranged
	}

	for _, event := range allEvents {
		datePrefix := ""
		if len(event.Date) >= 7 {
			datePrefix = strings.ToLower(event.Date[:7])
		}
		if len(monthPrefixes) > 0 && (datePrefix == "" || !monthPrefixes[datePrefix]) {
			continue
		}
		// If both webinar and certification are requested, don't filter by single type
		if requiredType != "" && requiredType != "both" && !strings.EqualFold(event.Type, requiredType) {
			continue
		}
		if s.isEventRelevantToQuery(event, queryLower) {
			relevantEvents = append(relevantEvents, event)
		}
	}

	if len(relevantEvents) == 0 {
		for _, event := range allEvents {
			datePrefix := ""
			if len(event.Date) >= 7 {
				datePrefix = strings.ToLower(event.Date[:7])
			}
			if len(monthPrefixes) > 0 && (datePrefix == "" || !monthPrefixes[datePrefix]) {
				continue
			}
			if requiredType != "" && requiredType != "both" && !strings.EqualFold(event.Type, requiredType) {
				continue
			}
			if len(monthPrefixes) > 0 || requiredType != "" {
				relevantEvents = append(relevantEvents, event)
			}
		}
	}

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
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if strings.Contains(field, queryLower) || strings.Contains(queryLower, field) {
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

func detectMonthPrefixes(queryLower string) map[string]bool {
	monthMap := map[string][]string{
		"2025-10": {"okt", "oktober", "october"},
		"2025-11": {"nov", "november"},
		"2025-12": {"des", "desember", "december"},
	}

	result := make(map[string]bool)
	for prefix, keywords := range monthMap {
		for _, keyword := range keywords {
			if strings.Contains(queryLower, keyword) {
				result[strings.ToLower(prefix)] = true
				break
			}
		}
	}

	// Numeric month handling: 10, 11, 12 (optionally preceded by "bulan"/"bln")
	// We keep it simple and robust for Indonesian queries
	if containsNumericMonth(queryLower) {
		if strings.Contains(queryLower, "10") {
			result["2025-10"] = true
		}
		if strings.Contains(queryLower, "11") {
			result["2025-11"] = true
		}
		if strings.Contains(queryLower, "12") {
			result["2025-12"] = true
		}
	}

	return result
}

func containsNumericMonth(s string) bool {
	// normalize whitespace
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	// split and strip trailing punctuation from tokens
	tokens := strings.Fields(s)
	clean := func(tok string) string {
		tok = strings.TrimSpace(tok)
		// strip common trailing punctuation
		tok = strings.Trim(tok, "?!,:.;()[]{}")
		return tok
	}
	for i, tok := range tokens {
		ct := clean(tok)
		if ct == "bulan" || ct == "bln" {
			if i+1 < len(tokens) {
				nt := clean(tokens[i+1])
				if nt == "10" || nt == "11" || nt == "12" {
					return true
				}
			}
		}
		if ct == "10" || ct == "11" || ct == "12" { // tolerate bare numeric month
			return true
		}
	}
	return false
}

func detectEventType(queryLower string) string {
	hasWeb := strings.Contains(queryLower, "webinar") || strings.Contains(queryLower, "seminar") || strings.Contains(queryLower, "talkshow") || strings.Contains(queryLower, "kuliah umum")

	certKeywords := []string{
		"sertifikasi",
		"certification",
		"certificate",
		"pelatihan",
		"workshop",
		"bootcamp",
	}
	hasCert := false
	for _, keyword := range certKeywords {
		if strings.Contains(queryLower, keyword) {
			hasCert = true
			break
		}
	}

	if hasWeb && hasCert {
		return "both"
	}
	if hasWeb {
		return "webinar"
	}
	if hasCert {
		return "certification"
	}
	return ""
}

// detectRelativeRange recognizes phrases like "minggu depan" and returns an inclusive [start, end] range
func detectRelativeRange(queryLower string, base time.Time) (time.Time, time.Time, bool) {
	// Normalize base to midnight
	base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location())
	if strings.Contains(queryLower, "minggu depan") || strings.Contains(queryLower, "pekan depan") {
		// find next Monday from base, then range Monday..Sunday
		// weekday: Monday=1 ... Sunday=0 in Go with Weekday()
		wd := int(base.Weekday()) // Sunday=0
		daysUntilNextMon := (8 - wd) % 7
		if daysUntilNextMon == 0 {
			daysUntilNextMon = 7
		}
		start := base.AddDate(0, 0, daysUntilNextMon)
		// end Sunday of that week
		end := start.AddDate(0, 0, 6)
		return start, end, true
	}
	return time.Time{}, time.Time{}, false
}
