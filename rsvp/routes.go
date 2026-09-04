package rsvp

import (
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	pbtemplate "github.com/pocketbase/pocketbase/tools/template"
)

func Routes(r *router.Router[*core.RequestEvent]) {
	g := r.Group("/rsvp")
	g.BindFunc(loadCookieAuth)

	g.POST("/{slug}/send", handleSend).Bind(apis.RequireAuth())
	g.GET("/{slug}", handleGet)
	g.POST("/{slug}/login", handleLogin)
	g.POST("/{slug}", handleSubmit)
}

func loadCookieAuth(e *core.RequestEvent) error {
	if e.Auth != nil {
		return e.Next()
	}
	cookie, err := e.Request.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return e.Next()
	}
	record, err := e.App.FindAuthRecordByToken(cookie.Value, core.TokenTypeAuth)
	if err == nil && record != nil && record.Collection().Name == "members" {
		e.Auth = record
	}
	return e.Next()
}

func handleSend(e *core.RequestEvent) error {
	if e.Auth == nil || e.Auth.Collection().Name != "users" {
		return apis.NewUnauthorizedError("Unauthorized.", nil)
	}
	event, err := FindBySlug(e.App, e.Request.PathValue("slug"))
	if err != nil {
		return apis.NewNotFoundError("RSVP not found.", err)
	}
	result, err := SendEmails(e.App, event)
	if err != nil {
		return apis.NewBadRequestError(err.Error(), err)
	}
	return e.JSON(http.StatusOK, result)
}

func handleGet(e *core.RequestEvent) error {
	event, err := FindBySlug(e.App, e.Request.PathValue("slug"))
	if err != nil {
		return renderPage(e, http.StatusNotFound, "error.html", map[string]any{
			"title":   "RSVP not found",
			"message": "We could not find this RSVP.",
		})
	}

	if token := e.Request.URL.Query().Get("token"); token != "" {
		record, err := e.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
		if err != nil || record == nil || record.Collection().Name != "members" {
			return renderPage(e, http.StatusUnauthorized, "error.html", map[string]any{
				"title":   rsvpTitle(event),
				"message": "This RSVP link is invalid or has expired. Please log in to continue.",
				"slug":    event.GetString("slug"),
			})
		}
		setAuthCookie(e, token)
		return e.Redirect(http.StatusFound, "/rsvp/"+event.GetString("slug"))
	}

	if e.Auth == nil || e.Auth.Collection().Name != "members" {
		return renderLogin(e, event, "")
	}
	return renderMemberView(e, event, e.Auth, "")
}

func handleLogin(e *core.RequestEvent) error {
	event, err := FindBySlug(e.App, e.Request.PathValue("slug"))
	if err != nil {
		return renderPage(e, http.StatusNotFound, "error.html", map[string]any{
			"title":   "RSVP not found",
			"message": "We could not find this RSVP.",
		})
	}

	if err := e.Request.ParseForm(); err != nil {
		return renderLogin(e, event, "Invalid request.")
	}
	username := strings.TrimSpace(e.Request.FormValue("username"))
	password := e.Request.FormValue("password")
	if username == "" || password == "" {
		return renderLogin(e, event, "Please enter your email and password.")
	}

	member, err := findMemberByLogin(e.App, username)
	if err != nil || member == nil || !member.ValidatePassword(password) {
		return renderLogin(e, event, "Invalid credentials.")
	}

	token, err := member.NewAuthToken()
	if err != nil {
		return renderLogin(e, event, "Unable to start a session. Please try again.")
	}
	setAuthCookie(e, token)
	e.Auth = member
	return renderMemberView(e, event, member, "")
}

