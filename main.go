package main

import (
	"log"
	"nmmpocket/lib"
	"os"

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
	stripe.Key = os.Getenv("STRIPE")
	lib.InitDB()
	app := pocketbase.New()

	app.Cron().MustAdd("check_invoice", "0 0 * * *", func() { lib.CheckInvoice(app) })

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		lib.RegisterStripeWebhook(se.Router, app)
		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
