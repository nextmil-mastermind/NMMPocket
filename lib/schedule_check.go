package lib

import (
	"encoding/json"
	"fmt"
	"maps"
	"nmmpocket/openphone"
	"nmmpocket/zoomcon"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
	"github.com/pocketbase/pocketbase/tools/types"
	"golang.org/x/net/html"
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
		case "zoom_admin_start_webinar":
			taskErr = zoom_admin_start_webinar(record, app)
		case "zoom_sms_send":
			taskErr = zoom_sms_send(record, app)
		// Add more functions as needed
		default:
			// unknown function
			fmt.Printf("Unknown scheduled job function: %s\n", record.GetString("function"))
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
		case types.JSONRaw:
			// Handle types.JSONRaw from PocketBase
			var paramMap map[string]any
			if err := json.Unmarshal([]byte(params), &paramMap); err == nil {
				maps.Copy(mainParams, paramMap)
			} else {
				fmt.Printf("Failed to unmarshal JSONRaw params: %v\n", err)
			}
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

	type MeetingStartParams struct {
		MeetingID    int64 `json:"meeting_id"`
		OccurrenceID int64 `json:"occurrence_id"`
		Recipients
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

	return zoomStartLinkHelper(params.Recipients, meeting, record, app) // placeholder for actual meeting start logic
}

func zoomStartLinkHelper(params Recipients, meeting zoomcon.Meeting, record *core.Record, app *pocketbase.PocketBase) error {
	//We don't need to grab the collection or filter, just the record itself
	errs := app.ExpandRecord(record, []string{"email_template"}, nil)
	if len(errs) > 0 {
		return fmt.Errorf("failed to expand email_template: %v", errs)
	}
	emailRecord := record.ExpandedOne("email_template")
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

		// Convert meeting start time to Eastern Time
		if t, err := time.Parse(time.RFC3339, meeting.StartTime); err == nil {
			paramMap["start_time_est"] = formatEasternTime(t)
		} else {
			fmt.Printf("Failed to parse meeting start time: %v\n", err)
			paramMap["start_time_est"] = meeting.StartTime // fallback to original
		}

		// Convert link expiration time to Eastern Time
		expiresAtUTC := time.Now().Add(120 * time.Minute)
		paramMap["link_expires_at"] = formatEasternTime(expiresAtUTC)
		paramMap["duration"] = meeting.Duration
		to.Params = &paramMap
		if len(params.CC) > 0 {
			to.CC = params.CC
		}
		tos = append(tos, to)
	}
	subject := emailRecord.GetString("subject")
	message := emailRecord.GetString("html")

	err := EmailSender(tos, subject, message, nil)
	if err != nil {
		fmt.Printf("failed to send meeting start email: %v\n", err)
		return err
	}
	return nil
}
func zoom_admin_start_webinar(record *core.Record, app *pocketbase.PocketBase) error {
	// Placeholder for webinar start logic
	type MeetingStartParams struct {
		WebinarId  int64      `json:"webinar_id"`
		Recipients Recipients `json:"recipients"`
	}
	var zt zoomcon.ZOOM_TOKEN
	_, err := zt.GetAccessToken()
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to get access token: %v\n", err)
		return err
	}
	var params MeetingStartParams
	if p := record.Get("params"); p != nil {
		err := record.UnmarshalJSONField("params", &params)
		if err != nil {
			return fmt.Errorf("failed to unmarshal params: %v", err)
		}
	} else {
		return fmt.Errorf("no params found for zoom_admin_start_webinar job")
	}
	webinar, err := zt.GrabWebinar(params.WebinarId)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to grab webinar: %v\n", err)
		return err
	}

	return zoomStartLinkHelper(params.Recipients, webinar, record, app)
}

