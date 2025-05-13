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

	fmt.Printf("Created new session:\n")
	fmt.Printf("Session ID: %s\n", sessionID)
	fmt.Printf("Expires at: %v\n", session.ExpiresAt)

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
		fmt.Printf("Error: Failed to render login page - %v\n", err)
		return apis.NewInternalServerError("Failed to render login page", err)
	}

	fmt.Printf("=== End OAuth Login GET Request ===\n\n")
	return e.HTML(http.StatusOK, html)
}

// handleLoginPostRoute handles the login form submission
func handleLoginPostRoute(e *core.RequestEvent) error {
	fmt.Printf("\n=== OAuth Login POST Request ===\n")

	// Get session ID from cookie
	sessionCookie, err := e.Request.Cookie("oauth_session")
	if err != nil {
		fmt.Printf("Error: No session cookie found - %v\n", err)
		return apis.NewBadRequestError("Invalid session", nil)
	}

	fmt.Printf("Session ID from cookie: %s\n", sessionCookie.Value)

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

	fmt.Printf("Login attempt for user: %s\n", username)

	// Get OAuth app to determine collection
	oauthApp, err := e.App.FindRecordById("oauth_apps", session.ClientID)
	if err != nil {
		fmt.Printf("Error: Invalid client - %v\n", err)
		return apis.NewBadRequestError("Invalid client", nil)
	}

	// Get the collection from oauth_apps
	collectionString := oauthApp.GetString("collection")
	if collectionString == "" {
		fmt.Printf("Error: No collection specified in oauth_app\n")
		return apis.NewBadRequestError("Invalid client configuration", nil)
	}

	// Get the auth collection
	authCollection, err := e.App.FindCollectionByNameOrId(collectionString)
	if err != nil {
		fmt.Printf("Error: Invalid auth collection - %v\n", err)
		return apis.NewBadRequestError("Invalid client configuration", nil)
	}

	// Authenticate user using the specified collection
	authRecord, err := e.App.FindAuthRecordByEmail(authCollection, username)
	if err != nil {
		fmt.Printf("Error: User not found - %v\n", err)
		return apis.NewBadRequestError("Invalid credentials", nil)
	}

	// Verify password
	if !authRecord.ValidatePassword(password) {
		fmt.Printf("Error: Invalid password\n")
		return apis.NewBadRequestError("Invalid credentials", nil)
	}

	// Generate authorization code
	authCode := security.RandomString(32)

	// Update session with auth code and user ID
	session.AuthCode = authCode
	session.UserID = authRecord.Id
	record.Load(session.toMap())

	if err := e.App.Save(record); err != nil {
		return apis.NewInternalServerError("Failed to update session", err)
	}

	fmt.Printf("Generated auth code: %s\n", authCode)
	fmt.Printf("For user ID: %s\n", authRecord.Id)

	// Redirect back to client with authorization code
	redirectURL := session.RedirectURI
	if redirectURL[len(redirectURL)-1] != '?' {
		redirectURL += "?"
	}
	redirectURL += "code=" + authCode + "&state=" + session.State

	fmt.Printf("Redirecting to: %s\n", redirectURL)

	// Clean up session
	if err := e.App.Delete(record); err != nil {
		fmt.Printf("Warning: Failed to delete session: %v\n", err)
	}

	fmt.Printf("=== End OAuth Login POST Request ===\n\n")
	return e.Redirect(http.StatusFound, redirectURL)
}

