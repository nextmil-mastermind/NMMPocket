package rsvp

import (
	"testing"

	pbtemplate "github.com/pocketbase/pocketbase/tools/template"
)

func TestTemplatesRender(t *testing.T) {
	pages := []string{"login.html", "form.html", "confirmation.html", "not_invited.html", "error.html"}
	data := map[string]any{
		"title":     "Test Event",
		"slug":      "test-event",
		"firstName": "Ada",
		"open":      true,
		"status":    "yes",
		"guests":    1,
		"note":      "hello",
		"error":     "",
		"message":   "not invited",
	}
	for _, name := range pages {
		html, err := pbtemplate.NewRegistry().LoadFiles("html/" + name).Render(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if html == "" {
			t.Fatalf("%s rendered empty", name)
		}
	}
}
