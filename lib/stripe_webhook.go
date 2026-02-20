package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentmethod"
)

// Register a stripe webhook
func RegisterStripeWebhook(sr *router.Router[*core.RequestEvent], app *pocketbase.PocketBase) {
	sr.GET("/stripe/invoice/{invoiceID}", generate_link_invoice)
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
			if processIntentSucceded(event, app) {
				return e.JSON(http.StatusOK, map[string]string{"status": "success"})
			} else {
				return e.JSON(http.StatusOK, map[string]string{"status": "failed"})
			}
		case "checkout.session.completed":
			// Handle successful checkout
			if processIntentSucceded(event, app) {
				return e.JSON(http.StatusOK, map[string]string{"status": "success"})
			} else {
				return e.JSON(http.StatusOK, map[string]string{"status": "failed"})
			}
		default:
			return e.JSON(http.StatusOK, map[string]string{"status": "success"})
		}
	})
}

func processIntentSucceded(event *stripe.Event, app *pocketbase.PocketBase) bool {
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
		err := invoiceResponseProcess(intent, app)
		if err != nil {
			log.Default().Println(err)
			return false
		}
	}
	return true
}

func invoiceResponseProcess(data map[string]any, app *pocketbase.PocketBase) error {
	record, err := app.FindFirstRecordByData("invoices", "session", data["id"].(string))
	if err != nil {
		return fmt.Errorf("failed to find invoice: %v", err)
	}
	record.Set("paid", true)
	//set paid date to now in 2022-01-01 10:00:00.123Z format
	record.Set("paid_date", time.Now().Format(time.RFC3339))
	err = app.Save(record)
	if err != nil {
		return fmt.Errorf("failed to save invoice: %v", err)
	}

	email := record.GetString("email")
	name := record.GetString("name")

	// Expand members relation if present
	app.ExpandRecord(record, []string{"members"}, nil)
	allMembers := record.ExpandedAll("members")
	if len(allMembers) > 0 {
		firstMember := allMembers[0]
		memberName := firstMember.GetString("first_name") + " " + firstMember.GetString("last_name")
		memberEmail := firstMember.GetString("email")
		if memberEmail != "" {
			email = memberEmail
			name = memberName
		}
	}
	collection, err := app.FindCollectionByNameOrId("admin_notifications")
	if err != nil {
		return fmt.Errorf("failed to find collection: %v", err)
	}
	if record.GetString("type") == "save" {
		params := &stripe.PaymentMethodParams{}
		result, err := paymentmethod.Get(data["payment_method"].(string), params)
		last4 := result.Card.Last4
		if err != nil {
			last4 = ""
		}

		err = save_card(email, data["payment_method"].(string), last4)
		if err != nil {
			return fmt.Errorf("failed to save card: %v", err)
		}
	}
	notify := core.NewRecord(collection)
	notify.Set("message", record.GetString("invoicename")+" has been paid by "+name+" ("+email+")")
	notify.Set("title", "Invoice")
	notify.Set("color", "green")
	notify.Set("url", "https://dashboard.stripe.com/payments/"+data["id"].(string))
	err = app.Save(notify)
	if err != nil {
		return fmt.Errorf("failed to save notification: %v", err)
	}
	return nil
}
