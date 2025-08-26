package openphone

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

var (
	queue   = make(chan Job, 500)                           // holds RegisterJob, StatusJob, etc.
	limiter = rate.NewLimiter(rate.Every(time.Second/3), 1) // 3 req/s with 1 burst to be more conservative
	httpC   = &http.Client{Timeout: 10 * time.Second}
	APIKey  = "your_api_key_here"
)

func Start(ctx context.Context) {
	// Create a background context if none provided
	if ctx == nil {
		fmt.Println("[Phone-WORKER] No context provided, using background context")
		ctx = context.Background()
	}

	// Create a new context with cancellation for the worker
	workerCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer func() {
			cancel() // Ensure context is canceled when worker exits
			fmt.Println("[Phone-WORKER] Worker goroutine exited")
		}()
		worker(workerCtx)
	}()
	APIKey = os.Getenv("OPENPHONE_API_KEY")
}

func worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[Phone-WORKER] Worker shutting down due to context cancellation: %v\n", ctx.Err())
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
				fmt.Println("[Phone-WORKER] Queue closed, worker shutting down")
				return
			}
			// Create a timeout context for each job
			jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if err := limiter.Wait(jobCtx); err != nil {
				fmt.Printf("[Phone-WORKER] Rate limiter wait failed: %v\n", err)
				cancel()
				if err == context.Canceled {
					continue // Skip this job if context was canceled
				}
				return
			}

			if err := job.Do(jobCtx); err != nil {
				// If OpenPhone signalled rate limit → re‑queue after delay
				if strings.Contains(err.Error(), "rate limit exceeded") {
					fmt.Printf("[Phone-WORKER] OpenPhone rate limit exceeded, requeueing job after 2s: %T\n", job)
					cancel()
					// Wait before requeueing to respect rate limits
					select {
					case <-time.After(2 * time.Second):
						Enqueue(job)
						fmt.Printf("[Phone-WORKER] Job requeued due to rate limit\n")
					case <-ctx.Done():
						fmt.Printf("[Phone-WORKER] Context canceled while waiting to requeue job\n")
						return
					}
					continue
				}

				// Handle context deadline exceeded (timeout)
				if strings.Contains(err.Error(), "context deadline exceeded") {
					fmt.Printf("[Phone-WORKER] Job timed out, requeueing: %T\n", job)
					// Requeue timeout jobs as well, but with a delay
					select {
					case <-time.After(5 * time.Second):
						Enqueue(job)
						fmt.Printf("[Phone-WORKER] Job requeued due to timeout\n")
					case <-ctx.Done():
						fmt.Printf("[Phone-WORKER] Context canceled while waiting to requeue timeout job\n")
						return
					}
					continue
				} else {
					fmt.Printf("[Phone-WORKER] Job failed with permanent error: %v\n", err)
					// Don't requeue permanent failures
				}
			} else {
				fmt.Printf("[Phone-WORKER] Job completed successfully\n")
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
