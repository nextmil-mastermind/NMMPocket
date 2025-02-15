package lib

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/stripe/stripe-go/v81"
)

// Register a stripe webhook
func RegisterStripeWebhook(sr *router.Router[*core.RequestEvent], app *pocketbase.PocketBase) {

	sr.POST("/stripe/webhook", func(e *core.RequestEvent) error {
		event := &stripe.Event{}

		// Parse the incoming event from the request body
		err := json.NewDecoder(e.Request.Body).Decode(event)
		if err != nil {
			return e.BadRequestError("Failed to parse webhook event: %v", err)
		}
		switch event.Type {
		case "payment_intent.succeeded":
			// Handle successful payment
			if processIntentSucceded(event, app.DB()) {
				return e.JSON(http.StatusOK, map[string]string{"status": "success"})
			} else {
				return e.JSON(http.StatusOK, map[string]string{"status": "failed"})
			}
		case "checkout.session.completed":
			// Handle successful checkout
			if processIntentSucceded(event, app.DB()) {
				return e.JSON(http.StatusOK, map[string]string{"status": "success"})
			} else {
				return e.JSON(http.StatusOK, map[string]string{"status": "failed"})
			}
		default:
			return e.JSON(http.StatusOK, map[string]string{"status": "success"})
		}
	})
}

func processIntentSucceded(event *stripe.Event, db dbx.Builder) bool {
	intent := event.Data.Object
	//check is metadata is present and contains a type field
	if intent["metadata"] == nil {
		return false
	}
	metadata := intent["metadata"].(map[string]interface{})
	if metadata["type"] == nil {
		return false
	}
	if metadata["type"] == "invoice" {
		invoiceResponseProcess(intent, db)
	}
	return true
}

func invoiceResponseProcess(data map[string]interface{}, db dbx.Builder) error {
	var invoice Invoice
	db.Select("*").From("invoices").Where(dbx.NewExp("session = {:session}", dbx.Params{"session": data["id"]})).One(&invoice)
	if invoice.ID == "" {
		return fmt.Errorf("invoice not found")
	}
	_, err := db.Update("invoices", dbx.Params{"paid": true}, dbx.NewExp("id = {:id}", dbx.Params{"id": invoice.ID})).Execute()
	if err != nil {
		return fmt.Errorf("failed to update invoice: %v", err)
	}
	params := dbx.Params{"message": invoice.InvoiceName + " has been paid by " + invoice.Name, "title": "Invoice", "color": "success", "url": "https://dashboard.stripe.com/payments/" + data["id"].(string)}
	db.Insert("admin_notifications", params).Execute()
	return nil
}
