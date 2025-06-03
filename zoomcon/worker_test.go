package zoomcon

import (
	"context"
	"slices"
	"testing"
	"time"
)

// dummyJob implements the Job interface and records the timestamp
// of every execution so we can verify the rate‑limiter.
type dummyJob struct {
	tsChan chan time.Time
}

func (d dummyJob) Do(ctx context.Context) error {
	d.tsChan <- time.Now()
	return nil
}

func TestLimiter20RPS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Channel to collect timestamps from executed jobs.
	hits := make(chan time.Time, 1000)

	// Start the worker(s) – uses the shared limiter inside the package.
	Start(ctx) // launches worker goroutine

	// Jam 100 jobs into the queue immediately.
	for i := 0; i < 100; i++ {
		queue <- dummyJob{tsChan: hits}
	}

	<-ctx.Done() // wait until context times out (5 s)
	close(hits)

	// Build a histogram of requests per 1‑second bucket.
	buckets := map[int64]int{}
	for ts := range hits {
		b := ts.Unix()
		buckets[b]++
	}

	// Sort and print requests per second for easier debugging.
	var seconds []int64
	for sec := range buckets {
		seconds = append(seconds, sec)
	}
	slices.Sort(seconds)

	for _, sec := range seconds {
		t.Logf("Second %d: %d requests", sec-seconds[0], buckets[sec])
	}

	// Assert no bucket exceeds 20 requests.
	for sec, n := range buckets {
		if n > 20 {
			t.Fatalf("bucket %d had %d (>20) calls", sec, n)
		}
	}
}
