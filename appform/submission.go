package appform

import (
	"net/http"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func ReceivedSubmissionRoute(app *pocketbase.PocketBase, e *core.RequestEvent) error {
	var submission ApplicationSubmission
	err := e.BindBody(&submission)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}
	success, err := submission.VerifyTurnstile(os.Getenv("turnstile_secret"))
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}
	collection, err := app.FindCollectionByNameOrId("applications")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("first_name", submission.FirstName)
	record.Set("last_name", submission.LastName)
	record.Set("email_address", submission.EmailAddress)
	record.Set("phone", submission.Phone)
	record.Set("company", submission.Company)
	record.Set("website", submission.Website)
	record.Set("address", submission.Address)
	record.Set("city", submission.City)
	record.Set("state", submission.State)
	record.Set("zip", submission.Zip)
	record.Set("message", submission.Message)
	record.Set("terms", submission.Terms)
	record.Set("human", success)
	err = app.Save(record)
	if err != nil {
		return err
	}
	return e.JSON(http.StatusOK, "Application submitted successfully")
}
