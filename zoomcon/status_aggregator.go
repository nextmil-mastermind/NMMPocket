package zoomcon

import (
	"context"
	"time"
)

var statusIn chan StatusEvent // global channel for status updates

// SetStatusChannel sets the global status channel from main
func SetStatusChannel(ch chan StatusEvent) {
	statusIn = ch
}

// StatusIn returns the global status channel
func StatusIn() chan StatusEvent {
	return statusIn
}

type StatusEvent struct {
	EventID string
	Type    string // "webinar" or "meeting"
	ID      string
	Email   string
}
type statusKey struct {
	EventID   string
	EventType string
}

func StartStatusAggregator(ctx context.Context, in <-chan StatusEvent) {
	const batchSize = 30
	batches := make(map[statusKey][]RegistrantStatus)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop() // Ensure ticker is cleaned up

	for {
		select {
		case <-ctx.Done():
			// Flush any remaining batches before shutting down
			for k, regs := range batches {
				if len(regs) > 0 {
					flushStatus(k, regs)
				}
			}
			return
		case ev := <-in: // new ID to update
			k := statusKey{EventID: ev.EventID, EventType: ev.Type}
			batches[k] = append(batches[k], RegistrantStatus{ID: ev.ID, Email: ev.Email})

			if len(batches[k]) >= batchSize {
				flushStatus(k, batches[k][:batchSize])
				batches[k] = batches[k][batchSize:]
			}

		case <-tick.C: // time-based flush
			for k, regs := range batches {
				if len(regs) > 0 {
					flushStatus(k, regs)
					delete(batches, k)
				}
			}
		}
	}
}

func flushStatus(k statusKey, regs []RegistrantStatus) {
	// queue a single StatusJob; the worker + limiter handle rate-control
	queue <- StatusJob{
		EventID:     k.EventID,
		Type:        k.EventType,
		Registrants: regs,
	}
}
