package zoomcon

import (
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

type MeetingResponse struct {
	Occurrences []MeetingOccurrence `json:"occurrences"`
}
type MeetingOccurrence struct {
	OccurrenceId string    `json:"occurrence_id"`
	StartTime    time.Time `json:"start_time"`
	Duration     int       `json:"duration"`
	Status       string    `json:"status"`
}
type Meeting struct {
	StartURL  string `json:"start_url"`
	Topic     string `json:"topic"`
	StartTime string `json:"start_time"` //2022-03-25T07:29:29Z RFC3339
	Duration  int    `json:"duration"`   // in minutes
}

type MemberReduced struct {
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Company    string    `json:"company"`
	Email      string    `json:"email"`
	Expiration time.Time `json:"expiration"`
	Group      string    `json:"group"`
	ID         string    `json:"id"`
	Phone      string    `json:"phone"`
}

func (m *MemberReduced) FromRecord(record *core.Record) {
	m.FirstName = record.Get("first_name").(string)
	m.LastName = record.Get("last_name").(string)
	m.Company = record.Get("company").(string)
	m.Email = record.Get("email").(string)
	m.Expiration = record.GetDateTime("expiration").Time()
	m.Group = record.Get("group").(string)
	m.ID = record.Get("id").(string)
	m.Phone = record.Get("phone").(string)
}

type MemberRegistration struct {
	MemberID  string
	JoinURL   string
	StartTime time.Time
	EndTime   time.Time
	Title     string
}

func (m *MemberRegistration) FromRecord(record *core.Record) {
	m.MemberID = record.Get("member").(string)
	m.JoinURL = record.Get("join_url").(string)
	m.StartTime = record.Get("start").(time.Time)
	m.EndTime = record.Get("end").(time.Time)
	m.Title = record.Get("title").(string)
}

func (m *MemberRegistration) Create(app *pocketbase.PocketBase) error {
	collection, err := app.FindCollectionByNameOrId("member_zoom")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("member", m.MemberID)
	record.Set("join_url", m.JoinURL)
	record.Set("start", m.StartTime)
	record.Set("end", m.EndTime)
	record.Set("title", m.Title)
	return app.Save(record)
}

type BatchRegistrationResult struct {
	Registrants []BatchRegistrantResponse `json:"registrants"`
}

type BatchRegistrantResponse struct {
	RegistrantID string `json:"registrant_id"`
	Email        string `json:"email"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	JoinURL      string `json:"join_url"`
}

// RegistrantStatus represents a single registrant (ID + Email) used when
// batching status updates to Zoom.
type RegistrantStatus struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UpdateRegistrantStatusRequest struct {
	Action      string             `json:"action"`
	Registrants []RegistrantStatus `json:"registrants"`
}
