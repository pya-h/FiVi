package engine

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestWaitForProxyReadySuccess(t *testing.T) {
	socksLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen socks: %v", err)
	}
	defer socksLn.Close()

	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen http: %v", err)
	}
	defer httpLn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := waitForProxyReady(ctx, socksLn.Addr().String(), httpLn.Addr().String(), 400*time.Millisecond); err != nil {
		t.Fatalf("waitForProxyReady returned error: %v", err)
	}
}

func TestWaitForProxyReadyTimeout(t *testing.T) {
	socksAddr, err := reserveAddr()
	if err != nil {
		t.Fatalf("reserve socks addr: %v", err)
	}
	httpAddr, err := reserveAddr()
	if err != nil {
		t.Fatalf("reserve http addr: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = waitForProxyReady(ctx, socksAddr, httpAddr, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected readiness timeout error, got nil")
	}
}
