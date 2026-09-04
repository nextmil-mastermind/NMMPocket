package rsvp

import "github.com/pocketbase/pocketbase/core"

func RegisterHooks(app core.App) {
	app.OnRecordCreate("rsvp").BindFunc(func(e *core.RecordEvent) error {
		e.Record.Set("invite_active_only", true)
		e.Record.Set("open", true)
		return e.Next()
	})
}
