package zoomcon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ZOOM_ACCESS_TOKEN struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
type ZOOM_TOKEN struct {
	AccessToken string
	Expires     time.Time
}

/*
GetAccessToken gets the access token for the Zoom API.
Returns: The access token for the Zoom API.
Error: An error if the access token retrieval fails.
*/
func (zt *ZOOM_TOKEN) GetAccessToken() (string, error) {
	if zt.Expires.Before(time.Now()) {
		var accessToken ZOOM_ACCESS_TOKEN
		url := "https://zoom.us/oauth/token?grant_type=account_credentials&account_id=" + os.Getenv("ZOOM_ID")
		method := "POST"

		client := &http.Client{}
		req, err := http.NewRequest(method, url, nil)

		if err != nil {
			fmt.Println(err)
			return "", err
		}
		req.Header.Add("Authorization", "Basic "+os.Getenv("ZOOM_CLIENT"))

		res, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
		json.Unmarshal(body, &accessToken)
		zt.AccessToken = accessToken.AccessToken
		zt.Expires = time.Now().Add(time.Duration(accessToken.ExpiresIn) * time.Second)
		return zt.AccessToken, nil
	}
	return zt.AccessToken, nil
}
