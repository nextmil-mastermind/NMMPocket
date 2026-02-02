package appform

import (
	"net/http"
	"nmmpocket/ghl"
	"os"
	"time"

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
	if success {
		if err := SentToGHL(submission.ToApplication(), false); err != nil {
			//log the error
			app.Logger().Error("Error sending to GHL", "error", err)
		}
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
	if success {
		if err := SentToGHL(submission.ToApplication(), true); err != nil {
			//log the error
			app.Logger().Error("Error sending to GHL", "error", err)
		}
	}
	return e.JSON(http.StatusOK, "Application submitted successfully")
}

func SentToGHL(submission Application, small bool) error {
	ghlstart := ghl.GoHighLevelRequest{
		AccessToken: os.Getenv("GHL_Token"),
		LocationId:  os.Getenv("GHL_Location"),
	}
	contactData := map[string]any{
		"firstName": submission.FirstName,
		"lastName":  submission.LastName,
		"email":     submission.EmailAddress,
		"phone":     submission.Phone,
	}
	if !small {
		contactData["companyName"] = submission.Company
		contactData["address1"] = submission.Address
		contactData["city"] = submission.City
		contactData["state"] = submission.State
		contactData["postalCode"] = submission.Zip
		contactData["website"] = submission.Website
	}
	contact, err := ghlstart.UpsertContact(contactData)
	if err != nil {
		return err
	}
	//lets write a sentence that says if the system determined this was a human submission
	isHumanMessage := ""
	if submission.Human != nil {
		if *submission.Human {
			isHumanMessage = "The system determined this was a human submission."
		} else {
			isHumanMessage = "The system determined this was NOT a human submission."
		}
	} else {
		isHumanMessage = "Human verification status unknown."
	}
	now := time.Now()
	dueDate := now.Add(48 * time.Hour).Format(time.RFC3339)
	task := ghl.Task{
		Title: "Follow up with application",
		Body: "Follow up with " + submission.FirstName + " " + submission.LastName +
			" regarding their application.\n" + isHumanMessage,
		DueDate:    dueDate,
		Completed:  false,
		AssignedTo: "ikUTBqoNOkbLPMyCaLcN",
	}
	if submission.ReferredBy != nil && *submission.ReferredBy != "" {
		task.Body += "\nReferred By: " + *submission.ReferredBy
	}
	if small {
		task.Body = "Follow up with " + submission.FirstName + " " + submission.LastName +
			" regarding their quick application as that one doesn't contain any additional details.\n" + isHumanMessage
	} else {
		task.Body = "Follow up with " + submission.FirstName + " " + submission.LastName +
			" regarding their full application.\n" + isHumanMessage
	}
	err = ghlstart.AddTask(contact["id"].(string), task)
	if err != nil {
		return err
	}

	return nil
}

/*func SendToEmailScheduler(submission Application) error {
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
*/
