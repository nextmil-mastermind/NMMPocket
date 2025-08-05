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
	ZOOM_URL_BASE = "https://api.zoom.us/v2/"
)

// ZoomAPIResponse represents a generic API response from Zoom
type ZoomAPIResponse struct {
	StatusCode int
	Body       []byte
}

// makeZoomRequest abstracts the common HTTP request logic for Zoom API calls
func makeZoomRequest(ctx context.Context, method, url string, payload io.Reader) (*ZoomAPIResponse, error) {
	var zt ZOOM_TOKEN
	token, err := zt.GetAccessToken()
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to get access token: %v\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to create request: %v\n", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := httpC.Do(req)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] HTTP request failed: %v\n", err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to read response body: %v\n", err)
		return nil, err
	}

	return &ZoomAPIResponse{
		StatusCode: res.StatusCode,
		Body:       body,
	}, nil
}

/*
RegisterWebinar registers a person to a webinar.
webinarID: The ID of the webinar to register the person to.
person: The person to register to the webinar.
Returns: The response from the webinar registration.
Error: An error if the webinar registration fails.
*/
func RegisterWebinar(ctx context.Context, webinarID string, person ZoomPerson) (RegisterWebinarResponse, error) {
	url := ZOOM_URL_BASE + "webinars/" + webinarID + "/registrants"

	payload := strings.NewReader(fmt.Sprintf(`{
        "first_name": "%s",
        "last_name":  "%s",
        "email":      "%s",
        "phone":      "%s"
    }`, person.FirstName, person.LastName, person.Email, person.Phone))

	response, err := makeZoomRequest(ctx, http.MethodPost, url, payload)
	if err != nil {
		return RegisterWebinarResponse{}, err
	}

	switch response.StatusCode {
	case http.StatusCreated: // 201
		var resp RegisterWebinarResponse
		if err := json.Unmarshal(response.Body, &resp); err != nil {
			fmt.Printf("[DEBUG-ZOOM-API] Failed to parse success response: %v\n", err)
			return RegisterWebinarResponse{}, err
		}
		return resp, nil
	case http.StatusTooManyRequests: // 429
		return RegisterWebinarResponse{}, errors.New("rate limit exceeded")
	default:
		return RegisterWebinarResponse{}, fmt.Errorf("zoom returned %d: %s", response.StatusCode, response.Body)
	}
}

func RegisterMeeting(ctx context.Context, meetingID string, occurrenceID string, person ZoomPerson) (RegisterWebinarResponse, error) {
	url := ZOOM_URL_BASE + "meetings/" + meetingID + "/registrants?occurrence_ids=" + occurrenceID

	payload := strings.NewReader(fmt.Sprintf(`{
        "first_name": "%s",
        "last_name":  "%s",
        "email":      "%s",
        "phone":      "%s",
		"auto_approve": true
    }`, person.FirstName, person.LastName, person.Email, person.Phone))

	response, err := makeZoomRequest(ctx, http.MethodPost, url, payload)
	if err != nil {
		return RegisterWebinarResponse{}, err
	}

	switch response.StatusCode {
	case http.StatusCreated: // 201
		var resp RegisterWebinarResponse
		if err := json.Unmarshal(response.Body, &resp); err != nil {
			fmt.Printf("[DEBUG-ZOOM-API] Failed to parse success response: %v\n", err)
			return RegisterWebinarResponse{}, err
		}
		return resp, nil
	case http.StatusTooManyRequests: // 429
		return RegisterWebinarResponse{}, errors.New("rate limit exceeded")
	default:
		return RegisterWebinarResponse{}, fmt.Errorf("zoom returned %d: %s", response.StatusCode, response.Body)
	}
}

/*
UpdateRegistrantStatus updates the status of a registrant to a webinar.
webinarID: The ID of the webinar to update the registrant status of.
registrants: The registrants to update the status of.
Returns: An error if the registrant status update fails.
*/
func UpdateRegistrantStatus(ctx context.Context, eventID string, registrants []RegistrantStatus, registrantType *string) error {
	var registrantTypeString string
	if registrantType == nil {
		registrantTypeString = "webinars"
	} else {
		registrantTypeString = *registrantType
	}

	var request UpdateRegistrantStatusRequest
	request.Action = "approve"
	request.Registrants = registrants

	url := ZOOM_URL_BASE + registrantTypeString + "/" + eventID + "/registrants/status"

	payload, err := json.Marshal(request)
	if err != nil {
		fmt.Println(err)
		return err
	}

	response, err := makeZoomRequest(ctx, http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	if response.StatusCode != 204 {
		return errors.New("failed to update registrant status")
	}
	return nil
}

func (zt *ZOOM_TOKEN) GrabSingleOccurrence(meetingID, occurrenceID int64) (Meeting, error) {
	url := fmt.Sprintf("https://api.zoom.us/v2/meetings/%d?occurrence_id=%d", meetingID, occurrenceID)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to create request: %v\n", err)
		return Meeting{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+zt.AccessToken)

	res, err := client.Do(req)
	if err != nil {
		return Meeting{}, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Meeting{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var occurrence Meeting
	if res.StatusCode == 200 {
		err = json.Unmarshal(body, &occurrence)
		if err != nil {
			return Meeting{}, fmt.Errorf("failed to unmarshal response body: %v", err)
		}
		return occurrence, nil
	}
	return Meeting{}, fmt.Errorf("error in response: %s", body)
}
