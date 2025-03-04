package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	ev "nmmpocket/event"

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
	metadata := intent["metadata"].(map[string]any)
	if metadata["type"] == nil {
		return false
	}
	if metadata["type"] == "invoice" {
		err := invoiceResponseProcess(intent, app)
		if err != nil {
			log.Default().Println(err)
			return false
		}
	} else if metadata["type"] == "event" {
		err := ev.EventResponseProcess(intent)
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
	err = app.Save(record)
	if err != nil {
		return fmt.Errorf("failed to save invoice: %v", err)
	}
	collection, err := app.FindCollectionByNameOrId("admin_notifications")
	if err != nil {
		return fmt.Errorf("failed to find collection: %v", err)
	}
	notify := core.NewRecord(collection)
	notify.Set("message", record.GetString("invoicename")+" has been paid by "+record.GetString("name"))
	notify.Set("title", "Invoice")
	notify.Set("color", "green")
	notify.Set("url", "https://dashboard.stripe.com/payments/"+data["id"].(string))
	err = app.Save(notify)
	if err != nil {
		return fmt.Errorf("failed to save notification: %v", err)
	}
	return nil
}
