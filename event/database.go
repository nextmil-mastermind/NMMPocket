package event

import (
	dab "nmmpocket/database"
	"strconv"
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
func UpdateOrder(WhereCol string, WhereString string, WhatCols []string, order Order) error {
	db := dab.Pg
	whatColsString := ""
	for i, col := range WhatCols {
		if i == 0 {
			whatColsString += col + " = $" + strconv.Itoa(i+1)
		} else {
			whatColsString += ", " + col + " = $" + strconv.Itoa(i+1)
		}
	}
	statement, err := db.Prepare("UPDATE orders SET " + whatColsString + " WHERE " + WhereCol + " = \"" + WhereString + "\"")
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
