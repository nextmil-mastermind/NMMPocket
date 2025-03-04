package emailsender

import "time"

// Recipient represents an email recipient.
type Recipient struct {
	Name   string
	Email  string
	Ticket string // optional ticket field
}

// Contact represents a sender, replyTo, or recipient in the request.
type Contact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// MessageVersion represents one message version for a recipient.
type MessageVersion struct {
	To     []Contact      `json:"to"`
	Params map[string]any `json:"params"`
}

// EmailData is the payload sent to the API.
type EmailData struct {
	Sender          Contact          `json:"sender"`
	ReplyTo         Contact          `json:"replyTo"`
	Subject         string           `json:"subject"`
	HtmlContent     string           `json:"htmlContent"`
	MessageVersions []MessageVersion `json:"messageVersions"`
	Attachment      *string          `json:"attachment,omitempty"`
}

// OrderData represents the order details.
type OrderData struct {
	FirstName      string
	LastName       string
	Email          string
	Title          string    // event title
	Venue          string    // event venue
	StartTime      time.Time // event start time
	Tickets        []map[string]any
	OrderReference string
	Address        *string // optional
}