// handleTokenRoute handles the OAuth2 token endpoint
func handleTokenRoute(e *core.RequestEvent) error {
	fmt.Printf("\n=== OAuth Token Request ===\n")
	fmt.Printf("Method: %s\n", e.Request.Method)
	fmt.Printf("Remote IP: %s\n", e.Request.RemoteAddr)

	// Only allow POST requests
	if e.Request.Method != http.MethodPost {
		fmt.Printf("Error: Invalid method\n")
		return apis.NewBadRequestError("Method not allowed", nil)
	}

	// Parse form data
	if err := e.Request.ParseForm(); err != nil {
		fmt.Printf("Error: Failed to parse form - %v\n", err)
		return apis.NewBadRequestError("Invalid request", err)
	}

	// Get required parameters
	grantType := e.Request.FormValue("grant_type")
	code := e.Request.FormValue("code")
	clientID := e.Request.FormValue("client_id")
	clientSecret := e.Request.FormValue("client_secret")
	redirectURI := e.Request.FormValue("redirect_uri")

	fmt.Printf("\nRequest Parameters:\n")
	fmt.Printf("grant_type: %s\n", grantType)
	fmt.Printf("client_id: %s\n", clientID)
	fmt.Printf("redirect_uri: %s\n", redirectURI)
	fmt.Printf("code length: %d\n", len(code))
	fmt.Printf("client_secret length: %d\n", len(clientSecret))

	// Validate required parameters
	if grantType == "" || code == "" || clientID == "" || clientSecret == "" || redirectURI == "" {
		fmt.Printf("\nError: Missing required parameters\n")
		fmt.Printf("has_grant_type: %v\n", grantType != "")
		fmt.Printf("has_code: %v\n", code != "")
		fmt.Printf("has_client_id: %v\n", clientID != "")
		fmt.Printf("has_client_secret: %v\n", clientSecret != "")
		fmt.Printf("has_redirect_uri: %v\n", redirectURI != "")
		return apis.NewBadRequestError("Missing required parameters", nil)
	}

	// Validate grant_type
	if grantType != "authorization_code" {
		fmt.Printf("\nError: Invalid grant type\n")
		return apis.NewBadRequestError("Invalid grant_type. Only 'authorization_code' is supported", nil)
	}

	// Validate client credentials
	oauthApp, err := e.App.FindRecordById("oauth_apps", clientID)
	if err != nil {
		fmt.Printf("\nError: Invalid client_id - %v\n", err)
		return apis.NewForbiddenError("Invalid client_id", nil)
	}

	if oauthApp.GetString("client_secret") != clientSecret {
		fmt.Printf("\nError: Invalid client_secret\n")
		return apis.NewForbiddenError("Invalid client_secret", nil)
	}

	allowedURI := oauthApp.GetString("redirect_uri")
	if !validateRedirectURI(allowedURI, redirectURI) {
		fmt.Printf("\nError: Invalid redirect_uri\n")
		fmt.Printf("Allowed: %s\n", allowedURI)
		fmt.Printf("Provided: %s\n", redirectURI)
		return apis.NewForbiddenError("Invalid redirect_uri", nil)
	}

	// Find session with matching auth code
	collection, err := e.App.FindCollectionByNameOrId("oauth_sessions")
	if err != nil {
		fmt.Printf("Error: Failed to find oauth_sessions collection - %v\n", err)
		return apis.NewInternalServerError("Failed to access sessions collection", err)
	}

	fmt.Printf("Searching for session with:\n")
	fmt.Printf("Auth code: %s\n", code)
	fmt.Printf("Client ID: %s\n", clientID)

	// First, let's see what sessions exist
	allRecords, err := e.App.FindRecordsByFilter(
		collection,
		"",
		"",
		100,
		0,
	)
	if err != nil {
		fmt.Printf("Error: Failed to list sessions - %v\n", err)
	} else {
		fmt.Printf("Found %d total sessions\n", len(allRecords))
		for _, r := range allRecords {
			fmt.Printf("Session: client_id=%s, auth_code=%s, expires_at=%v\n",
				r.GetString("client_id"),
				r.GetString("auth_code"),
				r.GetDateTime("expires_at").Time(),
			)
		}
	}

	// Now try to find our specific session
	records, err := e.App.FindRecordsByFilter(
		collection,
		"auth_code = {:code} && client_id = {:client_id}",
		"",
		1,
		0,
		dbx.Params{"code": code, "client_id": clientID},
	)
	if err != nil {
		fmt.Printf("Error: Failed to find session - %v\n", err)
		return apis.NewInternalServerError("Failed to find session", err)
	}

	if len(records) == 0 {
		fmt.Printf("Error: No matching session found\n")
		return apis.NewBadRequestError("Invalid authorization code", nil)
	}

	session := newOAuthSessionFromRecord(records[0])
	fmt.Printf("Found matching session:\n")
	fmt.Printf("Client ID: %s\n", session.ClientID)
	fmt.Printf("Auth Code: %s\n", session.AuthCode)
	fmt.Printf("Expires At: %v\n", session.ExpiresAt)

	user, err := e.App.FindRecordById("users", session.UserID)
	if err != nil {
		fmt.Printf("\nError: Invalid user - %v\n", err)
		return apis.NewBadRequestError("Invalid user", nil)
	}

	token, err := user.NewAuthToken()
	if err != nil {
		fmt.Printf("\nError: Failed to generate auth token - %v\n", err)
		return apis.NewBadRequestError("Invalid user", nil)
	}

	fmt.Printf("\nSuccess: Token generated for user %s\n", session.UserID)
	fmt.Printf("=== End OAuth Token Request ===\n\n")

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
