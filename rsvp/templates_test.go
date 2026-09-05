package rsvp

import (
	"strings"
	"testing"

	pbtemplate "github.com/pocketbase/pocketbase/tools/template"
)

func TestTemplatesRender(t *testing.T) {
	pages := []string{"login.html", "form.html", "confirmation.html", "not_invited.html", "error.html"}
	data := map[string]any{
		"title":       "Test Event",
		"slug":        "test-event",
		"firstName":   "Ada",
		"open":        true,
		"status":      "accept",
		"statusLabel": "Accepted",
		"guests":      1,
		"note":        "hello",
		"error":       "",
		"message":     "Event details",
		"hasGuests":   true,
		"hasNote":     true,
	}
	for _, name := range pages {
		html, err := pbtemplate.NewRegistry().LoadFiles("html/" + name).Render(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if html == "" {
			t.Fatalf("%s rendered empty", name)
		}
		if !strings.Contains(html, "/rsvp/theme.css") {
			t.Fatalf("%s missing theme stylesheet", name)
		}
	}
}