func zoom_sms_send(record *core.Record, app *pocketbase.PocketBase) error {
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

	// Initialize mainParams and properly handle the params
	mainParams := paramsHelper(record)

	// Validate required from_number parameter
	fromNumber, ok := mainParams["from_number"].(string)
	if !ok || fromNumber == "" {
		return fmt.Errorf("from_number parameter is required and must be a string")
	}

	// Pre-compile the template once
	temp := template.NewRegistry().LoadString(HTMLToText(emailRecord.GetString("html")))

	// Track messages for completion
	var messageCount int

	for i, rec := range records {
		errs := app.ExpandRecord(rec, []string{"member"}, nil)
		if len(errs) > 0 {
			fmt.Printf("failed to expand record %s: %v\n", rec.Id, errs)
			return fmt.Errorf("failed to expand record %s: %v", rec.Id, errs)
		}
		member := rec.ExpandedOne("member")
		phoneNumber := fmt.Sprintf("%v", member.Get("phone"))
		if phoneNumber == "" || phoneNumber == "<nil>" {
			fmt.Printf("Skipping record %d: empty phone number\n", i+1)
			continue
		}

		paramMap := make(map[string]any)
		// Copy main params first
		maps.Copy(paramMap, mainParams)
		paramMap["join_url"] = rec.GetString("join_url")
		paramMap["first_name"] = member.GetString("first_name")
		paramMap["last_name"] = member.GetString("last_name")
		paramMap["email"] = member.GetString("email")

		// Render the template with the record data
		text, err := temp.Render(paramMap)
		if err != nil {
			fmt.Printf("Failed to execute template for record %d: %v\n", i+1, err)
			continue
		}

		// Create and enqueue the message job
		messageJob := openphone.MessageJob{
			PhoneNumber: phoneNumber,
			FromNumber:  fromNumber,
			Content:     text,
		}

		messageCount++

		// Simply enqueue the job - the worker will handle retries and errors
		openphone.Enqueue(messageJob)
		fmt.Printf("Enqueued SMS %d to %s\n", i+1, phoneNumber)
	}

	fmt.Printf("Successfully enqueued %d SMS messages\n", messageCount)
	return nil
}

func HTMLToText(s string) string {
	// Parse the HTML
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return strings.TrimSpace(html.UnescapeString(s))
	}

	var b strings.Builder
	var walk func(*html.Node)
	newlineBlocks := map[string]bool{
		"p": true, "div": true, "section": true, "article": true, "header": true, "footer": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"ul": true, "ol": true, "li": true, "br": true, "table": true, "tr": true,
	}

	linkStack := []string{} // collect links to append as footnotes if you want

	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			name := strings.ToLower(n.Data)

			// Line breaks before certain blocks (except first char)
			if name == "br" {
				b.WriteString("\n")
			}

			if name == "a" {
				// capture href for optional footnotes
				for _, a := range n.Attr {
					if strings.EqualFold(a.Key, "href") && a.Val != "" {
						linkStack = append(linkStack, a.Val)
						break
					}
				}
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}

			// Add bullet for list items
			if name == "li" {
				b.WriteString("\n")
			}

			// Newline after block-ish elements
			if newlineBlocks[name] {
				b.WriteString("\n")
			}
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(doc)

	out := html.UnescapeString(b.String())

	// Normalize whitespace/newlines
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = strings.ReplaceAll(out, "\r", "\n")
	out = collapseBlankLines(strings.TrimSpace(out))

	// Optional: append links as footnotes
	//_ = linkStack // if you want:
	for i, link := range linkStack {
		out += fmt.Sprintf("\n[%d] %s", i+1, link)
	}

	return out
}

// Collapse 3+ blank lines to max 2, and shrink runs of spaces.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	blankRun := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			blankRun++
			if blankRun > 2 {
				continue
			}
			out = append(out, "")
		} else {
			blankRun = 0
			// shrink internal multiple spaces
			out = append(out, strings.Join(strings.Fields(ln), " "))
		}
	}
	return strings.Join(out, "\n")
}
