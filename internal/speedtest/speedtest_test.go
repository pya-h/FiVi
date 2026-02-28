package speedtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{10, 20, 30, 40, 50}
	if p := percentile(sorted, 50); p != 30 {
		t.Errorf("p50 = %v, want 30", p)
	}
	if p := percentile(sorted, 0); p != 10 {
		t.Errorf("p0 = %v, want 10", p)
	}
	if p := percentile(sorted, 100); p != 50 {
		t.Errorf("p100 = %v, want 50", p)
	}
}

func TestPercentileEmpty(t *testing.T) {
	if p := percentile(nil, 50); p != 0 {
		t.Errorf("percentile(nil) = %v, want 0", p)
	}
}

func TestPercentileSingle(t *testing.T) {
	sorted := []time.Duration{42}
	if p := percentile(sorted, 50); p != 42 {
		t.Errorf("p50 = %v, want 42", p)
	}
}

func TestJitterAllSame(t *testing.T) {
	pings := []time.Duration{100, 100, 100}
	if j := jitter(pings); j != 0 {
		t.Errorf("jitter(same) = %v, want 0", j)
	}
}

func TestJitterLessThanTwo(t *testing.T) {
	if j := jitter([]time.Duration{100}); j != 0 {
		t.Errorf("jitter(1 ping) = %v, want 0", j)
	}
	if j := jitter(nil); j != 0 {
		t.Errorf("jitter(nil) = %v, want 0", j)
	}
}

func TestJitterPositive(t *testing.T) {
	pings := []time.Duration{10 * time.Millisecond, 50 * time.Millisecond, 30 * time.Millisecond}
	j := jitter(pings)
	if j <= 0 {
		t.Error("jitter should be positive for varied pings")
	}
}

func TestMeasureLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	result := measureLatency(context.Background(), srv.Client(), srv.URL, 5)
	if len(result.Pings) != 5 {
		t.Fatalf("expected 5 pings, got %d", len(result.Pings))
	}
	if result.Min <= 0 {
		t.Error("Min should be positive")
	}
	if result.Min > result.P50 || result.P50 > result.P90 || result.P90 > result.Max {
		t.Errorf("ordering violated: min=%v p50=%v p90=%v max=%v",
			result.Min, result.P50, result.P90, result.Max)
	}
	if result.Jitter < 0 {
		t.Error("jitter should be non-negative")
	}
}

func TestMeasureLatencyAllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := srv.Client()
	srv.Close()

	result := measureLatency(context.Background(), client, srv.URL, 3)
	if len(result.Pings) != 0 {
		t.Fatalf("expected 0 pings from closed server, got %d", len(result.Pings))
	}
}

func TestMeasureDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := 1024
		if s := r.URL.Query().Get("bytes"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				n = v
			}
		}
		w.Write(make([]byte, n))
	}))
	defer srv.Close()

	orig := DownloadSizes
	DownloadSizes = []int64{1024, 4096}
	defer func() { DownloadSizes = orig }()

	result := measureDownload(context.Background(), srv.Client(), srv.URL)
	if len(result.Trials) != 2 {
		t.Fatalf("expected 2 trials, got %d", len(result.Trials))
	}
	for _, trial := range result.Trials {
		if trial.Err != nil {
			t.Errorf("trial %d bytes: unexpected error: %v", trial.Size, trial.Err)
		}
		if trial.SpeedMbps <= 0 {
			t.Errorf("trial %d bytes: speed should be positive", trial.Size)
		}
	}
	if result.BestMbps <= 0 {
		t.Error("BestMbps should be positive")
	}
	if result.TotalBytes != 5120 {
		t.Errorf("TotalBytes = %d, want 5120", result.TotalBytes)
	}
}

func TestMeasureUpload(t *testing.T) {
	var totalReceived int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		totalReceived += n
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := UploadSizes
	UploadSizes = []int64{512, 2048}
	defer func() { UploadSizes = orig }()

	result := measureUpload(context.Background(), srv.Client(), srv.URL)
	if len(result.Trials) != 2 {
		t.Fatalf("expected 2 trials, got %d", len(result.Trials))
	}
	for _, trial := range result.Trials {
		if trial.Err != nil {
			t.Errorf("trial %d bytes: unexpected error: %v", trial.Size, trial.Err)
		}
		if trial.SpeedMbps <= 0 {
			t.Errorf("trial %d bytes: speed should be positive", trial.Size)
		}
	}
	if result.BestMbps <= 0 {
		t.Error("BestMbps should be positive")
	}
	if totalReceived != 2560 {
		t.Errorf("server received %d bytes, want 2560", totalReceived)
	}
}

func TestRunWithClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latency", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		n := 1024
		if s := r.URL.Query().Get("bytes"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				n = v
			}
		}
		w.Write(make([]byte, n))
	})
	mux.HandleFunc("/up", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig1, orig2 := DownloadSizes, UploadSizes
	DownloadSizes = []int64{1024}
	UploadSizes = []int64{1024}
	defer func() { DownloadSizes, UploadSizes = orig1, orig2 }()

	cfg := Config{
		DownloadURL:  srv.URL + "/down",
		UploadURL:    srv.URL + "/up",
		LatencyURL:   srv.URL + "/latency",
		LatencyPings: 3,
	}
	cfg.applyDefaults()

	result := runWithClient(context.Background(), srv.Client(), &cfg)
	if len(result.Latency.Pings) != 3 {
		t.Errorf("expected 3 pings, got %d", len(result.Latency.Pings))
	}
	if len(result.Download.Trials) != 1 {
		t.Errorf("expected 1 download trial, got %d", len(result.Download.Trials))
	}
	if len(result.Upload.Trials) != 1 {
		t.Errorf("expected 1 upload trial, got %d", len(result.Upload.Trials))
	}
}

func TestFormatResult(t *testing.T) {
	r := &Result{
		Latency: LatencyResult{
			Pings:  []time.Duration{50 * time.Millisecond, 60 * time.Millisecond, 70 * time.Millisecond},
			Min:    50 * time.Millisecond,
			P50:    60 * time.Millisecond,
			P90:    70 * time.Millisecond,
			Max:    70 * time.Millisecond,
			Jitter: 8 * time.Millisecond,
		},
		Download: DownloadResult{
			BestMbps: 15.5,
			Trials: []SizeResult{
				{Size: 102400, SpeedMbps: 10.0, Duration: 80 * time.Millisecond},
				{Size: 1048576, SpeedMbps: 15.5, Duration: 540 * time.Millisecond},
			},
		},
		Upload: UploadResult{
			BestMbps: 5.2,
			Trials: []SizeResult{
				{Size: 102400, SpeedMbps: 5.2, Duration: 157 * time.Millisecond},
			},
		},
	}

	out := FormatResult(r)
	for _, want := range []string{
		"Speed Test Results",
		"Latency", "P50", "P90", "Jitter",
		"Download", "Upload",
		"15.50 Mbps", "5.20 Mbps",
		"100 KB", "1 MB",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatResult missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatResultNoData(t *testing.T) {
	r := &Result{}
	out := FormatResult(r)
	if !strings.Contains(out, "No successful pings") {
		t.Errorf("expected 'No successful pings' in:\n%s", out)
	}
	if !strings.Contains(out, "No successful downloads") {
		t.Errorf("expected 'No successful downloads' in:\n%s", out)
	}
	if !strings.Contains(out, "No successful uploads") {
		t.Errorf("expected 'No successful uploads' in:\n%s", out)
	}
}

func TestFormatResultWithErrors(t *testing.T) {
	r := &Result{
		Download: DownloadResult{
			BestMbps: 5.0,
			Trials: []SizeResult{
				{Size: 102400, SpeedMbps: 5.0, Duration: 160 * time.Millisecond},
				{Size: 1048576, Err: io.ErrUnexpectedEOF},
			},
		},
	}
	out := FormatResult(r)
	if !strings.Contains(out, "failed") {
		t.Errorf("expected 'failed' for errored trial in:\n%s", out)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := measureLatency(ctx, srv.Client(), srv.URL, 5)
	if len(result.Pings) != 0 {
		t.Errorf("expected 0 pings with cancelled context, got %d", len(result.Pings))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Microsecond, "500\u00b5s"},
		{42 * time.Millisecond, "42ms"},
		{1500 * time.Millisecond, "1.5s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{512, "512 B"},
		{102400, "100 KB"},
		{1048576, "1 MB"},
		{10485760, "10 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.n)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{}
	cfg.applyDefaults()
	if cfg.DownloadURL != DefaultDownloadURL {
		t.Errorf("DownloadURL = %q, want %q", cfg.DownloadURL, DefaultDownloadURL)
	}
	if cfg.UploadURL != DefaultUploadURL {
		t.Errorf("UploadURL = %q, want %q", cfg.UploadURL, DefaultUploadURL)
	}
	if cfg.LatencyURL != DefaultLatencyURL {
		t.Errorf("LatencyURL = %q, want %q", cfg.LatencyURL, DefaultLatencyURL)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.LatencyPings != DefaultLatencyPings {
		t.Errorf("LatencyPings = %d, want %d", cfg.LatencyPings, DefaultLatencyPings)
	}
}
