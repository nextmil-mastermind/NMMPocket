package passkeys

import (
	"encoding/base64"
	"io"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// RegisterRoutes registers the routes for the webauthn authentication
func RegisterRoutes(se *core.ServeEvent, app *pocketbase.PocketBase, webAuthn *webauthn.WebAuthn, webAuthnSessions map[string]*webauthn.SessionData) {

	se.Router.POST("/webauth/register/{collection}/{userb64}", func(e *core.RequestEvent) error {
		collection := e.Request.PathValue("collection")
		usernameb64 := e.Request.PathValue("userb64")
		email, err := base64.StdEncoding.DecodeString(usernameb64)
		//username must be the same as the authRecord username
		authRecord := e.Auth
		if authRecord.GetString("email") != string(email) {
			return apis.NewBadRequestError("Invalid username.", nil)
		}
		if err != nil {
			return apis.NewBadRequestError("Invalid username.", err)
		}
		user, err := FindUser(app, string(email), collection)
		if err != nil {
			return apis.NewNotFoundError("User not found.", err)
		}
		// start registration flow
		options, sessionData, err := webAuthn.BeginRegistration(user)
		if err != nil {
			return apis.NewBadRequestError("Failed to start registration flow.", err)
		}
		// store session data in the map
		webAuthnSessions[user.WebAuthnIdB64] = sessionData

		// send the challenge to the client
		return e.JSON(http.StatusOK, options)
	}).Bind(apis.RequireAuth())

	se.Router.POST("/webauth/register/{collection}/{userb64}/finish", func(e *core.RequestEvent) error {
		collection := e.Request.PathValue("collection")
		usernameb64 := e.Request.PathValue("userb64")
		email, err := base64.StdEncoding.DecodeString(usernameb64)
		if err != nil {
			return apis.NewBadRequestError("Invalid email.", err)
		}
		user, err := FindUser(app, string(email), collection)
		if err != nil {
			return apis.NewNotFoundError("User not found.", err)
		}
		// get the session data from the map
		sessionData := webAuthnSessions[user.WebAuthnIdB64]
		if sessionData == nil {
			return apis.NewBadRequestError("Invalid session data.", nil)
		}
		io.ReadAll(e.Request.Body)

		newCredential, err := webAuthn.FinishRegistration(user, *sessionData, e.Request)
		if err != nil {
			return apis.NewBadRequestError("Failed to finish registration flow.", err)
		}
		//extract device_name from the requestbody which is json encoded
		info, err := e.RequestInfo()
		if err != nil {
			return apis.NewBadRequestError("Failed to get request info.", err)
		}
		//check if the request has a query param called device_name
		device_name := info.Query["device_name"]
		if device_name == "" {
			device_name = "unknown"
		} else {
			device_nameb, err := base64.StdEncoding.DecodeString(device_name)
			if err != nil {
				return apis.NewBadRequestError("Invalid email.", err)
			}
			device_name = string(device_nameb)
		}

		// add the new credential to the user's stored credentials
		err = user.AddWebAuthnCredential(app, collection, *newCredential, device_name)
		if err != nil {
			return apis.NewBadRequestError("Failed to store new credential.", err)
		}
		// remove session data from the map
		delete(webAuthnSessions, user.WebAuthnIdB64)
		// return success with an authentication token
		return user.SendAuthTokenResponse(collection, app, e)
	})

	se.Router.POST("/webauth/login/{collection}/{userb64}", func(e *core.RequestEvent) error {
		collection := e.Request.PathValue("collection")
		usernameb64 := e.Request.PathValue("userb64")
		email, err := base64.StdEncoding.DecodeString(usernameb64)
		if err != nil {
			return apis.NewBadRequestError("Invalid username.", err)
		}
		user, err := FindUser(app, string(email), collection)
		if err != nil {
			return apis.NewNotFoundError("User not found.", err)
		}
		// start login flow
		options, sessionData, err := webAuthn.BeginLogin(user)
		if err != nil {
			return apis.NewBadRequestError("Failed to start login flow.", err)
		}
		// store session data in the map
		webAuthnSessions[user.WebAuthnIdB64] = sessionData
		// send the challenge to the client
		return e.JSON(http.StatusOK, options)
	})

	se.Router.POST("/webauth/login/{collection}/{userb64}/finish", func(e *core.RequestEvent) error {
		collection := e.Request.PathValue("collection")
		usernameb64 := e.Request.PathValue("userb64")
		email, err := base64.StdEncoding.DecodeString(usernameb64)
		if err != nil {
			return apis.NewBadRequestError("Invalid username.", err)
		}
		user, err := FindUser(app, string(email), collection)
		if err != nil {
			return apis.NewNotFoundError("User not found.", err)
		}
		// get the session data from the map
		sessionData := webAuthnSessions[user.WebAuthnIdB64]
		if sessionData == nil {
			return apis.NewBadRequestError("Invalid session data.", nil)
		}
		io.ReadAll(e.Request.Body)
		_, err = webAuthn.FinishLogin(user, *sessionData, e.Request)
		if err != nil {
			return apis.NewBadRequestError("Failed to finish login flow.", err)
		}
		// remove session data from the map
		delete(webAuthnSessions, user.WebAuthnIdB64)
		// return success
		return user.SendAuthTokenResponse(collection, app, e)
	})
	se.Router.DELETE("/webauthn/credential/{collection}/{userb64}/{credentialid}", func(e *core.RequestEvent) error {
		collection := e.Request.PathValue("collection")
		usernameb64 := e.Request.PathValue("userb64")
		email, err := base64.StdEncoding.DecodeString(usernameb64)
		if err != nil {
			return apis.NewBadRequestError("Invalid username.", err)
		}
		user, err := FindUser(app, string(email), collection)
		if err != nil {
			return apis.NewNotFoundError("User not found.", err)
		}
		// delete the credential
		credentialid := e.Request.PathValue("credentialid")
		err = user.DeleteWebAuthnCredential(app, collection, credentialid)
		if err != nil {
			return apis.NewBadRequestError("Failed to delete credential.", err)
		}
		// return success
		return e.JSON(http.StatusOK, nil)
	})
}
