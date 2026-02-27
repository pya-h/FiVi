package latency

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"testing"
	"time"

	"fivi/internal/parser"
)

// closedPort returns a localhost port that is guaranteed to not be listening.
func closedPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestScanBatchPassesReachable(t *testing.T) {
	// Start a local TCP listener to simulate a reachable server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().String()
	host, port, _ := net.SplitHostPort(addr)
	uri := fmt.Sprintf("vless://uuid@%s:%s?type=tcp&security=tls#test", host, port)
	candidates := []parser.Candidate{{Raw: uri, Scheme: "vless"}}

	ctx := context.Background()
	passed, rejected := ScanBatch(ctx, candidates, 2*time.Second, 5*time.Second, 10)
	if rejected != 0 {
		t.Fatalf("expected 0 rejected, got %d", rejected)
	}
	if len(passed) != 1 {
		t.Fatalf("expected 1 passed, got %d", len(passed))
	}
}

func TestScanBatchRejectsUnreachable(t *testing.T) {
	// Use a localhost port that is guaranteed to be closed (connection refused).
	addr := closedPort(t)
	host, port, _ := net.SplitHostPort(addr)
	uri := fmt.Sprintf("vless://uuid@%s:%s?type=tcp#unreachable", host, port)
	candidates := []parser.Candidate{{Raw: uri, Scheme: "vless"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	passed, rejected := ScanBatch(ctx, candidates, 1*time.Second, 500*time.Millisecond, 10)
	if rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", rejected)
	}
	if len(passed) != 0 {
		t.Fatalf("expected 0 passed, got %d", len(passed))
	}
}

func TestScanBatchPassesUnparseable(t *testing.T) {
	// Candidates with unparseable URIs should pass through (not rejected).
	candidates := []parser.Candidate{{Raw: "vmess://invalid-not-base64!!!", Scheme: "vmess"}}

	ctx := context.Background()
	passed, rejected := ScanBatch(ctx, candidates, 2*time.Second, 5*time.Second, 10)
	if rejected != 0 {
		t.Fatalf("expected 0 rejected for unparseable, got %d", rejected)
	}
	if len(passed) != 1 {
		t.Fatalf("expected 1 passed, got %d", len(passed))
	}
}

func TestScanBatchEmpty(t *testing.T) {
	passed, rejected := ScanBatch(context.Background(), nil, time.Second, time.Second, 10)
	if len(passed) != 0 || rejected != 0 {
		t.Fatal("expected empty results for empty input")
	}
}

func TestScanBatchMixed(t *testing.T) {
	// One reachable, one unreachable.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().String()
	host, port, _ := net.SplitHostPort(addr)

	payload := fmt.Sprintf(`{"add":"%s","port":"%s","id":"uuid","ps":"reachable"}`, host, port)
	vmessURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	// Get a closed port for the unreachable candidate.
	closedAddr := closedPort(t)
	closedHost, closedP, _ := net.SplitHostPort(closedAddr)
	unreachableURI := fmt.Sprintf("vless://uuid@%s:%s?type=tcp#unreachable", closedHost, closedP)

	candidates := []parser.Candidate{
		{Raw: vmessURI, Scheme: "vmess"},
		{Raw: unreachableURI, Scheme: "vless"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	passed, rejected := ScanBatch(ctx, candidates, 1*time.Second, 5*time.Second, 10)
	if rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", rejected)
	}
	if len(passed) != 1 {
		t.Fatalf("expected 1 passed, got %d", len(passed))
	}
	if passed[0].Scheme != "vmess" {
		t.Fatalf("expected vmess to pass, got %s", passed[0].Scheme)
	}
}
