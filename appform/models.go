package appform

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type Application struct {
	ID           *string `json:"id,omitempty"`
	ReferredBy   *string `json:"referred_by,omitempty"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	EmailAddress string  `json:"email_address"`
	Phone        string  `json:"phone"`
	Company      *string `json:"company,omitempty"`
	Website      *string `json:"website,omitempty"`
	Address      string  `json:"address"`
	City         string  `json:"city"`
	State        string  `json:"state"`
	Zip          string  `json:"zip"`
	Message      *string `json:"message,omitempty"`
	Terms        bool    `json:"terms"`
	Human        *bool   `json:"human,omitempty"`
}

type ApplicationSubmission struct {
	Application
	Turnstile string `json:"turnstile"`
}

type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
}

func (as *ApplicationSubmission) VerifyTurnstile(secret string) (bool, error) {
	form := url.Values{}
	form.Add("secret", secret)
	form.Add("response", as.Turnstile)

	resp, err := http.Post(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result TurnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.Success, nil
}
