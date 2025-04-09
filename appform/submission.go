package appform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt"
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
	record.Set("type", "full")
	err = app.Save(record)
	if err != nil {
		return err
	}
	if err := SendToEmailScheduler(submission.ToApplication()); err != nil {
		//log the error
		app.Logger().Error("Error sending to email scheduler", "error", err)
	}
	return e.JSON(http.StatusOK, "Application submitted successfully")
}

func ReceivedSmallSubmissionRoute(app *pocketbase.PocketBase, e *core.RequestEvent) error {
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
	record.Set("type", "small")
	record.Set("human", success)
	err = app.Save(record)
	if err != nil {
		return err
	}
	return e.JSON(http.StatusOK, "Application submitted successfully")
}

func SendToEmailScheduler(submission Application) error {
	//Create a JWT token with the submission id and the email address
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"submission_id": submission.ID,
		"email":         submission.EmailAddress,
	})
	tokenString, err := token.SignedString([]byte(os.Getenv("jwt_secret")))
	if err != nil {
		return err
	}
	jsonData, err := json.Marshal(submission)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "https://direct.nextmilmastermind.com/flows/trigger/7430360f-92e7-47ee-a6c8-9d6bc9ef719d", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("TokenAuth", tokenString)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
