package event

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
	HasExPeople string `db:"HasExPeople"`
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
