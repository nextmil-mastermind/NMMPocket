package event

import "github.com/pocketbase/pocketbase"

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
