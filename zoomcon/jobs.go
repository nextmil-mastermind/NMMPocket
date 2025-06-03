package zoomcon

import (
	"context"
	"fmt"
)

type Job interface {
	Do(ctx context.Context) error
}

type RegisterJob struct {
	WebinarID string
	Person    ZoomPerson
	RespCh    chan RegisterWebinarResponse
	ErrCh     chan error
}

type RegisterMeetingJob struct {
	MeetingID    string
	OccurrenceID string
	Person       ZoomPerson
	RespCh       chan RegistrationResult
	ErrCh        chan error
}

type StatusJob struct {
	EventID     string
	Type        string // "webinar" or "meeting"
	Registrants []RegistrantStatus
}

func (j RegisterJob) Do(ctx context.Context) error {
	resp, err := RegisterWebinar(ctx, j.WebinarID, j.Person)
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case j.ErrCh <- err:
			fmt.Printf("[DEBUG-JOB] Sent error for %s %s\n",
				j.Person.FirstName, j.Person.LastName)
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

func (j StatusJob) Do(ctx context.Context) error {
	// Create a done channel to handle completion
	done := make(chan error, 1)

	go func() {
		done <- UpdateRegistrantStatus(ctx, j.EventID, j.Registrants, &j.Type)
	}()

	// Wait for either context cancellation or job completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			fmt.Printf("[DEBUG-JOB] StatusJob failed for %s %s: %v\n",
				j.Type, j.EventID, err)
			return err
		}
		return nil
	}
}

func (j RegisterMeetingJob) Do(ctx context.Context) error {
	resp, err := RegisterMeeting(ctx, j.MeetingID, j.OccurrenceID, j.Person)
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case j.ErrCh <- err:
			fmt.Printf("[DEBUG-JOB] Sent error for %s %s\n",
				j.Person.FirstName, j.Person.LastName)
			return err
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case j.RespCh <- RegistrationResult{
		Response: resp,
		Email:    j.Person.Email,
	}:
		return nil
	}
}
