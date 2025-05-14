package authentication

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
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

// Convert OAuthSession to map for database storage
func (s *OAuthSession) toMap() map[string]any {
	return map[string]any{
		"client_id":    s.ClientID,
		"redirect_uri": s.RedirectURI,
		"state":        s.State,
		"expires_at":   s.ExpiresAt,
		"auth_code":    s.AuthCode,
		"user_id":      s.UserID,
	}
}

// Create OAuthSession from database record
func newOAuthSessionFromRecord(record *core.Record) *OAuthSession {
	return &OAuthSession{
		ClientID:    record.GetString("client_id"),
		RedirectURI: record.GetString("redirect_uri"),
		State:       record.GetString("state"),
		ExpiresAt:   record.GetDateTime("expires_at").Time(),
		AuthCode:    record.GetString("auth_code"),
		UserID:      record.GetString("user_id"),
	}
}

// validateRedirectURI checks if the provided redirect URI matches the allowed one, ignoring dynamic parts
func validateRedirectURI(allowedURI, providedURI string) bool {
	// Extract base domains
	allowedBase := extractBaseDomain(allowedURI)
	providedBase := extractBaseDomain(providedURI)

	// Check if base domains match
	return allowedBase == providedBase
}

// extractBaseDomain extracts the base domain from a URI, removing dynamic parts
func extractBaseDomain(uri string) string {
	// Remove protocol
	uri = strings.TrimPrefix(uri, "http://")
	uri = strings.TrimPrefix(uri, "https://")

	// Split by path segments
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return ""
	}

	// Return just the domain
	return parts[0]
}

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

	allowedURI := oauthApp.GetString("redirect_uri")
	if !validateRedirectURI(allowedURI, redirectURI) {
		return apis.NewForbiddenError("Invalid redirect_uri", nil)
	}

	// Store OAuth session data
	sessionID := security.RandomString(32)
	session := &OAuthSession{
		ClientID:    clientID,
		RedirectURI: redirectURI,
		State:       state,
		ExpiresAt:   time.Now().Add(10 * time.Minute), // Session expires in 10 minutes
	}

	// Create session record
	collection, err := e.App.FindCollectionByNameOrId("oauth_sessions")
	if err != nil {
		return apis.NewInternalServerError("Failed to access sessions collection", err)
	}

	record := core.NewRecord(collection)
	record.Load(session.toMap())
	record.Id = sessionID

	if err := e.App.Save(record); err != nil {
		return apis.NewInternalServerError("Failed to save session", err)
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
	var templatePath string
	if os.Getenv("is_prod") == "true" {
		templatePath = "/pb/authhtml/login.html"
	} else {
		templatePath = "authentication/html/login.html"
	}

	html, err := template.NewRegistry().LoadFiles(
		templatePath,
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

	// Get session record
	collection, err := e.App.FindCollectionByNameOrId("oauth_sessions")
	if err != nil {
		return apis.NewInternalServerError("Failed to access sessions collection", err)
	}

	record, err := e.App.FindRecordById(collection, sessionCookie.Value)
	if err != nil {
		return apis.NewBadRequestError("Session expired", nil)
	}

	session := newOAuthSessionFromRecord(record)
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		e.App.Delete(record)
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
	collectionString := oauthApp.GetString("collection")
	if collectionString == "" {
		return apis.NewBadRequestError("Invalid client configuration", nil)
	}

	// Get the auth collection
	authCollection, err := e.App.FindCollectionByNameOrId(collectionString)
	if err != nil {
		return apis.NewBadRequestError("Invalid client configuration", nil)
	}

	// Authenticate user using the specified collection
	authRecord, err := e.App.FindAuthRecordByEmail(authCollection, username)
	if err != nil {
		return apis.NewBadRequestError("Invalid credentials", nil)
	}

	// Verify password
	if !authRecord.ValidatePassword(password) {
		return apis.NewBadRequestError("Invalid credentials", nil)
	}

	// Generate authorization code
	authCode := security.RandomString(32)

	// Update session with auth code and user ID
	session.AuthCode = authCode
	session.UserID = authRecord.Id

	// Update the record with new values
	record.Set("auth_code", authCode)
	record.Set("user_id", authRecord.Id)

	if err := e.App.Save(record); err != nil {
		return apis.NewInternalServerError("Failed to update session", err)
	}

	// Redirect back to client with authorization code
	redirectURL := session.RedirectURI
	if redirectURL[len(redirectURL)-1] != '?' {
		redirectURL += "?"
	}
	redirectURL += "code=" + authCode + "&state=" + session.State

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

	allowedURI := oauthApp.GetString("redirect_uri")
	if !validateRedirectURI(allowedURI, redirectURI) {
		return apis.NewForbiddenError("Invalid redirect_uri", nil)
	}

	// Find session with matching auth code
	collection, err := e.App.FindCollectionByNameOrId("oauth_sessions")
	if err != nil {
		return apis.NewInternalServerError("Failed to access sessions collection", err)
	}

	records, err := e.App.FindRecordsByFilter(
		collection,
		"auth_code = {:code} && client_id = {:client_id}",
		"",
		1,
		0,
		dbx.Params{"code": code, "client_id": clientID},
	)
	if err != nil {
		return apis.NewInternalServerError("Failed to find session", err)
	}

	if len(records) == 0 {
		return apis.NewBadRequestError("Invalid authorization code", nil)
	}

	session := newOAuthSessionFromRecord(records[0])

	user, err := e.App.FindRecordById("users", session.UserID)
	if err != nil {
		return apis.NewBadRequestError("Invalid user", nil)
	}

	token, err := user.NewAuthToken()
	if err != nil {
		return apis.NewBadRequestError("Invalid user", nil)
	}

	// Now that we've successfully generated the token, we can delete the session
	if err := e.App.Delete(records[0]); err != nil {
		fmt.Printf("Warning: Failed to delete session: %v\n", err)
	}

	// Return token response
	return e.JSON(http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   1209600, // 14 days
	})
}

// handleUserInfoRoute handles the OAuth2 userinfo endpoint
func handleUserInfoRoute(e *core.RequestEvent) error {
	loggedInUser := e.Auth
	if loggedInUser == nil {
		return apis.NewBadRequestError("Not logged in", nil)
	}
	//check collection, if collection is users then return name, if collection is members then return first_name + " " + last_name
	user_data := map[string]any{}
	if loggedInUser.GetString("collectionName") == "users" {
		user_data["name"] = loggedInUser.GetString("name")
	} else if loggedInUser.GetString("collectionName") == "members" {
		user_data["name"] = loggedInUser.GetString("first_name") + " " + loggedInUser.GetString("last_name")
	}
	user_data["email"] = loggedInUser.GetString("email")
	user_data["sub"] = loggedInUser.Id
	user_data["picture"] = ""
	if loggedInUser.GetString("avatar") != "" {
		user_data["picture"] = "https://pocket.nextmil.org/api/files/" + loggedInUser.GetString("collectionName") + "/" + loggedInUser.Id + "/" + loggedInUser.GetString("avatar")
	}

	return e.JSON(http.StatusOK, user_data)
}

// RegisterOAuthRoutes registers the OAuth2 routes
func RegisterOAuthRoutes(router *router.Router[*core.RequestEvent]) {
	// OAuth2 routes
	router.GET("/oauth/login", handleLoginGetRoute)
	router.POST("/oauth/login", handleLoginPostRoute)
	router.POST("/oauth/token", handleTokenRoute)
	router.GET("/oauth/userinfo", handleUserInfoRoute).Bind(apis.RequireAuth())
}
