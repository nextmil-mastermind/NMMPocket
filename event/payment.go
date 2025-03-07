package event

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/customer"
)

// PaymentCancel handles order cancellation.
func PaymentCancel(e *core.RequestEvent) error {
	var data struct {
		Intent string `json:"intent"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&data); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Bad request"})
	}

	// In a real app, you would delete the order from the database.
	resp := map[string]string{"success": "Order cancelled."}
	return e.JSON(http.StatusOK, resp)
}

// PaymentCreate handles order creation.
func PaymentCreate(e *core.RequestEvent) error {
	var data PaymentData
	if err := json.NewDecoder(e.Request.Body).Decode(&data); err != nil {
		return e.JSON(http.StatusBadRequest, OrderResponse{Error: "Bad request"})
	}

	// Verify Captcha
	valid, err := verifyCaptcha(data.GoogleCaptcha)
	if err != nil || !valid {
		return e.JSON(http.StatusBadRequest, OrderResponse{Error: "Invalid Captcha"})
	}

	// Check event exists using stub
	if data.Event == "" || !CheckIfEventExists(data.Event) {
		return e.JSON(http.StatusBadRequest, OrderResponse{Error: "Requires Event!"})
	}

	eventObj, _ := getEvent(data.Event)

	var couponData *Coupon
	var extraData = map[string]string{}
	hasCoupon := false
	if strings.TrimSpace(data.Coupon) != "" {
		cData, exData, ok := hasACoupon(&data, eventObj)
		couponData = cData
		extraData = exData
		hasCoupon = ok
	}

	// Calculate total amount
	total := 0.0
	numOfTickets := len(data.Attendees)
	for _, attendee := range data.Attendees {
		idx := attendee.TicketType
		if idx < 0 || idx >= len(eventObj.TicketTypes) {
			idx = 0
		}
		total += eventObj.TicketTypes[idx]["price"].(float64)
	}

	// Apply coupon if any
	if hasCoupon && couponData != nil {
		if couponData.Type == "free" {
			resp := handleFreeCoupon(data, eventObj, *couponData, numOfTickets)
			return e.JSON(http.StatusOK, resp)
		} else if couponData.Type == "percent" {
			total -= total * (couponData.Amount / 100)
		} else if couponData.Type == "fixed" {
			total -= couponData.Amount
		}
		data = updateTicketType(data, *couponData, numOfTickets)
		if couponData.Type == "different_ticket" {
			newTotal := 0.0
			for _, attendee := range data.Attendees {
				idx := attendee.TicketType
				if idx < 0 || idx >= len(eventObj.TicketTypes) {
					idx = 0
				}
				newTotal += eventObj.TicketTypes[idx]["price"].(float64)
			}
			total = newTotal
			if total == 0 {
				resp := createFreeOrder(data, eventObj, false)
				return e.JSON(http.StatusOK, resp)
			}
		}
	}

	if total == 0 && !hasCoupon {
		resp := createFreeOrder(data, eventObj, false)
		return e.JSON(http.StatusOK, resp)
	}

	customer := retriveOrcreateCustomer(data.Email, data.FirstName+" "+data.LastName)

	if data.Update {
		resp := handleUpdate(data, total, customer, eventObj)
		return e.JSON(http.StatusOK, resp)
	}

	paymentIntent := stripe.PaymentIntent{
		Amount:       int64(total * 100),
		Currency:     "usd",
		Customer:     customer,
		ReceiptEmail: data.Email,
		Description:  "Event Payment",
	}
	paymentIntentParams := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(int64(total * 100)),
		Currency:      stripe.String("usd"),
		Customer:      stripe.String(customer.ID),
		ReceiptEmail:  stripe.String(data.Email),
		Description:   stripe.String("Event Payment"),
		PaymentMethod: stripe.String(data.Intent),
		OffSession:    stripe.Bool(true),
		Confirm:       stripe.Bool(true),
	}
	paymentIntentParams.AddMetadata("type", "event")
	paymentIntentParams.AddMetadata("event", data.Event)
	insertOrder(data, paymentIntent, eventObj)

	resp := OrderResponse{
		ClientSecret: paymentIntent.ClientSecret,
		Due:          total,
	}
	return e.JSON(http.StatusOK, resp)

}

// Verify the captcha token using Google recaptcha.
func verifyCaptcha(token string) (bool, error) {
	captchaSecret := os.Getenv("CAPTCHA_SECRET_KEY")
	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify",
		url.Values{"secret": {captchaSecret}, "response": {token}})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Success, nil
}

// retriveOrcreateCustomer simulates stripe customer retrieval or creation.
func retriveOrcreateCustomer(email, name string) *stripe.Customer {
	// List customers filtered by email.
	params := &stripe.CustomerListParams{}
	params.Filters.AddFilter("email", "", email)
	customerIter := customer.List(params)

	// Check if any customer was found.
	if !customerIter.Next() {
		// Create a new customer.
		params := &stripe.CustomerParams{
			Email: stripe.String(email),
			Name:  stripe.String(name),
		}
		cust, err := customer.New(params)
		if err != nil {
			return nil
		}
		return cust
	}
	return customerIter.Customer()
}

// insertOrder inserts an order into the database.
// Parses the payment intent and stores the order details using InsertOrder.
func insertOrder(data PaymentData, paymentIntent string, eventObj Event) error {
	var order Order
	order.Reference = paymentIntent
	order.FirstName = data.FirstName
	order.LastName = data.LastName
	order.Email = data.Email
	order.EventRef = data.Event
	order.Total = 0
	order.Refunded = false
	order.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	order.TicketCount = len(data.Attendees)
	order.From = data.From
	order.TicketUrl = ""
	// Insert order into database
	err := InsertOrder(order)
	if err != nil {
		// Handle error
		return err
	}
	return nil
}

// handleFreeCoupon processes a coupon of type "free".
func handleFreeCoupon(data PaymentData, eventObj Event, coupon Coupon, numOfTickets int) OrderResponse {
	orderID := generateID(13)
	// Update first attendee ticket type if needed.
	if len(data.Attendees) > 0 {
		data.Attendees[0].TicketType = 1
	}
	data = updateTicketType(data, coupon, numOfTickets)
	err := insertOrder(data, orderID, eventObj)
	if err != nil {
		return OrderResponse{Error: "Failed to insert order"}
	}
	//GoHighLevelStub(data, reference)
	return OrderResponse{Success: "Order fulfilled.", OrderID: orderID, Due: 0}
}

// handleUpdate simulates updating a payment.
func handleUpdate(data PaymentData, total float64, customer *stripe.Customer, eventObj Event) OrderResponse {
	// In a real app, fetch order from DB and update Stripe payment intent.

	paymentIntentParams := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(int64(total * 100)),
		Currency:      stripe.String("usd"),
		Customer:      stripe.String(customer.ID),
		ReceiptEmail:  stripe.String(data.Email),
		Description:   stripe.String("Event Payment"),
		PaymentMethod: stripe.String(data.Intent),
		OffSession:    stripe.Bool(true),
		Confirm:       stripe.Bool(true),
	}
	paymentIntentParams.AddMetadata("type", "event")
	paymentIntentParams.AddMetadata("event", data.Event)
	paymentIntent := stripe.PaymentIntent{
		Amount:       int64(total * 100),
		Currency:     "usd",
		Customer:     customer,
		ReceiptEmail: data.Email,
		Description:  "Event Payment",
	}
	// Update the order in the database
	insertOrder(data, paymentIntent.ID, eventObj)
	return OrderResponse{
		ClientSecret: paymentIntent.ClientSecret,
		Due:          total,
	}
}

// createFreeOrder creates an order with zero amount.
func createFreeOrder(data PaymentData, eventObj Event, ticketOverride bool) OrderResponse {
	orderID := generateID(13)
	if ticketOverride {
		for i := range data.Attendees {
			// Here we assume a default ticket id; adjust as needed.
			data.Attendees[i].TicketType = 1
		}
	}
	insertOrder(data, orderID, eventObj)
	reference, _ := AddOrder(data, 0)
	// Call GoHighLevel stub.
	GoHighLevelStub(data, reference)
	return OrderResponse{Success: "Order fulfilled.", OrderID: orderID, Due: 0}
}

// updateTicketType updates the ticket type field for all attendees if the coupon specifies a ticket.
func updateTicketType(data PaymentData, coupon Coupon, numOfTickets int) PaymentData {
	// If coupon specifies a ticket, update all attendees.
	if coupon.Ticket != 0 {
		for i := 0; i < numOfTickets && i < len(data.Attendees); i++ {
			data.Attendees[i].TicketType = coupon.Ticket
		}
	}
	return data
}

// hasACoupon processes coupon validity.
func hasACoupon(data *PaymentData, eventObj Event) (*Coupon, map[string]string, bool) {
	extraData := map[string]string{}
	code := strings.TrimSpace(strings.ToLower(data.Coupon))
	if eventObj.Coupons != nil {
		for _, coup := range eventObj.Coupons {
			if strings.ToLower(coup["Code"].(string)) == code {
				// Check expiration if set.
				if coup["expiration"] != "" {
					expTime, err := time.Parse(time.RFC3339, coup["expiration"].(string))
					if err == nil && time.Now().After(expTime) {
						extraData["message"] = "Coupon Expired!"
						return nil, extraData, false
					}
				}
				coupData := Coupon{
					Type:       coup["type"].(string),
					Amount:     coup["amount"].(float64),
					Ticket:     coup["ticket"].(int),
					Expiration: coup["expiration"].(string),
				}
				return &coupData, extraData, true
			}
		}
	}
	extraData["message"] = "Invalid Coupon!"
	return nil, extraData, false
}
