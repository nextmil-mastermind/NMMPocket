package zoomcon

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func Routes(sr *router.Router[*core.RequestEvent]) {

	sr.GET("/z/m/{id}", func(e *core.RequestEvent) error {
		id := e.Request.PathValue("id")
		record, err := e.App.FindRecordById("member_zoom", id)
		if err != nil {
			return err
		}
		url := record.GetString("join_url")
		return e.Redirect(302, url)
	})
}
