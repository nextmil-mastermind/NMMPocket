package openphone

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type APIResponse struct {
	StatusCode int
	Body       []byte
}

func SendMessage(ctx context.Context, phoneNumber, fromNumber, content string) (MessageResponse, error) {
	//lets make sure that the phoneNumber, can be made to conform to +1, for example if the user has 8138194188 or 813-819-4188 or (813)819-4188 or 813.819.4188 some might be already +13055555555
	formattedPhone := formatPhoneNumber(phoneNumber)

	payload := fmt.Sprintf(`{
		"to": ["%s"],
		"from": "%s",
		"content": "%s",
		"setInboxStatus": "done"
	}`, formattedPhone, fromNumber, content)

	resp, err := makeRequest(ctx, http.MethodPost, "/messages", io.NopCloser(strings.NewReader(payload)))
	if err != nil {
		return MessageResponse{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MessageResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var messageResp struct {
		Data MessageResponse `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &messageResp); err != nil {
		return MessageResponse{}, err
	}

	return messageResp.Data, nil
}

func makeRequest(ctx context.Context, method, url string, payload io.Reader) (*APIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method, "https://api.openphone.com/v1"+url, payload)
	if err != nil {
		fmt.Printf("[DEBUG-OpenPhone-API] Failed to create request: %v\n", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", APIKey)

	res, err := httpC.Do(req)
	if err != nil {
		fmt.Printf("[DEBUG-OpenPhone-API] HTTP request failed: %v\n", err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("[DEBUG-OpenPhone-API] Failed to read response body: %v\n", err)
		return nil, err
	}

	return &APIResponse{
		StatusCode: res.StatusCode,
		Body:       body,
	}, nil
}

func formatPhoneNumber(phone string) string {
	// Remove all non-numeric characters
	re := regexp.MustCompile(`\D`)
	cleaned := re.ReplaceAllString(phone, "")

	// If the number is already in E.164 format, return it
	if strings.HasPrefix(cleaned, "+") {
		return cleaned
	}
	// If the number starts with '1' and is 11 digits long, format it as +1XXXXXXXXXX
	if strings.HasPrefix(cleaned, "1") && len(cleaned) == 11 {
		return fmt.Sprintf("+%s", cleaned)
	}
	// Otherwise, format it as +1XXXXXXXXXX
	return fmt.Sprintf("+1%s", cleaned)
}
