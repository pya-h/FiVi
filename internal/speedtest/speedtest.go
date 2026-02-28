package speedtest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	DefaultDownloadURL  = "https://speed.cloudflare.com/__down"
	DefaultUploadURL    = "https://speed.cloudflare.com/__up"
	DefaultLatencyURL   = "https://www.gstatic.com/generate_204"
	DefaultLatencyPings = 10
	DefaultTimeout      = 60 * time.Second
)

// DownloadSizes are the byte counts to request from the download endpoint.
var DownloadSizes = []int64{
	100 * 1024,       // 100 KB
	1024 * 1024,      // 1 MB
	10 * 1024 * 1024, // 10 MB
}

// UploadSizes are the byte counts for upload POST payloads.
var UploadSizes = []int64{
	100 * 1024,  // 100 KB
	500 * 1024,  // 500 KB
	1024 * 1024, // 1 MB
}

// Config holds speed test settings.
type Config struct {
	SocksAddr    string
	DownloadURL  string
	UploadURL    string
	LatencyURL   string
	Timeout      time.Duration
	LatencyPings int
	Log          func(format string, args ...any) // optional progress callback
}

func (c *Config) applyDefaults() {
	if c.DownloadURL == "" {
		c.DownloadURL = DefaultDownloadURL
	}
	if c.UploadURL == "" {
		c.UploadURL = DefaultUploadURL
	}
	if c.LatencyURL == "" {
		c.LatencyURL = DefaultLatencyURL
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.LatencyPings <= 0 {
		c.LatencyPings = DefaultLatencyPings
	}
}

func (c *Config) log(format string, args ...any) {
	if c.Log != nil {
		c.Log(format, args...)
	}
}

// SizeResult holds the result of a single download or upload trial.
type SizeResult struct {
	Size      int64
	SpeedMbps float64
	Duration  time.Duration
	Err       error
}

// DownloadResult holds download speed test results.
type DownloadResult struct {
	Trials     []SizeResult
	BestMbps   float64
	TotalBytes int64
	Duration   time.Duration
}

// UploadResult holds upload speed test results.
type UploadResult struct {
	Trials     []SizeResult
	BestMbps   float64
	TotalBytes int64
	Duration   time.Duration
}

// LatencyResult holds latency and jitter measurements.
type LatencyResult struct {
	Pings  []time.Duration
	Min    time.Duration
	P50    time.Duration
	P90    time.Duration
	Max    time.Duration
	Jitter time.Duration
}

// Result holds the complete speed test output.
type Result struct {
	Download DownloadResult
	Upload   UploadResult
	Latency  LatencyResult
}

// Run executes the full speed test through the given SOCKS proxy.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	cfg.applyDefaults()

	client, err := newSOCKSClient(cfg.SocksAddr, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("connect to socks proxy %s: %w", cfg.SocksAddr, err)
	}

	return runWithClient(ctx, client, &cfg), nil
}

// runWithClient runs speed tests using the provided HTTP client.
func runWithClient(ctx context.Context, client *http.Client, cfg *Config) *Result {
	result := &Result{}

	cfg.log("measuring latency (%d pings)...", cfg.LatencyPings)
	result.Latency = measureLatency(ctx, client, cfg.LatencyURL, cfg.LatencyPings)

	cfg.log("measuring download speed...")
	result.Download = measureDownload(ctx, client, cfg.DownloadURL)

	cfg.log("measuring upload speed...")
	result.Upload = measureUpload(ctx, client, cfg.UploadURL)

	return result
}

func measureLatency(ctx context.Context, client *http.Client, url string, count int) LatencyResult {
	var pings []time.Duration
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			break
		}
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			continue
		}
		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start)
		cancel()
		if err != nil {
			continue
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		pings = append(pings, latency)
	}

	if len(pings) == 0 {
		return LatencyResult{}
	}

	sorted := make([]time.Duration, len(pings))
	copy(sorted, pings)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	return LatencyResult{
		Pings:  pings,
		Min:    sorted[0],
		P50:    percentile(sorted, 50),
		P90:    percentile(sorted, 90),
		Max:    sorted[len(sorted)-1],
		Jitter: jitter(pings),
	}
}

func measureDownload(ctx context.Context, client *http.Client, baseURL string) DownloadResult {
	var result DownloadResult
	for _, size := range DownloadSizes {
		if ctx.Err() != nil {
			break
		}
		url := fmt.Sprintf("%s?bytes=%d", baseURL, size)
		trial := downloadTrial(ctx, client, url, size)
		result.Trials = append(result.Trials, trial)
		if trial.Err == nil {
			result.TotalBytes += size
			result.Duration += trial.Duration
			if trial.SpeedMbps > result.BestMbps {
				result.BestMbps = trial.SpeedMbps
			}
		}
	}
	return result
}

func downloadTrial(ctx context.Context, client *http.Client, url string, expectedSize int64) SizeResult {
	timeout := 30 * time.Second
	if expectedSize > 1024*1024 {
		timeout = 60 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return SizeResult{Size: expectedSize, Err: err}
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return SizeResult{Size: expectedSize, Err: err}
	}
	defer resp.Body.Close()

	n, _ := io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start)

	if n <= 0 || elapsed <= 0 {
		return SizeResult{Size: expectedSize, Err: fmt.Errorf("no data received")}
	}

	mbps := (float64(n*8) / 1_000_000.0) / elapsed.Seconds()
	return SizeResult{
		Size:      expectedSize,
		SpeedMbps: mbps,
		Duration:  elapsed,
	}
}

