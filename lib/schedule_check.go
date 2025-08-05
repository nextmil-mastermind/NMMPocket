package lib

import (
	"encoding/json"
	"fmt"
	"maps"
	"nmmpocket/zoomcon"
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
		case "zoom_admin_start_meeting":
			taskErr = zoom_admin_start_meeting(record, app)
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
	errs := app.ExpandRecord(record, []string{"email_template"}, nil)
	if len(errs) > 0 {
		return fmt.Errorf("failed to expand email_template: %v", errs)
	}
	emailRecord := record.ExpandedOne("email_template")
	records, err := app.FindRecordsByFilter(collection, filter, "", 0, 0)
	if err != nil {
		// handle error
		return err
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
	err = EmailSender(tos, subject, message, nil)
	if err != nil {
		fmt.Printf("failed to send email: %v\n", err)
		return err
	}
	return nil
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

func zoom_admin_start_meeting(record *core.Record, app *pocketbase.PocketBase) error {
	//We don't need to grab the collection or filter, just the record itself
	errs := app.ExpandRecord(record, []string{"email_template"}, nil)
	if len(errs) > 0 {
		return fmt.Errorf("failed to expand email_template: %v", errs)
	}
	emailRecord := record.ExpandedOne("email_template")
	type MeetingStartParams struct {
		MeetingID    int64            `json:"meeting_id"`
		OccurrenceID int64            `json:"occurrence_id"`
		Emails       []map[string]any `json:"emails"`
		CC           []map[string]any `json:"cc,omitempty"`
	}
	var params MeetingStartParams
	if p := record.Get("params"); p != nil {
		err := record.UnmarshalJSONField("params", &params)
		if err != nil {
			return fmt.Errorf("failed to unmarshal params: %v", err)
		}
	} else {
		return fmt.Errorf("no params found for zoom_admin_start_meeting job")
	}
	var zt zoomcon.ZOOM_TOKEN
	_, err := zt.GetAccessToken()
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to get access token: %v\n", err)
		return err
	}
	meeting, err := zt.GrabSingleOccurrence(params.MeetingID, params.OccurrenceID)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to grab single occurrence: %v\n", err)
		return err
	}
	var tos []Recipient
	for _, em := range params.Emails {
		to := Recipient{
			Email:     fmt.Sprintf("%v", em["email"]),
			Name:      fmt.Sprintf("%v", em["first_name"]) + " " + fmt.Sprintf("%v", em["last_name"]),
			FirstName: fmt.Sprintf("%v", em["first_name"]),
		}
		paramMap := make(map[string]any)
		paramMap["start_url"] = meeting.StartURL
		paramMap["topic"] = meeting.Topic
		paramMap["start_time"] = meeting.StartTime
		fmt.Printf("Meeting start time raw: %s\n", meeting.StartTime)
		//StartTime is in RFC3339 format, we can parse it and then convert it to EST MMM/DD/YYYY HH:MM AM/PM
		if t, err := time.Parse(time.RFC3339, meeting.StartTime); err == nil {
			loc, _ := time.LoadLocation("America/New_York")
			paramMap["start_time_est"] = t.In(loc).Format("01/02/2006 03:04 PM") + " EST"
		} else {
			paramMap["start_time_est"] = meeting.StartTime // fallback to original
		}
		paramMap["link_expires_at"] = time.Now().Add(120 * time.Minute).Format("01/02/2006 03:04 PM")
		paramMap["duration"] = meeting.Duration
		to.Params = &paramMap
		if len(params.CC) > 0 {
			to.CC = params.CC
		}
		tos = append(tos, to)
	}
	subject := emailRecord.GetString("subject")
	message := emailRecord.GetString("html")

	err = EmailSender(tos, subject, message, nil)
	if err != nil {
		fmt.Printf("failed to send meeting start email: %v\n", err)
		return err
	}
	return nil // placeholder for actual meeting start logic
}
