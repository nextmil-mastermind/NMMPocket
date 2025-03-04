package event

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

var PbApp *pocketbase.PocketBase

type Order struct {
	Reference   string  `db:"reference"`
	FirstName   string  `db:"first_name"`
	LastName    string  `db:"last_name"`
	Email       string  `db:"email"`
	EventRef    string  `db:"event_ref"`
	Total       float64 `db:"total"`
	Refunded    bool    `db:"refunded"`
	CreatedAt   string  `db:"date"`
	TicketCount int     `db:"ticket_count"`
	From        string  `db:"from,omitempty"`
	TicketUrl   string  `db:"ticket_url,omitempty"`
}

type Ticket struct {
	Reference   string `db:"reference"`
	FirstName   string `db:"first_name"`
	LastName    string `db:"last_name"`
	Email       string `db:"email"`
	EventRef    string `db:"event_ref"`
	orderID     string `db:"order_id"`
	TicketID    int    `db:"ticket_id"`
	IsCancelled bool   `db:"is_cancelled"`
	HasArrived  bool   `db:"has_arrived"`
	ArrivalAt   string `db:"arrival,omitempty"`
	CreatedAt   string `db:"date"`
}

type Checkout struct {
	SessionID   string `db:"session_id"`
	FirstName   string `db:"first_name"`
	LastName    string `db:"last_name"`
	Email       string `db:"email"`
	HasExPeople bool   `db:"HasExPeople"`
	ExtraPeople string `db:"ExtraPeople,omitempty"`
	Phone       string `db:"phone,omitempty"`
	OrderURL    string `db:"orderUrl,omitempty"`
	Processed   bool   `db:"processed"`
	From        string `db:"from,omitempty"`
	Type        string `db:"type,omitempty"`
	EventRef    string `db:"event_ref"`
	TicketID    int    `db:"ticket_id"`
	Indentifier string `db:"identifier,omitempty"`
	DateAdded   string `db:"date_added"`
}

type Event struct {
	Reference     string           `db:"reference" json:"reference"`
	Title         string           `db:"title" json:"title"`
	URL           string           `db:"url" json:"url"`
	StartTime     string           `db:"start_time" json:"start_time"`
	EndTime       string           `db:"end_time" json:"end_time"`
	Venue         string           `db:"venue" json:"venue"`
	Address       string           `db:"address" json:"address"`
	Paid          bool             `db:"paid" json:"paid"`
	ClientRefID   string           `db:"client_reference_id" json:"client_reference_id"`
	Type          string           `db:"type" json:"type"`
	Languange     string           `db:"languange" json:"languange"`
	ResponseFound string           `db:"response_found" json:"response_found"`
	ResponseWait  string           `db:"response_wait" json:"response_wait"`
	Duplicate     string           `db:"duplicate" json:"duplicate"`
	TicketTypes   []map[string]any `db:"ticket_types" json:"ticket_types"`
	Coupons       []map[string]any `db:"coupons" json:"coupons"`
	Billing       bool             `db:"billing" json:"billing"`
	Member        float64          `db:"member" json:"member"`
	GTAG          string           `db:"gtag" json:"gtag"`
}

// FromRecord converts a Record to an Event.
// It populates the Event fields based on the Record's data.
// This method is useful for converting database records into Event objects.
// It returns an error if any field conversion fails.
func (e *Event) FromRecord(record *core.Record) error {
	e.Reference = record.GetString("reference")
	e.Title = record.GetString("title")
	e.URL = record.GetString("url")
	e.StartTime = record.GetString("start_time")
	e.EndTime = record.GetString("end_time")
	e.Venue = record.GetString("venue")
	e.Paid = record.GetBool("paid")
	e.ClientRefID = record.GetString("client_reference_id")
	e.Type = record.GetString("type")
	e.Languange = record.GetString("languange")
	e.ResponseFound = record.GetString("response_found")
	e.ResponseWait = record.GetString("response_wait")
	e.Duplicate = record.GetString("duplicate")
	e.Address = record.GetString("address")
	if err := record.UnmarshalJSONField("ticket_types", &e.TicketTypes); err != nil {
		return err
	}
	if err := record.UnmarshalJSONField("coupons", &e.Coupons); err != nil {
		return err
	}
	e.Billing = record.GetBool("billing")
	e.Member = record.GetFloat("member")
	e.GTAG = record.GetString("gtag")
	return nil
}
