package event

import (
	"encoding/json"
	"math/rand"
	"time"
)

func AddOrder(data map[string]any, amount float64) string {
	var emailData map[string]any
	var orderID = generateID(14)
	var today = time.Now().Format("2006-01-02 15:04:05")
	order := Order{
		Reference:   orderID,
		FirstName:   data["first_name"].(string),
		LastName:    data["last_name"].(string),
		Email:       data["email"].(string),
		EventRef:    data["event_ref"].(string),
		Total:       amount,
		Refunded:    false,
		CreatedAt:   today,
		TicketCount: 0,
		From:        data["from"].(string),
		TicketUrl:   "",
	}
	emailData = map[string]any{
		"email":           data["email"].(string),
		"first_name":      data["first_name"].(string),
		"last_name":       data["last_name"].(string),
		"event_ref":       data["event_ref"].(string),
		"order_reference": orderID,
		"total":           amount,
		"tickets":         []map[string]any{},
	}
	mainTicket := map[string]any{
		"email":      data["email"].(string),
		"first_name": data["first_name"].(string),
		"last_name":  data["last_name"].(string),
		"ticket_id":  data["ticket_id"].(int),
		"main":       true,
	}
	var attendees = []map[string]any{}
	attendees = append(attendees, mainTicket)
	var peopleNum = 1
	if hasExPeople, ok := data["HasExPeople"].(bool); ok && hasExPeople {
		if extraPeopleStr, ok := data["ExtraPeople"].(string); ok {
			var extraPeople []map[string]any
			if err := json.Unmarshal([]byte(extraPeopleStr), &extraPeople); err == nil {
				attendees = append(attendees, extraPeople...)
				peopleNum += len(extraPeople)
			}
		}
	}
	order.TicketCount = peopleNum
	InsertOrder(order)

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
			EventRef:    data["event_ref"].(string),
			orderID:     orderID,
			TicketID:    ticket_id,
			IsCancelled: false,
			HasArrived:  false,
			ArrivalAt:   "",
			CreatedAt:   today,
		}
		emailData["tickets"] = append(emailData["tickets"].([]map[string]any), map[string]any{
			"email":      attendee["email"].(string),
			"first_name": attendee["first_name"].(string),
			"last_name":  attendee["last_name"].(string),
			"reference":  referenceP,
			"ticket_id":  ticket_id,
			"main":       i == 0,
		})
		InsertTicket(ticket)
	}
	return orderID
}
func generateID(length int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
