package latency

import (
	"context"
	"net"
	"sync"
	"time"

	"fivi/internal/parser"
)

// ScanResult holds a candidate alongside its latency scan outcome.
type ScanResult struct {
	Candidate parser.Candidate
	Addr      string
	Latency   time.Duration
	Rejected  bool // true if failed or above cutoff
}

// ScanBatch performs parallel TCP connect latency checks on candidates and
// returns only those within the cutoff. Candidates whose server address cannot
// be extracted are kept (passed through) to avoid rejecting parseable configs
// that have unusual URI formats.
func ScanBatch(ctx context.Context, candidates []parser.Candidate, dialTimeout, cutoff time.Duration, parallelism int) (passed []parser.Candidate, rejected int) {
	passed, rejected, _ = ScanBatchWithLatency(ctx, candidates, dialTimeout, cutoff, parallelism)
	return passed, rejected
}

// ScanBatchWithLatency performs ScanBatch and also returns a per-candidate
// latency map (raw URI -> connect latency) for passed candidates where latency
// measurement was available.
func ScanBatchWithLatency(ctx context.Context, candidates []parser.Candidate, dialTimeout, cutoff time.Duration, parallelism int) (passed []parser.Candidate, rejected int, latencies map[string]time.Duration) {
	if len(candidates) == 0 {
		return nil, 0, map[string]time.Duration{}
	}
	if parallelism < 1 {
		parallelism = 50
	}

	results := make([]ScanResult, len(candidates))
	jobs := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case idx, ok := <-jobs:
					if !ok {
						return
					}
					c := candidates[idx]
					sr := scanOne(ctx, c, dialTimeout, cutoff)
					results[idx] = sr
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i := range candidates {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()

	wg.Wait()

	latencies = make(map[string]time.Duration, len(candidates))
	passed = make([]parser.Candidate, 0, len(candidates))
	for i, sr := range results {
		if sr.Rejected {
			rejected++
			continue
		}
		passed = append(passed, candidates[i])
		if sr.Latency > 0 {
			latencies[candidates[i].Raw] = sr.Latency
		}
	}
	return passed, rejected, latencies
}

func scanOne(ctx context.Context, c parser.Candidate, dialTimeout, cutoff time.Duration) ScanResult {
	addr, err := ExtractServerAddr(c.Raw)
	if err != nil {
		// Cannot extract address — pass through rather than reject.
		return ScanResult{Candidate: c, Rejected: false}
	}

	start := time.Now()
	conn, err := (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, "tcp", addr)
	elapsed := time.Since(start)

	if err != nil {
		return ScanResult{Candidate: c, Addr: addr, Rejected: true}
	}
	_ = conn.Close()

	if elapsed > cutoff {
		return ScanResult{Candidate: c, Addr: addr, Latency: elapsed, Rejected: true}
	}
	return ScanResult{Candidate: c, Addr: addr, Latency: elapsed, Rejected: false}
}
