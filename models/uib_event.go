package models

import (
	"time"
)

// UIBEvent represents a UIB event (certification or webinar)
type UIBEvent struct {
	ID               string `json:"id"`
	Type             string `json:"type"` // "certification" or "webinar"
	Title            string `json:"title"`
	Date             string `json:"date"`
	Time             string `json:"time,omitempty"`
	Location         string `json:"location,omitempty"`
	Platform         string `json:"platform,omitempty"`
	Institution      string `json:"institution"`
	Department       string `json:"department"`
	Description      string `json:"description"`
	Speaker          string `json:"speaker,omitempty"`
	Requirements     string `json:"requirements,omitempty"`
	RegistrationFee  string `json:"registration_fee,omitempty"`
	Contact          string `json:"contact,omitempty"`
	RegistrationLink string `json:"registration_link,omitempty"`
	Mark             string `json:"mark"` // "UIB_OFFICIAL"

	// Additional fields for certifications
	Certificate       string `json:"certificate,omitempty"`
	MaterialsIncluded string `json:"materials_included,omitempty"`
	TechStack         string `json:"tech_stack,omitempty"`
	FinalProject      string `json:"final_project,omitempty"`
	ProjectOutcome    string `json:"project_outcome,omitempty"`
	CertificationBody string `json:"certification_body,omitempty"`

	// Additional fields for webinars
	LiveQA                bool   `json:"live_qa,omitempty"`
	RecordingAvailable    bool   `json:"recording_available,omitempty"`
	InteractivePoll       bool   `json:"interactive_poll,omitempty"`
	CertificateAttendance bool   `json:"certificate_attendance,omitempty"`
	NetworkingSession     bool   `json:"networking_session,omitempty"`
	RegistrationDeadline  string `json:"registration_deadline,omitempty"`
}

// UIBEventsData represents the complete UIB events data structure
type UIBEventsData struct {
	UIBEvents struct {
		October2025  []UIBEvent `json:"october_2025"`
		November2025 []UIBEvent `json:"november_2025"`
		December2025 []UIBEvent `json:"december_2025"`
	} `json:"uib_events"`
	Metadata struct {
		LastUpdated    string `json:"last_updated"`
		TotalEvents    int    `json:"total_events"`
		Institution    string `json:"institution"`
		ContactGeneral string `json:"contact_general"`
		Website        string `json:"website"`
		Note           string `json:"note"`
	} `json:"metadata"`
}

// EventSearchCriteria for filtering events
type EventSearchCriteria struct {
	EventType  string // "certification", "webinar", or "" for all
	Month      string // "october", "november", "december", or "" for all
	DateFrom   time.Time
	DateTo     time.Time
	Department string
	FreeOnly   bool // true to filter only free events
}

// EventSummary for quick display
type EventSummary struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Date       string `json:"date"`
	Time       string `json:"time"`
	Department string `json:"department"`
	IsFree     bool   `json:"is_free"`
	Mark       string `json:"mark"`
}
