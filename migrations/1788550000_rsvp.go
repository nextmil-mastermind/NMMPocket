package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		members, err := app.FindCollectionByNameOrId("members")
		if err != nil {
			return err
		}
		emailTemplates, err := app.FindCollectionByNameOrId("email_templates")
		if err != nil {
			return err
		}

		groupValues := []string{"founder"}
		if gf, ok := members.Fields.GetByName("group").(*core.SelectField); ok && len(gf.Values) > 0 {
			groupValues = gf.Values
		}

		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			rsvp = core.NewBaseCollection("rsvp")
			rsvp.Fields.Add(&core.TextField{
				Name:        "title",
				Required:    true,
				Presentable: true,
			})
		}

		addFieldIfMissing(rsvp, &core.TextField{
			Name:     "slug",
			Required: true,
			Pattern:  `^[a-z0-9]+(?:-[a-z0-9]+)*$`,
			Help:     "URL name, e.g. q4-workshop. Used in /rsvp/{slug}.",
		})
		addFieldIfMissing(rsvp, &core.RelationField{
			Name:         "members",
			CollectionId: members.Id,
			MaxSelect:    2000,
			Help:         "Optional explicit invite list. Empty means no extra ID filter.",
		})
		addFieldIfMissing(rsvp, &core.SelectField{
			Name:      "groups",
			Values:    groupValues,
			MaxSelect: max(len(groupValues), 2),
			Help:      "Optional group filter. Empty means any group.",
		})
		addFieldIfMissing(rsvp, &core.BoolField{
			Name: "invite_active_only",
			Help: "If checked, only members with a future expiration (or founders) are invited. Set on create; uncheck after saving to include expired members.",
		})
		addFieldIfMissing(rsvp, &core.RelationField{
			Name:         "email_template",
			CollectionId: emailTemplates.Id,
			MaxSelect:    1,
			Help:         "Template used when sending RSVP emails.",
		})
		addFieldIfMissing(rsvp, &core.DateField{
			Name: "sent_at",
			Help: "Set automatically when RSVP emails are sent.",
		})
		addFieldIfMissing(rsvp, &core.BoolField{
			Name: "open",
			Help: "If unchecked, members can view but cannot submit. Set on create; uncheck after saving to close.",
		})
		addFieldIfMissing(rsvp, &core.EditorField{
			Name: "not_invited_message",
			Help: "Shown to logged-in members who are not invited. Leave empty for the default message.",
		})

		if rsvp.Fields.GetByName("title") == nil {
			rsvp.Fields.Add(&core.TextField{
				Name:        "title",
				Required:    true,
				Presentable: true,
			})
		}

		rsvp.AddIndex("idx_rsvp_slug", true, "slug", "")

		if err := app.Save(rsvp); err != nil {
			return err
		}

		if _, err := app.FindCollectionByNameOrId("rsvp_responses"); err == nil {
			return nil
		}

		responses := core.NewBaseCollection("rsvp_responses")
		responses.ListRule = nil
		responses.ViewRule = nil
		responses.CreateRule = nil
		responses.UpdateRule = nil
		responses.DeleteRule = nil
		responses.Fields.Add(
			&core.RelationField{
				Name:          "rsvp",
				CollectionId:  rsvp.Id,
				Required:      true,
				MaxSelect:     1,
				CascadeDelete: true,
			},
			&core.RelationField{
				Name:         "member",
				CollectionId: members.Id,
				Required:     true,
				MaxSelect:    1,
			},
			&core.SelectField{
				Name:      "status",
				Values:    []string{"yes", "no", "maybe"},
				MaxSelect: 1,
			},
			&core.NumberField{
				Name:    "guests",
				Min:     floatPtr(0),
				OnlyInt: true,
			},
			&core.TextField{
				Name: "note",
				Max:  2000,
			},
		)
		responses.AddIndex("idx_rsvp_responses_unique", true, "rsvp, member", "")

		return app.Save(responses)
	}, func(app core.App) error {
		if responses, err := app.FindCollectionByNameOrId("rsvp_responses"); err == nil {
			if err := app.Delete(responses); err != nil {
				return err
			}
		}

		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return nil
		}
		rsvp.RemoveIndex("idx_rsvp_slug")
		for _, name := range []string{
			"slug", "members", "groups", "invite_active_only",
			"email_template", "sent_at", "open", "not_invited_message",
		} {
			if f := rsvp.Fields.GetByName(name); f != nil {
				rsvp.Fields.RemoveById(f.GetId())
			}
		}
		return app.Save(rsvp)
	})
}

func addFieldIfMissing(collection *core.Collection, field core.Field) {
	if collection.Fields.GetByName(field.GetName()) == nil {
		collection.Fields.Add(field)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
