package lib

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/paymentintent"
)

type SavedCard struct {
	ID        int    `db:"id"`
	Email     string `db:"email"`
	PaymentID string `db:"payment_id"`
	Last_4    string `db:"last_4"`
}

func createStripeCharge(invoice Invoice, app *pocketbase.PocketBase) (bool, error) {
	email := invoice.Email
	//check stored cards to see if the email is in there
	card, err := grab_card(email)
	if err != nil {
		return false, err
	}
	// List customers filtered by email.
	params := &stripe.CustomerListParams{}
	params.Filters.AddFilter("email", "", email)
	customerIter := customer.List(params)

	// Check if any customer was found.
	if !customerIter.Next() {
		return false, fmt.Errorf("no customer found with email %q", email)
	}
	cust := customerIter.Customer()

	amountCents := int64(invoice.Amount * 100)

	// Create PaymentIntent parameters.
	piParams := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(amountCents),
		Currency:      stripe.String("usd"),
		Customer:      stripe.String(cust.ID),
		PaymentMethod: stripe.String(card.PaymentID),
		OffSession:    stripe.Bool(true),
		Confirm:       stripe.Bool(true),
		ReceiptEmail:  stripe.String(email),
		Description:   stripe.String(invoice.InvoiceName),
	}
	piParams.AddMetadata("type", "invoice")

	// Create the PaymentIntent.
	pi, err := paymentintent.New(piParams)
	if err != nil {
		return false, err
	}
	record, err := app.FindRecordById("invoices", invoice.ID)
	if err != nil {
		return false, err
	}
	record.Set("session", pi.ID)
	err = app.Save(record)
	if err != nil {
		return false, err
	}
	return true, nil

}

type AutoPayForce struct {
	InvoiceID string `json:"invoice_id"`
}

func InvoiceAutopayForceRoute(e *core.RequestEvent) error {
	if e.Auth.Collection().Name != "users" {
		return e.JSON(403, map[string]string{"error": "Unauthorized"})
	}
	var data AutoPayForce
	if err := e.BindBody(&data); err != nil {
		return e.JSON(400, map[string]string{"error": "Invalid request body"})
	}
	record, err := e.App.FindRecordById("invoices", data.InvoiceID)
	if err != nil {
		return e.JSON(404, map[string]string{"error": "Invoice not found"})
	}

	invoice := Invoice{}
	// Using custom decoder configuration with WeaklyTypedInput option
	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &invoice,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return e.JSON(500, map[string]string{"error": "Failed to create decoder"})
	}

	if err := decoder.Decode(record.PublicExport()); err != nil {
		// Log the error and record data for debugging
		fmt.Printf("Invoice decode error: %v, data: %v\n", err, record.PublicExport())
		return e.JSON(500, map[string]string{"error": "Failed to decode invoice"})
	}

	charged, err := createStripeCharge(invoice, e.App.(*pocketbase.PocketBase))
	if err != nil {
		return e.JSON(500, map[string]string{"error": "Failed to create charge"})
	}
	if !charged {
		return e.JSON(500, map[string]string{"error": "Failed to charge invoice"})
	}

	return e.JSON(200, map[string]string{"status": "success"})
}

func grab_card(email string) (SavedCard, error) {
	var card []SavedCard
	// get card from db
	res, err := pgDB.Query("SELECT * FROM stored_cards WHERE email = $1", email)
	if err != nil {
		return SavedCard{}, err
	}
	defer res.Close()
	card, err = rowsToCard(res)
	if err != nil {
		return SavedCard{}, err
	}
	if len(card) == 0 {
		return SavedCard{}, fmt.Errorf("no card found for email %q", email)
	}
	return card[0], nil

}

func save_card(email string, paymentID string, last4 string) error {
	// Use PostgreSQL's upsert feature with email as the unique key
	_, err := pgDB.Exec(
		"INSERT INTO stored_cards (email, payment_id, last_4) VALUES ($1, $2, $3) "+
			"ON CONFLICT (email) DO UPDATE SET payment_id = $2, last_4 = $3",
		email, paymentID, last4)

	if err != nil {
		return fmt.Errorf("failed to save card: %w", err)
	}
	return nil
}

