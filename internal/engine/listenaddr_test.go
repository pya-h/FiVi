package engine

import (
	"net"
	"strings"
	"testing"
)

func TestEnsureListenAddrsAvailable(t *testing.T) {
	if err := ensureListenAddrsAvailable("127.0.0.1:0", "127.0.0.1:0"); err != nil {
		t.Fatalf("expected available wildcard addresses, got error: %v", err)
	}
}

func TestEnsureListenAddrsAvailableDetectsConflict(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen setup failed: %v", err)
	}
	defer ln.Close()

	err = ensureListenAddrsAvailable(ln.Addr().String(), "127.0.0.1:0")
	if err == nil {
		t.Fatalf("expected port conflict error")
	}
	if !strings.Contains(err.Error(), "--socks-addr") {
		t.Fatalf("expected actionable flag hint in error, got: %v", err)
	}
}
