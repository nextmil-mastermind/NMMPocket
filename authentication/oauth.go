package authentication

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/template"
)

// OAuthSession represents the data we need to store during the OAuth flow
type OAuthSession struct {
	ClientID    string
	RedirectURI string
	State       string
	ExpiresAt   time.Time
	AuthCode    string
	UserID      string
}

// In-memory store for OAuth sessions (in production, use a proper session store)
var oauthSessions = make(map[string]*OAuthSession)

func handleLoginGetRoute(e *core.RequestEvent) error {
	// Extract OAuth2 parameters
	clientID := e.Request.URL.Query().Get("client_id")
	redirectURI := e.Request.URL.Query().Get("redirect_uri")
	state := e.Request.URL.Query().Get("state")
	responseType := e.Request.URL.Query().Get("response_type")

	// Validate required parameters
	if clientID == "" || redirectURI == "" || state == "" {
		return apis.NewBadRequestError("Missing required OAuth2 parameters", nil)
	}

	// Validate response_type
	if responseType != "code" {
		return apis.NewBadRequestError("Invalid response_type. Only 'code' is supported", nil)
	}

	// Validate the client_id and redirect_uri
	oauthApp, err := e.App.FindRecordById("oauth_apps", clientID)
	if err != nil {
		return apis.NewForbiddenError("Invalid client_id", nil)
	}

	if oauthApp.GetString("redirect_uri") != redirectURI {
		return apis.NewForbiddenError("Invalid redirect_uri", nil)
	}

	// Store OAuth session data
	sessionID := security.RandomString(32)
	oauthSessions[sessionID] = &OAuthSession{
		ClientID:    clientID,
		RedirectURI: redirectURI,
		State:       state,
		ExpiresAt:   time.Now().Add(10 * time.Minute), // Session expires in 10 minutes
	}

	// Set session cookie
	http.SetCookie(e.Response, &http.Cookie{
		Name:     "oauth_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	// Render login page
	html, err := template.NewRegistry().LoadFiles(
		"authentication/html/login.html",
	).Render(map[string]any{
		"client_name": oauthApp.GetString("name"),
	})
	if err != nil {
		return apis.NewInternalServerError("Failed to render login page", err)
	}

	return e.HTML(http.StatusOK, html)
}

// handleLoginPostRoute handles the login form submission
func handleLoginPostRoute(e *core.RequestEvent) error {
	// Get session ID from cookie
	sessionCookie, err := e.Request.Cookie("oauth_session")
	if err != nil {
		return apis.NewBadRequestError("Invalid session", nil)
	}

	// Get session data
	session, exists := oauthSessions[sessionCookie.Value]
	if !exists || time.Now().After(session.ExpiresAt) {
		return apis.NewBadRequestError("Session expired", nil)
	}

	// Parse login form data
	username := e.Request.FormValue("username")
	password := e.Request.FormValue("password")

	// Get OAuth app to determine collection
	oauthApp, err := e.App.FindRecordById("oauth_apps", session.ClientID)
	if err != nil {
		return apis.NewBadRequestError("Invalid client", nil)
	}

	// Get the collection from oauth_apps
	collection := oauthApp.GetString("collection")
	if collection == "" {
		return apis.NewBadRequestError("Invalid client configuration", nil)
	}

	// Authenticate user using the specified collection
	authRecord, err := e.App.FindAuthRecordByEmail(collection, username)
	if err != nil {
		return apis.NewBadRequestError("Invalid credentials", nil)
	}

	// Verify password
	if !authRecord.ValidatePassword(password) {
		return apis.NewBadRequestError("Invalid credentials", nil)
	}

	// Generate authorization code
	authCode := security.RandomString(32)

	// Store auth code in session
	session.AuthCode = authCode
	session.UserID = authRecord.Id

	// Redirect back to client with authorization code
	redirectURL := session.RedirectURI
	if redirectURL[len(redirectURL)-1] != '?' {
		redirectURL += "?"
	}
	redirectURL += "code=" + authCode + "&state=" + session.State

	// Clean up session
	delete(oauthSessions, sessionCookie.Value)

	return e.Redirect(http.StatusFound, redirectURL)
}

// handleTokenRoute handles the OAuth2 token endpoint
func handleTokenRoute(e *core.RequestEvent) error {
	// Only allow POST requests
	if e.Request.Method != http.MethodPost {
		return apis.NewBadRequestError("Method not allowed", nil)
	}

	// Parse form data
	if err := e.Request.ParseForm(); err != nil {
		return apis.NewBadRequestError("Invalid request", err)
	}

	// Get required parameters
	grantType := e.Request.FormValue("grant_type")
	code := e.Request.FormValue("code")
	clientID := e.Request.FormValue("client_id")
	clientSecret := e.Request.FormValue("client_secret")
	redirectURI := e.Request.FormValue("redirect_uri")

	// Validate required parameters
	if grantType == "" || code == "" || clientID == "" || clientSecret == "" || redirectURI == "" {
		return apis.NewBadRequestError("Missing required parameters", nil)
	}

	// Validate grant_type
	if grantType != "authorization_code" {
		return apis.NewBadRequestError("Invalid grant_type. Only 'authorization_code' is supported", nil)
	}

	// Validate client credentials
	oauthApp, err := e.App.FindRecordById("oauth_apps", clientID)
	if err != nil {
		return apis.NewForbiddenError("Invalid client_id", nil)
	}

	if oauthApp.GetString("client_secret") != clientSecret {
		return apis.NewForbiddenError("Invalid client_secret", nil)
	}

	if oauthApp.GetString("redirect_uri") != redirectURI {
		return apis.NewForbiddenError("Invalid redirect_uri", nil)
	}

	// Find session with matching auth code
	var session *OAuthSession
	for _, s := range oauthSessions {
		if s.AuthCode == code && s.ClientID == clientID {
			session = s
			break
		}
	}

	if session == nil {
		return apis.NewBadRequestError("Invalid authorization code", nil)
	}

	user, err := e.App.FindRecordById("users", session.UserID)
	if err != nil {
		return apis.NewBadRequestError("Invalid user", nil)
	}
	token, err := user.NewAuthToken()
	if err != nil {
		return apis.NewBadRequestError("Invalid user", nil)
	}

	// Return token response
	return e.JSON(http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   3600, // 1 hour
	})
}

// RegisterOAuthRoutes registers the OAuth2 routes
func RegisterOAuthRoutes(router *router.Router[*core.RequestEvent]) {
	// OAuth2 routes
	router.GET("/oauth/login", handleLoginGetRoute)
	router.POST("/oauth/login", handleLoginPostRoute)
	router.POST("/oauth/token", handleTokenRoute)
}
