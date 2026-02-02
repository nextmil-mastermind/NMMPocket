package ghl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// doRequest is a helper method that performs the HTTP request with common logic
func (req *GoHighLevelRequest) doRequest(method, endpoint string, data, result any) error {
	url := "https://services.leadconnectorhq.com" + endpoint

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	httpReq, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Add("Version", "2021-07-28")
	httpReq.Header.Add("Authorization", "Bearer "+req.AccessToken)
	if data != nil {
		httpReq.Header.Add("Content-Type", "application/json")
	}

	client := &http.Client{}
	res, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %d: %s", res.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// Public methods for different HTTP verbs
func (req *GoHighLevelRequest) POST(EndPoint string, Data, result any) error {
	return req.doRequest("POST", EndPoint, Data, result)
}

func (req *GoHighLevelRequest) GET(EndPoint string, result any) error {
	return req.doRequest("GET", EndPoint, nil, result)
}

func (req *GoHighLevelRequest) Delete(EndPoint string, Data, result any) error {
	return req.doRequest("DELETE", EndPoint, Data, result)
}

func (req *GoHighLevelRequest) Put(EndPoint string, Data, result any) error {
	return req.doRequest("PUT", EndPoint, Data, result)
}

// SearchContacts searches for contacts by email and returns a list of matching contacts
func (req *GoHighLevelRequest) SearchContacts(Email string) ([]map[string]any, error) {
	var result struct {
		Contacts []map[string]any `json:"contacts"`
	}
	var searchParams = map[string]any{
		"locationId": req.LocationId,
		"page":       1,
		"pageLimit":  20,
		"query":      Email,
	}

	err := req.POST("/contacts/search", searchParams, &result)
	if err != nil {
		return nil, err
	}
	return result.Contacts, nil
}

//API Actions

/*
UpsertContact upserts a contact in GoHighLevel and returns the contact data

	Note: ContactData must contain at least "firstName", "lastName", "email", and "phone" fields
	Should not have any tags in the ContactData; use AddTags method after upserting if needed as if you do it will replace existing tags.
*/
func (req *GoHighLevelRequest) UpsertContact(ContactData map[string]any) (map[string]any, error) {
	var result map[string]any
	ContactData["locationId"] = req.LocationId
	ContactData["createNewIfDuplicateAllowed"] = false
	err := req.POST("/contacts/upsert", ContactData, &result)
	if err != nil {
		return nil, err
	}
	if result["succeeded"] == false {
		return nil, fmt.Errorf("failed to upsert contact: %v", result["message"])
	}
	return result["contact"].(map[string]any), nil
}

// AddTags adds tags to a contact
func (req *GoHighLevelRequest) AddTags(ContactID string, Tags []string) error {
	var result map[string]any
	var tagData = map[string]any{
		"tags": Tags,
	}
	err := req.POST("/contacts/"+ContactID+"/tags", tagData, &result)
	if err != nil {
		return err
	}
	return nil
}

func (req *GoHighLevelRequest) RemoveTags(ContactID string, Tags []string) error {
	var result map[string]any
	var tagData = map[string]any{
		"tags": Tags,
	}
	err := req.Delete("/contacts/"+ContactID+"/tags", tagData, &result)
	if err != nil {
		return err
	}
	return nil
}

/*
Temp Deprecation due to limitatioins on GHL ability to use custom objects in Workflows
func (req *GoHighLevelRequest) CreateTicketRecord(EventId string, TicketId string, URL string) (string, error) {
	var result map[string]any
	var ticketData = map[string]any{
		"locationId": req.LocationId,
		"properties": map[string]any{
			"event_id":  EventId,
			"url":       URL,
			"ticket_id": TicketId,
		},
	}
	err := req.POST("/objects/"+req.TicketObjectId+"/records", ticketData, &result)
	if err != nil {
		return "", err
	}
	if result["record"] == nil {
		return "", fmt.Errorf("failed to create ticket record: %v", result)
	}
	return result["record"].(map[string]any)["id"].(string), nil
}

func (req *GoHighLevelRequest) AssignTicketToContact(ContactID string, TicketId string) error {
	var result map[string]any
	var assignData = map[string]any{
		"locationId":     req.LocationId,
		"associationId":  req.TicketAssocId,
		"firstRecordId":  ContactID,
		"secondRecordId": TicketId,
	}
	err := req.POST("/associations/relations", assignData, &result)
	if err != nil {
		return err
	}
	return nil
}*/
