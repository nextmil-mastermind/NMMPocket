package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		emailBasic, err := app.FindCollectionByNameOrId("email_basic")
		if err != nil {
			return err
		}
		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return err
		}

		if f, ok := rsvp.Fields.GetByName("email_template").(*core.RelationField); ok {
			f.CollectionId = emailBasic.Id
			f.MaxSelect = 1
			f.Help = "email_basic template used when sending RSVP emails. Include {{params.rsvp_url}}."
		} else {
			addFieldIfMissing(rsvp, &core.RelationField{
				Name:         "email_template",
				CollectionId: emailBasic.Id,
				MaxSelect:    1,
				Help:         "email_basic template used when sending RSVP emails. Include {{params.rsvp_url}}.",
			})
		}
		if err := app.Save(rsvp); err != nil {
			return err
		}

		records, err := app.FindAllRecords("rsvp")
		if err != nil {
			return err
		}
		for _, record := range records {
			id := record.GetString("email_template")
			if id == "" {
				continue
			}
			if _, err := app.FindRecordById("email_basic", id); err == nil {
				continue
			}
			record.Set("email_template", "")
			if err := app.Save(record); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		emailTemplates, err := app.FindCollectionByNameOrId("email_templates")
		if err != nil {
			return nil
		}
		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return nil
		}
		if f, ok := rsvp.Fields.GetByName("email_template").(*core.RelationField); ok {
			f.CollectionId = emailTemplates.Id
			return app.Save(rsvp)
		}
		return nil
	})
}
