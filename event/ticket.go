package event

import (
	"encoding/json"
	"math/rand"
	"nmmpocket/emailsender"
	"time"
)

func AddOrder(data Checkout, amount float64) (string, error) {
	var emailData emailsender.OrderData
	var orderID = generateID(14)
	var today = time.Now().Format("2006-01-02 15:04:05")
	order := Order{
		Reference:   orderID,
		FirstName:   data.FirstName,
		LastName:    data.LastName,
		Email:       data.Email,
		EventRef:    data.EventRef,
		Total:       amount,
		Refunded:    false,
		CreatedAt:   today,
		TicketCount: 0,
		From:        data.From,
		TicketUrl:   "",
	}
	emailData = emailsender.OrderData{
		FirstName:      data.FirstName,
		LastName:       data.LastName,
		Email:          data.Email,
		EventRef:       data.EventRef,
		Title:          "",
		Venue:          "",
		StartTime:      time.Now(),
		OrderReference: orderID,
		Address:        nil,
		Total:          amount,
	}

	mainTicket := map[string]any{
		"email":      data.Email,
		"first_name": data.FirstName,
		"last_name":  data.LastName,
		"ticket_id":  data.TicketID,
		"main":       true,
	}
	var attendees = []map[string]any{}
	attendees = append(attendees, mainTicket)
	var peopleNum = 1
	if data.HasExPeople {
		if data.ExtraPeople != "" {
			var extraPeople []map[string]any
			if err := json.Unmarshal([]byte(data.ExtraPeople), &extraPeople); err == nil {
				attendees = append(attendees, extraPeople...)
				peopleNum += len(extraPeople)
			}
		}
	}
	order.TicketCount = peopleNum
	err := InsertOrder(order)
	if err != nil {
		return "", err
	}

	for i, attendee := range attendees {
		ticket_id := 0
		if attendee["ticket_id"] != nil {
			ticket_id = attendee["ticket_id"].(int)
		}
		referenceP := orderID + "-" + generateID(3)
		ticket := Ticket{
			Reference:   referenceP,
			FirstName:   attendee["first_name"].(string),
			LastName:    attendee["last_name"].(string),
			Email:       attendee["email"].(string),
			EventRef:    data.EventRef,
			orderID:     orderID,
			TicketID:    ticket_id,
			IsCancelled: false,
			HasArrived:  false,
			ArrivalAt:   "",
			CreatedAt:   today,
		}
		emailData.Tickets = append(emailData.Tickets, map[string]any{
			"email":      attendee["email"].(string),
			"first_name": attendee["first_name"].(string),
			"last_name":  attendee["last_name"].(string),
			"reference":  referenceP,
			"ticket_id":  ticket_id,
			"main":       i == 0,
		})
		InsertTicket(ticket)
	}
	// get event
	event, err := getEvent(data.EventRef)
	if err != nil {
		return "", err
	}
	emailData.Title = event.Title
	emailData.Venue = event.Venue
	emailData.StartTime, _ = time.Parse("2006-01-02 15:04:05", event.StartTime)
	emailData.Address = &event.Address
	emailData.Total = amount
	// send email
	err = emailsender.SendOrderEmail(emailData)
	if err != nil {
		return "", err
	}
	return orderID, nil
}
func generateID(length int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
