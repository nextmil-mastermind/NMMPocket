package zoomcon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pocketbase/pocketbase"
)

// RegistrationResult stores both the registration response and the associated email
type RegistrationResult struct {
	Response RegisterWebinarResponse
	Email    string
}

// RegisterMembers registers students for a meeting - using the old API
func RegisterMembers(app *pocketbase.PocketBase) error {
	var zt ZOOM_TOKEN
	_, err := zt.GetAccessToken()
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to get access token: %v\n", err)
		return err
	}
	meeting, err := zt.GrabMeetingOccurences()
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to grab meeting occurences: %v\n", err)
		return err
	}
	members, err := getMembers(app)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to get members: %v\n", err)
		return err
	}
	zoomMeeting := os.Getenv("MemberMeeting")
	eRespCh := make(chan RegistrationResult, len(members))
	eErrCh := make(chan error, len(members))

	// Calculate end time once since it's the same for all members
	endTime := meeting.StartTime.Add(time.Duration(meeting.Duration) * time.Minute)

	// Create a slice to store successful registrations
	var successfulRegistrations []RegistrationResult

	for _, member := range members {
		go func(p MemberReduced) {
			Enqueue(RegisterMeetingJob{
				MeetingID:    zoomMeeting,
				OccurrenceID: meeting.OccurrenceId,
				Person:       ZoomPerson{FirstName: p.FirstName, LastName: p.LastName, Email: p.Email, Phone: p.Phone},
				RespCh:       eRespCh,
				ErrCh:        eErrCh,
			})
		}(member)
	}

	// Create a map to store member IDs by email for quick lookup
	memberIDsByEmail := make(map[string]string)
	for _, member := range members {
		memberIDsByEmail[member.Email] = member.ID
	}

	// Collect all responses
	for range members {
		select {
		case resp := <-eRespCh:
			successfulRegistrations = append(successfulRegistrations, resp)
			fmt.Printf("[DEBUG-ZOOM-API] Registered member with ID: %s\n", resp.Response.RegistrantID)

			// Get the member ID from our map
			memberID, exists := memberIDsByEmail[resp.Email]
			if !exists {
				fmt.Printf("[DEBUG-ZOOM-API] Could not find member ID for email: %s\n", resp.Email)
				continue
			}

			// Save the registration in the database
			registration := MemberRegistration{
				MemberID:  memberID,
				JoinURL:   resp.Response.JoinURL,
				StartTime: meeting.StartTime,
				EndTime:   endTime,
				Title:     resp.Response.Topic,
			}
			if err := registration.Create(app); err != nil {
				fmt.Printf("[DEBUG-ZOOM-API] Failed to save registration in database: %v\n", err)
			}
		case err := <-eErrCh:
			fmt.Printf("[DEBUG-ZOOM-API] Failed to register: %v\n", err)
		}
	}
	return nil
}

func (zt ZOOM_TOKEN) GrabMeetingOccurences() (MeetingOccurrence, error) {
	zoom_meeting := os.Getenv("MemberMeeting")
	url := "https://api.zoom.us/v2/meetings/" + zoom_meeting
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[DEBUG-ZOOM-API] Failed to create request: %v\n", err)
		return MeetingOccurrence{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+zt.AccessToken)

	res, err := client.Do(req)
	if err != nil {
		return MeetingOccurrence{}, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return MeetingOccurrence{}, fmt.Errorf("Failed to read response body: %v", err)
	}

	var response MeetingResponse
	//Let's extract the occurrences from the body if 200
	if res.StatusCode == 200 {
		err = json.Unmarshal(body, &response)
		if err != nil {
			return MeetingOccurrence{}, fmt.Errorf("Failed to unmarshal response body: %v", err)
		}
		for _, meeting := range response.Occurrences {
			if meeting.Status == "available" {
				return meeting, nil
			}
		}
	}
	return MeetingOccurrence{}, fmt.Errorf("no occurrences found or error in response: %s", body)
}

func getMembers(app *pocketbase.PocketBase) ([]MemberReduced, error) {
	members, err := app.FindRecordsByFilter("members", "expiration > @monthEnd || group = \"founder\"", "-expiration", 0, 0)
	if err != nil {
		return nil, err
	}
	var membersReduced []MemberReduced
	for _, member := range members {
		var memberReduced MemberReduced
		memberReduced.FromRecord(member)
		membersReduced = append(membersReduced, memberReduced)
	}
	return membersReduced, nil
}
