package zoomcon

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

var (
	queue   = make(chan Job, 10_000)                         // holds RegisterJob, StatusJob, etc.
	limiter = rate.NewLimiter(rate.Every(time.Second/20), 1) // 20‑req/s with 1‑sec burst
	httpC   = &http.Client{Timeout: 10 * time.Second}
)

func Start(ctx context.Context) {
	// Create a background context if none provided
	if ctx == nil {
		fmt.Println("[DEBUG-WORKER] No context provided, using background context")
		ctx = context.Background()
	}

	// Create a new context with cancellation for the worker
	workerCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer func() {
			cancel() // Ensure context is canceled when worker exits
			fmt.Println("[DEBUG-WORKER] Worker goroutine exited")
		}()
		worker(workerCtx)
	}()
}

func worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[DEBUG-WORKER] Worker shutting down due to context cancellation: %v\n", ctx.Err())
			// Process any remaining jobs in the queue before exiting
			for {
				select {
				case job, ok := <-queue:
					if !ok {
						return
					}
					// Create a timeout context for each job
					jobCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					job.Do(jobCtx) // Try to process but don't wait too long
					cancel()
				default:
					return
				}
			}
		case job, ok := <-queue:
			if !ok {
				fmt.Println("[DEBUG-WORKER] Queue closed, worker shutting down")
				return
			}
			// Create a timeout context for each job
			jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if err := limiter.Wait(jobCtx); err != nil {
				fmt.Printf("[DEBUG-WORKER] Rate limiter wait failed: %v\n", err)
				cancel()
				if err == context.Canceled {
					continue // Skip this job if context was canceled
				}
				return
			}

			if err := job.Do(jobCtx); err != nil {
				// If Zoom signalled per‑second overflow → re‑queue after 1 s
				if err.Error() == "rate limit exceeded" {
					fmt.Printf("[DEBUG-WORKER] Zoom rate limit exceeded, requeueing job after 1s: %T\n", job)
					time.Sleep(time.Second)
					Enqueue(job)
					cancel()
					continue
				}
				fmt.Printf("[DEBUG-WORKER] Job failed with error: %v\n", err)
				// TODO: add structured logging / DLQ for permanent failures
			}
			cancel() // Clean up the job context
		}
	}
}

func Enqueue(job Job) {
	select {
	case queue <- job: // fast path
		return
	default: // buffer full
		// fire-and-forget goroutine that *will* block until space is free,
		// but your handler can return 200 immediately.
		go func() {
			queue <- job
		}()
	}
}
