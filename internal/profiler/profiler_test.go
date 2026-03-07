package profiler

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNormalizeAddr(t *testing.T) {
	if got := normalizeAddr(""); got != DefaultAddr {
		t.Fatalf("normalizeAddr("")=%q want=%q", got, DefaultAddr)
	}
	if got := normalizeAddr(" 127.0.0.1:7070 "); got != "127.0.0.1:7070" {
		t.Fatalf("normalizeAddr(trim)=%q", got)
	}
}

func TestStartServesPProfIndex(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := Start(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = srv.Stop(stopCtx)
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + srv.Addr + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET pprof index: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pprof status=%d want=200", resp.StatusCode)
	}
}
