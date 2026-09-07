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
	record, err := app.FindFirstRecordByFilter("rsvp", "slug = {:slug}", dbx.Params{"slug": slug})
	if err != nil {
		return nil, err
	}
	_ = app.ExpandRecord(record, []string{"members"}, nil)
	return record, nil
}

func IsMemberInvited(event, member *core.Record) bool {
	if event.GetBool("invite_active_only") {
		exp := memberExpiration(member)
		if !exp.After(time.Now()) && member.GetString("group") != founderGroup {
			return false
		}
	}
	groups := nonzeroStrings(event.GetStringSlice("groups"))
	if len(groups) > 0 && !slices.Contains(groups, member.GetString("group")) {
		return false
	}
	ids := eventMemberIDs(event)
	if len(ids) > 0 {
		if !slices.Contains(ids, member.Id) {
			return false
		}
	} else if !event.GetBool("members_only") {
		return false
	}
	return true
}

func eventAllowsGuests(event *core.Record) bool {
	return event.GetBool("allow_guests")
}

func eventIsOpen(event *core.Record) bool {
	if !event.GetBool("open") {
		return false
	}
	deadline := event.GetDateTime("expiration").Time()
	if !deadline.IsZero() && !deadline.After(time.Now()) {
		return false
	}
	return true
}

func memberExpiration(member *core.Record) time.Time {
	return member.GetDateTime("expiration").Time()
}

func eventMemberIDs(event *core.Record) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, id := range event.GetStringSlice("members") {
		add(id)
	}
	switch raw := event.Get("members").(type) {
	case []string:
		for _, id := range raw {
			add(id)
		}
	case []any:
		for _, item := range raw {
			switch v := item.(type) {
			case string:
				add(v)
			case *core.Record:
				add(v.Id)
			}
		}
	}
	for _, rec := range event.ExpandedAll("members") {
		add(rec.Id)
	}
	return ids
}

func nonzeroStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func ResolveMembers(app core.App, event *core.Record) ([]*core.Record, error) {
	_ = app.ExpandRecord(event, []string{"members"}, nil)

	ids := eventMemberIDs(event)
	var records []*core.Record
	if len(ids) > 0 {
		found, err := app.FindRecordsByIds("members", ids)
		if err != nil {
			return nil, fmt.Errorf("resolve invited members: %w", err)
		}
		records = found
	} else if event.GetBool("members_only") {
		found, err := app.FindRecordsByFilter("members", "id != ''", "-expiration", 0, 0)
		if err != nil {
			return nil, fmt.Errorf("resolve invited members: %w", err)
		}
		records = found
	} else {
		return nil, nil
	}

	out := make([]*core.Record, 0, len(records))
	for _, member := range records {
		if IsMemberInvited(event, member) {
			out = append(out, member)
		}
	}
	return out, nil
}
