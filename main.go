package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"nmmpocket/appform"
	"nmmpocket/authentication"
	"nmmpocket/lib"
	"nmmpocket/openphone"
	"nmmpocket/zoomcon"
	"os"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/stripe/stripe-go/v81"
)

var (
	statusIn = make(chan zoomcon.StatusEvent, 10_000) // global so any code can push
	appCtx   context.Context
	cancel   context.CancelFunc
)

func main() {
	//check if env of is_prod is set or is set to true
	//if not set to true, load the .env file
	isProd := os.Getenv("is_prod")
	if isProd == "" || isProd == "false" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}
	authentication.Init()
	rpOriginsEnv := os.Getenv("origins")
	rpOrigins := strings.Split(rpOriginsEnv, ",")

	// configure and initialize webauthn
	wconfig := &webauthn.Config{
		RPDisplayName: os.Getenv("appname"), // Display Name for your site
		RPID:          os.Getenv("fqdn"),    // Generally the FQDN for your site
		RPOrigins:     rpOrigins,            // array of origins from where the webapp is served
	}

	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		fmt.Println(err)
		return
	}

	// create a map holding the sessions used during registration and login flow
	webAuthnSessions := make(map[string]*webauthn.SessionData)
	stripe.Key = os.Getenv("STRIPE")
	lib.InitDB()
	app := pocketbase.New()
	appCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Initialize Zoom components before the server starts
	zoomcon.SetStatusChannel(statusIn)
	zoomcon.Start(appCtx) // Start the worker with the application context
	go zoomcon.StartStatusAggregator(appCtx, statusIn)

	openphone.Start(appCtx)

	app.Cron().MustAdd("check_invoice", "0 11 * * *", func() { lib.CheckInvoice(app) })
	app.Cron().Add("schedule_check", "0,30 * * * *", func() { lib.ScheduleCheck(app) })
	app.Cron().MustAdd("student_zoom_reg", "0 12 * * 1", func() {
		now := time.Now()
		if !isFourthMonday(now) {
			return // Ignore Sundays, 1st/2nd/3rd/5th Mondays, etc.
		}
		err := zoomcon.RegisterMembers(app)
		if err != nil {
			app.Logger().Error("Failed to register members for Zoom meeting", "error", err)
		}
	})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		lib.RegisterStripeWebhook(se.Router, app)
		authentication.RegisterOAuthRoutes(se.Router)

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
			user, err := lib.FindUser(app, string(email), collection)
			if err != nil {
				return apis.NewNotFoundError("User not found.", err)
			}
			// start the registration flow
			options, sessionData, err := webAuthn.BeginRegistration(user,
				webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
				webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
					UserVerification: protocol.VerificationPreferred,
				}))
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
			user, err := lib.FindUser(app, string(email), collection)
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
			user, err := lib.FindUser(app, string(email), collection)
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
			user, err := lib.FindUser(app, string(email), collection)
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
			user, err := lib.FindUser(app, string(email), collection)
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
		//receive submissions from the appform
		se.Router.POST("/appform/submission", func(e *core.RequestEvent) error {
			return appform.ReceivedSubmissionRoute(app, e)
		})
		se.Router.POST("/appform/submission/small", func(e *core.RequestEvent) error {
			return appform.ReceivedSmallSubmissionRoute(app, e)
		})
		se.Router.POST("/invoice/autopay/force", lib.InvoiceAutopayForceRoute).Bind(apis.RequireAuth())
		authentication.Routes(se.Router)
		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// isFourthMonday returns true if it is a Monday and its day is 22–28.
func isFourthMonday(t time.Time) bool {
	return t.Weekday() == time.Monday && t.Day() >= 22 && t.Day() <= 28
}
