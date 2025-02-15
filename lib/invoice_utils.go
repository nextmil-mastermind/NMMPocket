package lib

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
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
