package lib

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/stripe/stripe-go/v81"
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
