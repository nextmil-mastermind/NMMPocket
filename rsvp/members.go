package rsvp

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func rsvpTitle(record *core.Record) string {
	title := record.GetString("title")
	if title == "" {
		title = record.GetString("name")
	}
	if title == "" {
		title = record.GetString("slug")
	}
	return title
}

func FindBySlug(app core.App, slug string) (*core.Record, error) {
	return app.FindFirstRecordByFilter("rsvp", "slug = {:slug}", dbx.Params{"slug": slug})
}

func IsMemberInvited(event, member *core.Record) bool {
	if member.GetString("email") == infoEmail {
		return false
	}
	if event.GetBool("invite_active_only") {
		exp := eventExpiration(member)
		if !exp.After(time.Now()) && member.GetString("group") != founderGroup {
			return false
		}
	}
	groups := event.GetStringSlice("groups")
	if len(groups) > 0 && !slices.Contains(groups, member.GetString("group")) {
		return false
	}
	invited := event.GetStringSlice("members")
	if len(invited) > 0 && !slices.Contains(invited, member.Id) {
		return false
	}
	return true
}

func eventExpiration(member *core.Record) time.Time {
	return member.GetDateTime("expiration").Time()
}

func ResolveMembers(app core.App, event *core.Record) ([]*core.Record, error) {
	var parts []string
	params := dbx.Params{}

	parts = append(parts, "email != {:infoEmail}")
	params["infoEmail"] = infoEmail

	if event.GetBool("invite_active_only") {
		parts = append(parts, "(expiration > @now || group = {:founder})")
		params["founder"] = founderGroup
	}

	groups := event.GetStringSlice("groups")
	if len(groups) > 0 {
		parts = append(parts, "group ?= {:groups}")
		params["groups"] = groups
	}

	ids := event.GetStringSlice("members")
	if len(ids) > 0 {
		parts = append(parts, "id ?= {:ids}")
		params["ids"] = ids
	}

	filter := strings.Join(parts, " && ")
	records, err := app.FindRecordsByFilter("members", filter, "-expiration", 0, 0, params)
	if err != nil {
		return nil, fmt.Errorf("resolve invited members: %w", err)
	}
	return records, nil
}
