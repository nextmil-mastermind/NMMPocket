package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"
)

// EmailSender sends an email using the Brevo API.
// Parameters:
//   - to: slice of recipient details (each recipient is a map with keys "email", "first_name", "last_name", etc.)
//   - subject: email subject
//   - message: email body as HTML content
//   - attachment: optional attachment URL
//   - ticketurl: optional ticket URL
//
// Returns an error if the email fails to send.
func EmailSender(to []Recipient, subject, message string, attachment *[]BrevoAttachment) error {

	// Build messageVersions.
	var messageVersions []MessageVersion
	for _, r := range to {
		params := map[string]any{
			"name":       r.Name,
			"email":      r.Email,
			"first_name": r.FirstName,
		}

		if r.Params != nil {
			maps.Copy(params, *r.Params)
		}
		mv := MessageVersion{
			To:     []Contact{{Name: r.Name, Email: r.Email}},
			Params: params,
		}
		if len(r.CC) > 0 {
			mv.CC = r.CC
		}
		messageVersions = append(messageVersions, mv)
	}

	// Build the complete payload.
	payload := EmailData{
		Sender: Contact{
			Name:  os.Getenv("SENDER_NAME"),
			Email: os.Getenv("SENDER_EMAIL"),
		},
		ReplyTo: Contact{
			Name:  os.Getenv("REPLY_NAME"),
			Email: os.Getenv("REPLY_EMAIL"),
		},
		Subject:         subject,
		HTMLContent:     message,
		MessageVersions: messageVersions,
	}
	if attachment != nil {
		payload.Attachment = attachment
	}

	// Marshal payload to JSON.
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	err = BrevoRequest(b)
	if err != nil {
		return fmt.Errorf("brevo request: %w", err)
	}

	return nil
}

// BrevoRequest Requires a payload of type []byte and returns an error.
func BrevoRequest(payload []byte) error {
	// Prepare HTTP request.
	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", os.Getenv("BREVO_API_KEY"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// If response is not OK, read the error body.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// from string to time.RFC3339
func sendTimeConvert(timestring *string) *string {
	if timestring == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *timestring)
	if err != nil {
		fmt.Println("Error parsing time:", err)
		return nil
	}
	tStr := t.Format(time.RFC3339)
	return &tStr
}
