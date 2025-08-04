package lib

import (
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func ScheduleCheck(app *pocketbase.PocketBase) {
	//Runs every 30 minutes
	var now = time.Now().UTC().Add(-1 * time.Minute) //give a minute leeway
	var next30 = now.Add(30 * time.Minute).Format("2006-01-02 15:04:05")

	var filter = "done = false && run_at <= '" + next30 + "' && run_at >= '" + now.Format("2006-01-02 15:04:05") + "'"
	records, err := app.FindRecordsByFilter("scheduled_jobs", filter, "", 0, 0)
	if err != nil {
		fmt.Printf("Failed to fetch scheduled jobs: %v\n", err)
		return
	}
	fmt.Printf("Found %d scheduled jobs to run\n", len(records))
	for _, record := range records {
		var taskErr error
		switch record.GetString("function") {
		case "zoom_email_send":
			taskErr = zoom_email_send(record, app)
		case "other_function":
			// call other function
		default:
			// unknown function
		}
		if taskErr == nil {
			record.Set("done", true)
			record.Set("last_run", time.Now().UTC())
			err = app.Save(record)
			if err != nil {
				fmt.Printf("failed to mark job as done: %v\n", err)
			}
		} else {
			fmt.Printf("failed to run %s: %v\n", record.GetString("function"), taskErr)
		}
	}
}

func zoom_email_send(record *core.Record, app *pocketbase.PocketBase) error {
	//grab the collection and filter from the record
	collection := record.GetString("collection")
	filter := record.GetString("filter")
	emailId := record.GetString("email_template")
	emailRecord, err := app.FindRecordById("email_basic", emailId)
	if err != nil {
		// handle error
		return err
	}
	records, err := app.FindRecordsByFilter(collection, filter, "", 0, 0)
	if err != nil {
		// handle error
		return err
	}
	if emailRecord == nil {
		// handle error
		return fmt.Errorf("email template not found")
	}

	// Fix: Initialize mainParams and properly handle the params
	mainParams := paramsHelper(record)

	var tos []Recipient
	subject := emailRecord.GetString("subject")
	message := emailRecord.GetString("html")
	for _, r := range records {
		errs := app.ExpandRecord(r, []string{"member"}, nil)
		if len(errs) > 0 {
			fmt.Printf("failed to expand record %s: %v\n", r.Id, errs)
			return fmt.Errorf("failed to expand record %s: %v", r.Id, errs)
		}
		to := Recipient{
			Email:     r.ExpandedOne("member").GetString("email"),
			Name:      r.ExpandedOne("member").GetString("first_name") + " " + r.ExpandedOne("member").GetString("last_name"),
			FirstName: r.ExpandedOne("member").GetString("first_name"),
		}

		// Fix: Start with mainParams and then add record-specific params
		paramMap := make(map[string]any)
		// Copy main params first
		maps.Copy(paramMap, mainParams)
		paramMap["join_url"] = r.GetString("join_url")
		to.Params = &paramMap
		tos = append(tos, to)
	}
	err = EmailSender(tos, subject, message, nil, false)
	if err != nil {
		fmt.Printf("failed to send email: %v\n", err)
		return err
	}
	return nil
}

type ScheduledJob struct {
	Collection string          `json:"collection"`
	Filter     string          `json:"filter"`
	Function   string          `json:"function"`
	RunAt      time.Time       `json:"run_at"`
	Done       bool            `json:"done"`
	LastRun    time.Time       `json:"last_run"`
	Email      string          `json:"email_template"`
	Params     *map[string]any `json:"params,omitempty"`
	Record     *core.Record
}

func (s *ScheduledJob) MarkDone(app *pocketbase.PocketBase) error {
	s.Done = true
	s.LastRun = time.Now().UTC()
	return app.Save(s.Record)
}

func (s *ScheduledJob) FromRecord(record *core.Record) {
	s.Collection = record.GetString("collection")
	s.Filter = record.GetString("filter")
	s.Function = record.GetString("function")
	runAt := record.GetDateTime("run_at")
	s.RunAt = runAt.Time()
	s.Done = record.GetBool("done")
	lastRun := record.GetDateTime("last_run")
	s.LastRun = lastRun.Time()
	s.Email = record.GetString("email_template")
	if params := record.Get("params"); params != nil {
		if paramMap, ok := params.(map[string]any); ok {
			s.Params = &paramMap
		}
	}
	s.Record = record
}

func paramsHelper(record *core.Record) map[string]any {
	mainParams := make(map[string]any)
	if paramsRaw := record.Get("params"); paramsRaw != nil {
		switch params := paramsRaw.(type) {
		case map[string]any:
			maps.Copy(mainParams, params)
		case []byte:
			// Handle JSONRaw as bytes
			var paramMap map[string]any
			if err := json.Unmarshal(params, &paramMap); err == nil {
				maps.Copy(mainParams, paramMap)
			} else {
				fmt.Printf("Failed to unmarshal params: %v\n", err)
			}
		default:
			// Try to convert to string and then unmarshal
			if str := fmt.Sprintf("%s", params); str != "" && str != "<nil>" {
				var paramMap map[string]any
				if err := json.Unmarshal([]byte(str), &paramMap); err == nil {
					maps.Copy(mainParams, paramMap)
				} else {
					fmt.Printf("Failed to unmarshal params string: %v\n", err)
				}
			}
			fmt.Printf("Unexpected params type: %T\n", paramsRaw)
		}
	}
	return mainParams
}
