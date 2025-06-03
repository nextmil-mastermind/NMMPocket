package zoomcon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type RegisterWebinarResponse struct {
	RegistrantID string `json:"registrant_id"`
	ID           int    `json:"id"`
	Topic        string `json:"topic"`
	StartTime    string `json:"start_time"`
	JoinURL      string `json:"join_url"`
}
type ZoomPerson struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

const (
	ZOOM_URL_BASE = "https://api.zoom.us/v2/webinars/"
)

/*
RegisterWebinar registers a person to a webinar.
webinarID: The ID of the webinar to register the person to.
person: The person to register to the webinar.
Returns: The response from the webinar registration.
Error: An error if the webinar registration fails.
*/
func RegisterWebinar(ctx context.Context, webinarID string, person ZoomPerson) (RegisterWebinarResponse, error) {
	var zt ZOOM_TOKEN
	token, err := zt.GetAccessToken()
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to get access token: %v\n", err)
		return RegisterWebinarResponse{}, err
	}

	url := ZOOM_URL_BASE + webinarID + "/registrants"

	payload := strings.NewReader(fmt.Sprintf(`{
        "first_name": "%s",
        "last_name":  "%s",
        "email":      "%s",
        "phone":      "%s"
    }`, person.FirstName, person.LastName, person.Email, person.Phone))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to create request: %v\n", err)
		return RegisterWebinarResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := httpC.Do(req)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] HTTP request failed: %v\n", err)
		return RegisterWebinarResponse{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to read response body: %v\n", err)
		return RegisterWebinarResponse{}, err
	}

	switch res.StatusCode {
	case http.StatusCreated: // 201
		var resp RegisterWebinarResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			fmt.Printf("[DEBUG-ZOOM-API] Failed to parse success response: %v\n", err)
			return RegisterWebinarResponse{}, err
		}
		return resp, nil
	case http.StatusTooManyRequests: // 429
		return RegisterWebinarResponse{}, errors.New("rate limit exceeded")
	default:
		return RegisterWebinarResponse{}, fmt.Errorf("zoom returned %d: %s", res.StatusCode, body)
	}
}

// RegistrantStatus represents a single registrant (ID + Email) used when
// batching status updates to Zoom.
type RegistrantStatus struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UpdateRegistrantStatusRequest struct {
	Action      string             `json:"action"`
	Registrants []RegistrantStatus `json:"registrants"`
}

/*
UpdateRegistrantStatus updates the status of a registrant to a webinar.
webinarID: The ID of the webinar to update the registrant status of.
registrants: The registrants to update the status of.
Returns: An error if the registrant status update fails.
*/
func UpdateRegistrantStatus(ctx context.Context, webinarID string, registrants []RegistrantStatus) error {
	var zt ZOOM_TOKEN
	token, err := zt.GetAccessToken()
	if err != nil {
		fmt.Println(err)
		return err
	}
	var request UpdateRegistrantStatusRequest
	request.Action = "approve"

	request.Registrants = registrants

	url := ZOOM_URL_BASE + webinarID + "/registrants/status"
	method := "PUT"

	payload, err := json.Marshal(request)
	if err != nil {
		fmt.Println(err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println(err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	res, err := httpC.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer res.Body.Close()

	_, err = io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if res.StatusCode != 204 {
		return errors.New("failed to update registrant status")
	}
	return nil
}
