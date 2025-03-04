package event

import (
	dab "nmmpocket/database"
)

func InsertOrder(order Order) error {
	db := dab.Pg
	statement, err := db.Prepare("INSERT INTO orders (reference, first_name, last_name, email, event_ref, total, refunded, date, ticket_count) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)")
	if err != nil {
		return err
	}
	_, err = statement.Exec(order.Reference, order.FirstName, order.LastName, order.Email, order.EventRef, order.Total, order.Refunded, order.CreatedAt, order.TicketCount)
	if err != nil {
		return err
	}
	return nil
}
func InsertTicket(ticket Ticket) error {
	db := dab.Pg
	statement, err := db.Prepare("INSERT INTO tickets (reference, first_name, last_name, email, event_ref, order_id, ticket_id, is_cancelled, has_arrived, arrival, date) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")
	if err != nil {
		return err
	}
	_, err = statement.Exec(ticket.Reference, ticket.FirstName, ticket.LastName, ticket.Email, ticket.EventRef, ticket.orderID, ticket.TicketID, ticket.IsCancelled, ticket.HasArrived, ticket.ArrivalAt)
	if err != nil {
		return err
	}
	return nil
}

func selectCheckout(sessionID string) (Checkout, error) {
	db := dab.Pg
	statement, err := db.Prepare("SELECT * FROM checkout WHERE session_id = $1")
	if err != nil {
		return Checkout{}, err
	}
	row := statement.QueryRow(sessionID)
	var checkout Checkout
	err = row.Scan(&checkout.SessionID, &checkout.FirstName, &checkout.LastName, &checkout.Email, &checkout.HasExPeople, &checkout.ExtraPeople, &checkout.Phone, &checkout.OrderURL, &checkout.Processed, &checkout.From, &checkout.Type, &checkout.EventRef, &checkout.TicketID, &checkout.Indentifier, &checkout.DateAdded)
	if err != nil {
		return Checkout{}, err
	}
	return checkout, nil
}