func generate_link_invoice(e *core.RequestEvent) error {
	if e.Auth.Collection().Name != "members" {
		return e.JSON(403, map[string]string{"error": "Unauthorized"})
	}
	//invoice id from path
	invoiceID := e.Request.PathValue("invoiceID")
	record, err := e.App.FindRecordById("invoices", invoiceID)
	if err != nil {
		return e.HTML(404, "<h1>Invoice not found</h1>")
	}
	if record.GetBool("paid") {
		return e.HTML(200, "<h1>Invoice has already been paid</h1>")
	}
	//check if invoice has been updated in the last 12 hours
	//redirect user to session_url(if its not empty) else create a new session url
	updatedAt := record.GetDateTime("updated")
	if time.Since(updatedAt.Time()) < 12*time.Hour {
		sessionURL := record.GetString("session_url")
		if sessionURL != "" {
			return e.Redirect(302, sessionURL)
		}
	}
	paySession, err := generateSession(record)
	if err != nil {
		log.Printf("Error generating session: %v", err)
		return e.JSON(500, map[string]string{"error": "Failed to generate payment session"})
	}
	record.Set("session_url", paySession.URL)
	record.Set("session", paySession.ID)
	err = e.App.Save(record)
	if err != nil {
		log.Printf("Error saving invoice record: %v", err)
		return e.JSON(500, map[string]string{"error": "Failed to save invoice session"})
	}
	return e.Redirect(302, paySession.URL)
}

func generateSession(invoice *core.Record) (*stripe.CheckoutSession, error) {
	// Check if customer exists based on the invoice email.
	email := invoice.GetString("email")
	custParams := &stripe.CustomerListParams{
		Email: stripe.String(email),
	}
	custParams.Limit = stripe.Int64(1)
	ci := customer.List(custParams)
	var cust *stripe.Customer
	if ci.Next() {
		cust = ci.Customer()
	} else {
		// Create a new customer.
		name := invoice.GetString("name")
		createParams := &stripe.CustomerParams{
			Email: stripe.String(email),
			Name:  stripe.String(name),
		}
		var err error
		cust, err = customer.New(createParams)
		if err != nil {
			log.Printf("Error creating customer: %v", err)
			return nil, err
		}
	}

	// Convert invoice amount to the proper unit.
	// Assumes invoice["amount"] is either string or a float64.
	var unitAmount int64
	switch v := invoice.Get("amount").(type) {
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			unitAmount = int64(f * 100)
		}
	case float64:
		unitAmount = int64(v * 100)
	default:
		unitAmount = 0
	}

	// Build the checkout session parameters.
	// Retrieve additional invoice details.
	invoiceName := invoice.GetString("invoicename")
	description := invoice.GetString("description")
	invoiceType := invoice.GetString("type")
	sessionParams := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("usd"),
					UnitAmount: stripe.Int64(unitAmount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(invoiceName),
						Description: stripe.String(description),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		BillingAddressCollection: stripe.String("required"),
		Customer:                 stripe.String(cust.ID),
		Mode:                     stripe.String("payment"),
		Metadata: map[string]string{
			"type": "invoice",
		},
	}

	// For "standard" invoices, enable saving the payment method. Otherwise, set up for off-session payments.
	if invoiceType == "" || invoiceType == "standard" {
		sessionParams.SavedPaymentMethodOptions = &stripe.CheckoutSessionSavedPaymentMethodOptionsParams{
			PaymentMethodSave: stripe.String("enabled"),
		}
	} else {
		sessionParams.PaymentIntentData = &stripe.CheckoutSessionPaymentIntentDataParams{
			SetupFutureUsage: stripe.String("off_session"),
		}
	}

	sessionParams.SuccessURL = stripe.String("https://nextmilmastermind.com/thank-you")
	sessionParams.CancelURL = stripe.String("https://nextmilmastermind.com")

	paySession, err := session.New(sessionParams)
	if err != nil {
		log.Printf("Error creating checkout session: %v", err)
		return nil, err
	}

	return paySession, nil
	/*pb.UpdateInvoice(invoiceID, updateData)

	response := map[string]any{"status": "success"}
	if sendDataback {
		response["invoice"] = invoice
	}
	if isEmbedded {
		response["clientsecret"] = paySession.ClientSecret
	} else {
		response["url"] = paySession.URL
	}
	return response*/
}
