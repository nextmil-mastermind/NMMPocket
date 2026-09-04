package rsvp

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const responseCollection = "rsvp_response"

type responseSchema struct {
	Collection  *core.Collection
	RSVPField   string
	MemberField string
	StatusField string
	GuestsField string
	NoteField   string
	BoolStatus  bool
}

func loadResponseSchema(app core.App) (*responseSchema, error) {
	collection, err := app.FindCollectionByNameOrId(responseCollection)
	if err != nil {
		return nil, err
	}
	s := &responseSchema{
		Collection:  collection,
		RSVPField:   firstPresentField(collection, "rsvp", "event"),
		MemberField: firstPresentField(collection, "member", "user"),
		StatusField: firstPresentField(collection, "status", "response", "attending", "accepted"),
		GuestsField: firstPresentField(collection, "guests"),
		NoteField:   firstPresentField(collection, "note"),
	}
	if s.RSVPField == "" || s.MemberField == "" {
		return nil, fmt.Errorf("rsvp_response is missing rsvp/member relations")
	}
	if s.StatusField != "" {
		if _, ok := collection.Fields.GetByName(s.StatusField).(*core.BoolField); ok {
			s.BoolStatus = true
		}
	}
	return s, nil
}

func firstPresentField(collection *core.Collection, names ...string) string {
	for _, name := range names {
		if collection.Fields.GetByName(name) != nil {
			return name
		}
	}
	return ""
}

func (s *responseSchema) find(app core.App, rsvpID, memberID string) (*core.Record, error) {
	filter := s.RSVPField + " = {:rsvp} && " + s.MemberField + " = {:member}"
	return app.FindFirstRecordByFilter(responseCollection, filter, dbx.Params{
		"rsvp":   rsvpID,
		"member": memberID,
	})
}

func (s *responseSchema) setStatus(record *core.Record, accept bool) {
	if s.StatusField == "" {
		return
	}
	if s.BoolStatus {
		record.Set(s.StatusField, accept)
		return
	}
	record.Set(s.StatusField, statusValue(s.Collection.Fields.GetByName(s.StatusField), accept))
}

func (s *responseSchema) readStatus(record *core.Record) string {
	if s.StatusField == "" || record == nil {
		return ""
	}
	if s.BoolStatus {
		if record.GetBool(s.StatusField) {
			return "accept"
		}
		return "decline"
	}
	raw := record.GetString(s.StatusField)
	switch raw {
	case "accept", "yes", "true", "accepted", "attending":
		return "accept"
	case "decline", "no", "false", "declined":
		return "decline"
	default:
		return raw
	}
}

func statusValue(field core.Field, accept bool) string {
	sf, ok := field.(*core.SelectField)
	if !ok || len(sf.Values) == 0 {
		if accept {
			return "accept"
		}
		return "decline"
	}
	preferAccept := []string{"accept", "yes", "accepted", "attending"}
	preferDecline := []string{"decline", "no", "declined"}
	if accept {
		if v := firstIn(sf.Values, preferAccept...); v != "" {
			return v
		}
		return sf.Values[0]
	}
	if v := firstIn(sf.Values, preferDecline...); v != "" {
		return v
	}
	if len(sf.Values) > 1 {
		return sf.Values[1]
	}
	return sf.Values[0]
}

func firstIn(values []string, names ...string) string {
	for _, name := range names {
		for _, value := range values {
			if value == name {
				return value
			}
		}
	}
	return ""
}
