package lib

import (
	"os"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

var (
	send_name   = os.Getenv("BREVO_SENDER_NAME")
	send_email  = os.Getenv("BREVO_SENDER_EMAIL")
	brevo_key   = os.Getenv("BREVO_API_KEY")
	brevo_reply = os.Getenv("BREVO_REPLY")
)

type Sender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ReplyTo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type MessageVersion struct {
	To     []Contact      `json:"to"`
	Params map[string]any `json:"params"`
}

type EmailData struct {
	Sender          Contact            `json:"sender"`
	Subject         string             `json:"subject"`
	HTMLContent     string             `json:"htmlContent"`
	MessageVersions []MessageVersion   `json:"messageVersions"`
	ReplyTo         Contact            `json:"replyTo"`
	ScheduledAt     *string            `json:"scheduledAt,omitempty"`
	Attachment      *[]BrevoAttachment `json:"attachment,omitempty"`
}
type BrevoAttachment struct {
	URL     *string `json:"url,omitempty"`
	Name    string  `json:"name"`
	Content *string `json:"content,omitempty"`
}

// Recipient represents an email recipient.
type Recipient struct {
	Name      string
	Email     string
	FirstName string
	Params    *map[string]any
}

// Contact represents a sender, replyTo, or recipient in the request.
type Contact struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	FirstName string `json:"first_name,omitempty"` // optional, used for personalized emails
}

// OrderData represents the order details.
type OrderData struct {
	FirstName      string
	LastName       string
	Email          string
	EventRef       string
	Title          string    // event title
	Venue          string    // event venue
	StartTime      time.Time // event start time
	Tickets        []map[string]any
	OrderReference string
	Address        *string // optional
	Total          float64
	Type           string // event type
}

// ScheduledJob represents a scheduled job in the system.
type ScheduledJob struct {
	Collection string          `json:"collection"`
	Filter     string          `json:"filter"`
	Function   string          `json:"function"`
	RunAt      time.Time       `json:"run_at"`
	Done       bool            `json:"done"`
	LastRun    time.Time       `json:"last_run"`
	Email      string          `json:"email_template"`
	Params     *map[string]any `json:"params,omitempty"`
	Record     *core.Record
}

func (s *ScheduledJob) MarkDone(app *pocketbase.PocketBase) error {
	s.Done = true
	s.LastRun = time.Now().UTC()
	return app.Save(s.Record)
}

func (s *ScheduledJob) FromRecord(record *core.Record) {
	s.Collection = record.GetString("collection")
	s.Filter = record.GetString("filter")
	s.Function = record.GetString("function")
	runAt := record.GetDateTime("run_at")
	s.RunAt = runAt.Time()
	s.Done = record.GetBool("done")
	lastRun := record.GetDateTime("last_run")
	s.LastRun = lastRun.Time()
	s.Email = record.GetString("email_template")
	if params := record.Get("params"); params != nil {
		if paramMap, ok := params.(map[string]any); ok {
			s.Params = &paramMap
		}
	}
	s.Record = record
}
