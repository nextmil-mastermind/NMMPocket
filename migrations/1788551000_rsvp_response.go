package migrations

import (
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

		rsvpField := firstFieldName(responses, "rsvp", "event")
		memberField := firstFieldName(responses, "member", "user")
		if rsvpField != "" && memberField != "" && responses.GetIndex("idx_rsvp_response_unique") == "" {
			responses.AddIndex("idx_rsvp_response_unique", true, rsvpField+", "+memberField, "")
		}

		return app.Save(responses)
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
