package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if extra, err := app.FindCollectionByNameOrId("rsvp_responses"); err == nil {
			if err := app.Delete(extra); err != nil {
				return err
			}
		}

		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return err
		}
		members, err := app.FindCollectionByNameOrId("members")
		if err != nil {
			return err
		}

		responses, err := app.FindCollectionByNameOrId("rsvp_response")
		if err != nil {
			responses = core.NewBaseCollection("rsvp_response")
			responses.ListRule = nil
			responses.ViewRule = nil
			responses.CreateRule = nil
			responses.UpdateRule = nil
			responses.DeleteRule = nil
		}

		if !hasAnyField(responses, "rsvp", "event") {
			responses.Fields.Add(&core.RelationField{
				Name:          "rsvp",
				CollectionId:  rsvp.Id,
				Required:      true,
				MaxSelect:     1,
				CascadeDelete: true,
			})
		}
		if !hasAnyField(responses, "member", "user") {
			responses.Fields.Add(&core.RelationField{
				Name:         "member",
				CollectionId: members.Id,
				Required:     true,
				MaxSelect:    1,
			})
		}
		if !hasAnyField(responses, "status", "response", "attending", "accepted") {
			responses.Fields.Add(&core.SelectField{
				Name:      "status",
				Values:    []string{"accept", "decline"},
				MaxSelect: 1,
			})
		}
		if responses.Fields.GetByName("guests") == nil {
			responses.Fields.Add(&core.NumberField{
				Name:    "guests",
				Min:     floatPtr(0),
				OnlyInt: true,
			})
		}
		if responses.Fields.GetByName("note") == nil {
			responses.Fields.Add(&core.TextField{
				Name: "note",
				Max:  2000,
			})
		}

		// Save new fields first. Existing duplicate (rsvp, member) rows
		// must be cleaned up before the unique index can be created.
		if err := app.Save(responses); err != nil {
			return err
		}

		responses, err = app.FindCollectionByNameOrId("rsvp_response")
		if err != nil {
			return err
		}
		rsvpField := firstFieldName(responses, "rsvp", "event")
		memberField := firstFieldName(responses, "member", "user")
		if rsvpField != "" && memberField != "" {
			if err := dedupeRSVPResponses(app, rsvpField, memberField); err != nil {
				return err
			}
		}
		if rsvpField != "" && memberField != "" && responses.GetIndex("idx_rsvp_response_unique") == "" {
			responses.AddIndex("idx_rsvp_response_unique", true, rsvpField+", "+memberField, "")
			if err := app.Save(responses); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		responses, err := app.FindCollectionByNameOrId("rsvp_response")
		if err != nil {
			return nil
		}
		responses.RemoveIndex("idx_rsvp_response_unique")
		for _, name := range []string{"status", "guests", "note"} {
			if f := responses.Fields.GetByName(name); f != nil {
				responses.Fields.RemoveById(f.GetId())
			}
		}
		return app.Save(responses)
	})
}

func hasAnyField(collection *core.Collection, names ...string) bool {
	return firstFieldName(collection, names...) != ""
}

func firstFieldName(collection *core.Collection, names ...string) string {
	for _, name := range names {
		if collection.Fields.GetByName(name) != nil {
			return name
		}
	}
	return ""
}

func dedupeRSVPResponses(app core.App, rsvpField, memberField string) error {
	records, err := app.FindAllRecords("rsvp_response")
	if err != nil {
		return err
	}

	type pair struct{ rsvp, member string }
	best := map[pair]*core.Record{}
	var remove []*core.Record

	for _, record := range records {
		rsvpID := record.GetString(rsvpField)
		memberID := record.GetString(memberField)
		if rsvpID == "" || memberID == "" {
			remove = append(remove, record)
			continue
		}
		key := pair{rsvp: rsvpID, member: memberID}
		existing := best[key]
		if existing == nil {
			best[key] = record
			continue
		}
		if preferRSVPResponse(record, existing) {
			remove = append(remove, existing)
			best[key] = record
			continue
		}
		remove = append(remove, record)
	}

	for _, record := range remove {
		if err := app.Delete(record); err != nil {
			return fmt.Errorf("dedupe rsvp_response %s: %w", record.Id, err)
		}
	}
	return nil
}

func preferRSVPResponse(candidate, current *core.Record) bool {
	candStatus := rsvpResponseHasDecision(candidate)
	currStatus := rsvpResponseHasDecision(current)
	if candStatus != currStatus {
		return candStatus
	}
	return candidate.GetDateTime("updated").Time().After(current.GetDateTime("updated").Time())
}

func rsvpResponseHasDecision(record *core.Record) bool {
	for _, name := range []string{"status", "response"} {
		if record.GetString(name) != "" {
			return true
		}
	}
	return record.GetBool("attending") || record.GetBool("accepted")
}
