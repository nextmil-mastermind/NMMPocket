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
	RespCh      chan MessageResponse
	ErrCh       chan error
}

func (j MessageJob) Do(ctx context.Context) error {
	resp, err := SendMessage(ctx, j.PhoneNumber, j.FromNumber, j.Content)
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case j.ErrCh <- err:
			fmt.Printf("[DEBUG-JOB] Sent error for %s %s\n",
				j.PhoneNumber, j.FromNumber)
			return err
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case j.RespCh <- resp:
		return nil
	}
}