func measureUpload(ctx context.Context, client *http.Client, url string) UploadResult {
	var result UploadResult
	for _, size := range UploadSizes {
		if ctx.Err() != nil {
			break
		}
		trial := uploadTrial(ctx, client, url, size)
		result.Trials = append(result.Trials, trial)
		if trial.Err == nil {
			result.TotalBytes += size
			result.Duration += trial.Duration
			if trial.SpeedMbps > result.BestMbps {
				result.BestMbps = trial.SpeedMbps
			}
		}
	}
	return result
}

func uploadTrial(ctx context.Context, client *http.Client, url string, size int64) SizeResult {
	payload := make([]byte, size)
	rand.Read(payload)

	timeout := 30 * time.Second
	if size > 512*1024 {
		timeout = 60 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return SizeResult{Size: size, Err: err}
	}
	req.ContentLength = size

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return SizeResult{Size: size, Err: err}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	elapsed := time.Since(start)

	if elapsed <= 0 {
		return SizeResult{Size: size, Err: fmt.Errorf("upload too fast to measure")}
	}

	mbps := (float64(size*8) / 1_000_000.0) / elapsed.Seconds()
	return SizeResult{
		Size:      size,
		SpeedMbps: mbps,
		Duration:  elapsed,
	}
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return time.Duration(float64(sorted[lower])*(1-frac) + float64(sorted[upper])*frac)
}

func jitter(pings []time.Duration) time.Duration {
	if len(pings) < 2 {
		return 0
	}
	var sum float64
	for _, p := range pings {
		sum += float64(p)
	}
	mean := sum / float64(len(pings))
	var variance float64
	for _, p := range pings {
		d := float64(p) - mean
		variance += d * d
	}
	variance /= float64(len(pings))
	return time.Duration(math.Sqrt(variance))
}

func newSOCKSClient(socksAddr string, timeout time.Duration) (*http.Client, error) {
	base := &net.Dialer{Timeout: timeout}
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, base)
	if err != nil {
		return nil, err
	}
	dialContext := func(c context.Context, network, addr string) (net.Conn, error) {
		if d, ok := dialer.(proxy.ContextDialer); ok {
			return d.DialContext(c, network, addr)
		}
		return dialer.Dial(network, addr)
	}
	tr := &http.Transport{
		DialContext:           dialContext,
		TLSClientConfig:      &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:  timeout,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{Transport: tr, Timeout: timeout}, nil
}

// FormatResult returns a human-readable summary of the speed test.
func FormatResult(r *Result) string {
	var b strings.Builder
	b.WriteString("\n=== Speed Test Results ===\n")

	// Latency
	b.WriteString("\n  Latency\n")
	if len(r.Latency.Pings) == 0 {
		b.WriteString("    No successful pings\n")
	} else {
		fmt.Fprintf(&b, "    Min:    %s\n", formatDuration(r.Latency.Min))
		fmt.Fprintf(&b, "    P50:    %s\n", formatDuration(r.Latency.P50))
		fmt.Fprintf(&b, "    P90:    %s\n", formatDuration(r.Latency.P90))
		fmt.Fprintf(&b, "    Max:    %s\n", formatDuration(r.Latency.Max))
		fmt.Fprintf(&b, "    Jitter: %s\n", formatDuration(r.Latency.Jitter))
		fmt.Fprintf(&b, "    Pings:  %d successful\n", len(r.Latency.Pings))
	}

	// Download
	b.WriteString("\n  Download\n")
	if r.Download.BestMbps <= 0 {
		b.WriteString("    No successful downloads\n")
	} else {
		fmt.Fprintf(&b, "    Best:   %.2f Mbps\n", r.Download.BestMbps)
		for _, t := range r.Download.Trials {
			label := formatBytes(t.Size)
			if t.Err != nil {
				fmt.Fprintf(&b, "    %s: failed (%v)\n", label, t.Err)
			} else {
				fmt.Fprintf(&b, "    %s: %.2f Mbps (%s)\n", label, t.SpeedMbps, formatDuration(t.Duration))
			}
		}
	}

	// Upload
	b.WriteString("\n  Upload\n")
	if r.Upload.BestMbps <= 0 {
		b.WriteString("    No successful uploads\n")
	} else {
		fmt.Fprintf(&b, "    Best:   %.2f Mbps\n", r.Upload.BestMbps)
		for _, t := range r.Upload.Trials {
			label := formatBytes(t.Size)
			if t.Err != nil {
				fmt.Fprintf(&b, "    %s: failed (%v)\n", label, t.Err)
			} else {
				fmt.Fprintf(&b, "    %s: %.2f Mbps (%s)\n", label, t.SpeedMbps, formatDuration(t.Duration))
			}
		}
	}

	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d\u00b5s", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatBytes(n int64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%d MB", n/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%d KB", n/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
