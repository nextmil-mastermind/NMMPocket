package main

import (
	"fmt"
	"log"
	"nmmpocket/database"
	"nmmpocket/emailsender"
	"nmmpocket/event"
	"nmmpocket/lib"
	"nmmpocket/passkeys"
	"os"
	"strings"

	_ "github.com/lib/pq"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/stripe/stripe-go/v81"
)

func main() {
	//check if env of is_prod is set or is set to true
	//if not set to true, load the .env file
	isProd := os.Getenv("is_prod")
	if isProd == "" || isProd == "false" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}
	rpOriginsEnv := os.Getenv("origins")
	rpOrigins := strings.Split(rpOriginsEnv, ",")

	// configure and initialize webauthn
	wconfig := &webauthn.Config{
		RPDisplayName: os.Getenv("appname"), // Display Name for your site
		RPID:          os.Getenv("fqdn"),    // Generally the FQDN for your site
		RPOrigins:     rpOrigins,            // array of origins from where the webapp is served
	}

	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		fmt.Println(err)
		return
	}

	// create a map holding the sessions used during registration and login flow
	webAuthnSessions := make(map[string]*webauthn.SessionData)
	stripe.Key = os.Getenv("STRIPE")
	database.InitDB()
	emailsender.LoadEmailTemplate()
	app := pocketbase.New()
	event.PbApp = app
	app.Cron().MustAdd("check_invoice", "0 11 * * *", func() { lib.CheckInvoice(app) })

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Register Stripe Webhook
		lib.RegisterStripeWebhook(se.Router, app)

		// Register the webauthn routes
		passkeys.RegisterRoutes(se, app, webAuthn, webAuthnSessions)

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
