package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return err
		}
		addFieldIfMissing(rsvp, &core.BoolField{
			Name: "allow_guests",
			Help: "If checked, invited members can enter additional guests on the RSVP form.",
		})
		return app.Save(rsvp)
	}, func(app core.App) error {
		rsvp, err := app.FindCollectionByNameOrId("rsvp")
		if err != nil {
			return nil
		}
		if f := rsvp.Fields.GetByName("allow_guests"); f != nil {
			rsvp.Fields.RemoveById(f.GetId())
			return app.Save(rsvp)
		}
		return nil
	})
}