func handleSubmit(e *core.RequestEvent) error {
	event, err := FindBySlug(e.App, e.Request.PathValue("slug"))
	if err != nil {
		return renderPage(e, http.StatusNotFound, "error.html", map[string]any{
			"title":   "RSVP not found",
			"message": "We could not find this RSVP.",
		})
	}
	if e.Auth == nil || e.Auth.Collection().Name != "members" {
		return renderLogin(e, event, "Please log in to submit your RSVP.")
	}
	if !IsMemberInvited(event, e.Auth) {
		return renderNotInvited(e, event)
	}
	if !eventIsOpen(event) {
		return renderMemberView(e, event, e.Auth, "This RSVP is closed.")
	}

	if err := e.Request.ParseForm(); err != nil {
		return renderMemberView(e, event, e.Auth, "Invalid request.")
	}
	status := strings.ToLower(strings.TrimSpace(e.Request.FormValue("status")))
	if status == "yes" {
		status = "accept"
	}
	if status == "no" {
		status = "decline"
	}
	if status != "accept" && status != "decline" {
		return renderMemberView(e, event, e.Auth, "Please choose Accept or Decline.")
	}
	guests, _ := strconv.Atoi(e.Request.FormValue("guests"))
	if guests < 0 {
		guests = 0
	}
	note := strings.TrimSpace(e.Request.FormValue("note"))

	schema, err := loadResponseSchema(e.App)
	if err != nil {
		return renderMemberView(e, event, e.Auth, "Unable to save your RSVP.")
	}
	record, err := schema.find(e.App, event.Id, e.Auth.Id)
	if err != nil {
		record = core.NewRecord(schema.Collection)
		record.Set(schema.RSVPField, event.Id)
		record.Set(schema.MemberField, e.Auth.Id)
	}
	schema.setStatus(record, status == "accept")
	if schema.GuestsField != "" {
		record.Set(schema.GuestsField, guests)
	}
	if schema.NoteField != "" {
		record.Set(schema.NoteField, note)
	}
	if err := e.App.Save(record); err != nil {
		return renderMemberView(e, event, e.Auth, "Unable to save your RSVP.")
	}
	return renderPage(e, http.StatusOK, "confirmation.html", map[string]any{
		"title":       rsvpTitle(event),
		"slug":        event.GetString("slug"),
		"status":      status,
		"statusLabel": statusLabel(status),
		"guests":      guests,
		"note":        note,
		"hasGuests":   schema.GuestsField != "",
	})
}

func renderMemberView(e *core.RequestEvent, event, member *core.Record, formError string) error {
	if !IsMemberInvited(event, member) {
		return renderNotInvited(e, event)
	}

	schema, err := loadResponseSchema(e.App)
	if err != nil {
		return renderPage(e, http.StatusInternalServerError, "error.html", map[string]any{
			"title":   rsvpTitle(event),
			"message": "Unable to load this RSVP.",
		})
	}
	existing, _ := schema.find(e.App, event.Id, member.Id)
	status := schema.readStatus(existing)
	guests := 0
	note := ""
	if existing != nil {
		if schema.GuestsField != "" {
			guests = existing.GetInt(schema.GuestsField)
		}
		if schema.NoteField != "" {
			note = existing.GetString(schema.NoteField)
		}
	}

	msg := strings.TrimSpace(event.GetString("message"))
	var body any
	if msg != "" {
		body = template.HTML(msg)
	}

	return renderPage(e, http.StatusOK, "form.html", map[string]any{
		"title":     rsvpTitle(event),
		"slug":      event.GetString("slug"),
		"firstName": member.GetString("first_name"),
		"open":      eventIsOpen(event),
		"message":   body,
		"status":    status,
		"guests":    guests,
		"note":      note,
		"hasGuests": schema.GuestsField != "",
		"hasNote":   schema.NoteField != "",
		"error":     formError,
	})
}

func statusLabel(status string) string {
	if status == "accept" {
		return "Accepted"
	}
	if status == "decline" {
		return "Declined"
	}
	return status
}

func renderNotInvited(e *core.RequestEvent, event *core.Record) error {
	msg := strings.TrimSpace(event.GetString("not_invited_message"))
	var body any
	if msg == "" {
		body = defaultNotInvited
	} else {
		body = template.HTML(msg)
	}
	return renderPage(e, http.StatusOK, "not_invited.html", map[string]any{
		"title":   rsvpTitle(event),
		"message": body,
	})
}

func renderLogin(e *core.RequestEvent, event *core.Record, errMsg string) error {
	return renderPage(e, http.StatusOK, "login.html", map[string]any{
		"title": rsvpTitle(event),
		"slug":  event.GetString("slug"),
		"error": errMsg,
	})
}

func renderPage(e *core.RequestEvent, status int, name string, data map[string]any) error {
	html, err := pbtemplate.NewRegistry().LoadFiles(templatePath(name)).Render(data)
	if err != nil {
		return apis.NewInternalServerError("Failed to render page", err)
	}
	return e.HTML(status, html)
}

func templatePath(name string) string {
	if os.Getenv("is_prod") == "true" {
		return "/pb/rsvphtml/" + name
	}
	return "rsvp/html/" + name
}

func setAuthCookie(e *core.RequestEvent, token string) {
	http.SetCookie(e.Response, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/rsvp",
		HttpOnly: true,
		Secure:   e.IsTLS() || os.Getenv("is_prod") == "true",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   cookieMaxAge,
	})
}

func findMemberByLogin(app core.App, login string) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId("members")
	if err != nil {
		return nil, err
	}
	if rec, err := app.FindAuthRecordByEmail(collection, login); err == nil {
		return rec, nil
	}
	return app.FindFirstRecordByFilter(
		"members",
		"username = {:login} || email = {:login}",
		dbx.Params{"login": login},
	)
}
