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
		return replaceEmailTemplateRelation(app, emailBasic.Id)
	}, func(app core.App) error {
		emailTemplates, err := app.FindCollectionByNameOrId("email_templates")
		if err != nil {
			return nil
		}
		return replaceEmailTemplateRelation(app, emailTemplates.Id)
	})
}

func replaceEmailTemplateRelation(app core.App, collectionID string) error {
	rsvp, err := app.FindCollectionByNameOrId("rsvp")
	if err != nil {
		return err
	}

	help := "email_basic template used when sending RSVP emails. Include {{params.rsvp_url}}."
	if f, ok := rsvp.Fields.GetByName("email_template").(*core.RelationField); ok {
		if f.CollectionId == collectionID {
			return nil
		}
		rsvp.Fields.RemoveById(f.GetId())
		if err := app.Save(rsvp); err != nil {
			return err
		}
		rsvp, err = app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return err
		}
	}

	addFieldIfMissing(rsvp, &core.RelationField{
		Name:         "email_template",
		CollectionId: collectionID,
		MaxSelect:    1,
		Help:         help,
	})
	return app.Save(rsvp)
}
