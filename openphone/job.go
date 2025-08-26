package openphone

import (
	"context"
	"fmt"
)

type Job interface {
	Do(ctx context.Context) error
}

type MessageResponse struct {
	Status string `json:"status"`
}

type MessageJob struct {
	PhoneNumber string
	FromNumber  string
	Content     string
}

func (j MessageJob) Do(ctx context.Context) error {
	resp, err := SendMessage(ctx, j.PhoneNumber, j.FromNumber, j.Content)
	if err != nil {
		fmt.Printf("[DEBUG-JOB] Error sending SMS to %s: %v\n", j.PhoneNumber, err)
		return err
	}

	fmt.Printf("[DEBUG-JOB] SMS sent successfully to %s: %v\n", j.PhoneNumber, resp)
	return nil
}
