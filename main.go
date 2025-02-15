package main

import (
	"log"
	"nmmpocket/lib"
	"os"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
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
	lib.InitDB()
	app := pocketbase.New()

	app.Cron().MustAdd("check_invoice", "0 0 * * *", func() { lib.CheckInvoice(app) })

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
