package rsvp

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"nmmpocket/lib"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type SendResult struct {
	Invited int      `json:"invited"`
	Emailed int      `json:"emailed"`
	Errors  []string `json:"errors,omitempty"`
}

func SendEmails(app core.App, event *core.Record) (*SendResult, error) {
	errs := app.ExpandRecord(event, []string{"email_template"}, nil)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to expand email_template: %v", errs)
	}
	emailRecord := event.ExpandedOne("email_template")
	if emailRecord == nil {
		return nil, fmt.Errorf("rsvp %q has no email_template", event.GetString("slug"))
	}

	members, err := ResolveMembers(app, event)
	if err != nil {
		return nil, err
	}

	result := &SendResult{Invited: len(members)}
	if len(members) == 0 {
		return result, nil
	}

	responses, err := app.FindCollectionByNameOrId("rsvp_responses")
	if err != nil {
		return nil, err
	}

	base := strings.TrimRight(os.Getenv("appurl"), "/")
	if base == "" {
		base = "https://pocket.nextmil.org"
	}
	slug := event.GetString("slug")
	title := rsvpTitle(event)
	subject := emailRecord.GetString("subject")
	message := emailRecord.GetString("html")

	var tos []lib.Recipient
	for _, member := range members {
		if err := upsertPendingResponse(app, responses, event.Id, member.Id); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", member.GetString("email"), err))
			continue
		}
		emailAddr := member.GetString("email")
		if emailAddr == "" {
			result.Errors = append(result.Errors, member.Id+": missing email")
			continue
		}
		token, err := member.NewAuthToken()
		if err != nil {
			result.Errors = append(result.Errors, emailAddr+": "+err.Error())
			continue
		}
		rsvpURL := base + "/rsvp/" + url.PathEscape(slug) + "?token=" + url.QueryEscape(token)
		firstName := member.GetString("first_name")
		params := map[string]any{
			"first_name": firstName,
			"last_name":  member.GetString("last_name"),
			"rsvp_url":   rsvpURL,
			"title":      title,
			"slug":       slug,
		}
		tos = append(tos, lib.Recipient{
			Email:     emailAddr,
			Name:      strings.TrimSpace(firstName + " " + member.GetString("last_name")),
			FirstName: firstName,
			Params:    &params,
		})
	}

	if len(tos) == 0 {
		return result, fmt.Errorf("no emails sent")
	}
	if err := lib.EmailSender(tos, subject, message, nil); err != nil {
		return result, err
	}

	result.Emailed = len(tos)
	now, err := types.ParseDateTime(time.Now().UTC())
	if err != nil {
		return result, err
	}
	event.Set("sent_at", now)
	if err := app.Save(event); err != nil {
		return result, err
	}
	return result, nil
}

func upsertPendingResponse(app core.App, collection *core.Collection, rsvpID, memberID string) error {
	existing, err := app.FindFirstRecordByFilter(
		"rsvp_responses",
		"rsvp = {:rsvp} && member = {:member}",
		dbx.Params{"rsvp": rsvpID, "member": memberID},
	)
	if err == nil && existing != nil {
		return nil
	}
	record := core.NewRecord(collection)
	record.Set("rsvp", rsvpID)
	record.Set("member", memberID)
	record.Set("guests", 0)
	return app.Save(record)
}
