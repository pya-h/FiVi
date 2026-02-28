package dns

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Remote != "tls://8.8.8.8" {
		t.Fatalf("expected remote tls://8.8.8.8, got %q", cfg.Remote)
	}
	if cfg.Direct != "223.5.5.5" {
		t.Fatalf("expected direct 223.5.5.5, got %q", cfg.Direct)
	}
}

func TestNeedsBootstrapIP(t *testing.T) {
	cfg := Config{Remote: "tls://8.8.8.8", Direct: "223.5.5.5"}
	if cfg.NeedsBootstrap() {
		t.Fatal("IP-based remote should not need bootstrap")
	}
}

func TestNeedsBootstrapHostname(t *testing.T) {
	cfg := Config{Remote: "https://dns.google/dns-query", Direct: "223.5.5.5"}
	if !cfg.NeedsBootstrap() {
		t.Fatal("hostname-based remote should need bootstrap")
	}
}

func TestNeedsBootstrapDoTHostname(t *testing.T) {
	cfg := Config{Remote: "tls://dns.google", Direct: "223.5.5.5"}
	if !cfg.NeedsBootstrap() {
		t.Fatal("DoT hostname-based remote should need bootstrap")
	}
}

func TestNeedsBootstrapPlainIP(t *testing.T) {
	cfg := Config{Remote: "1.1.1.1", Direct: "223.5.5.5"}
	if cfg.NeedsBootstrap() {
		t.Fatal("plain IP remote should not need bootstrap")
	}
}

func TestRemoteIsDoH(t *testing.T) {
	cfg := Config{Remote: "https://1.1.1.1/dns-query"}
	if !cfg.RemoteIsDoH() {
		t.Fatal("expected DoH")
	}
	if cfg.RemoteIsDoT() {
		t.Fatal("should not be DoT")
	}
}

func TestRemoteIsDoT(t *testing.T) {
	cfg := Config{Remote: "tls://8.8.8.8"}
	if !cfg.RemoteIsDoT() {
		t.Fatal("expected DoT")
	}
	if cfg.RemoteIsDoH() {
		t.Fatal("should not be DoH")
	}
}

func TestRemoteIsPlain(t *testing.T) {
	cfg := Config{Remote: "8.8.8.8"}
	if !cfg.RemoteIsPlain() {
		t.Fatal("expected plain")
	}
}

func TestBootstrapAddr(t *testing.T) {
	cfg := Config{Remote: "https://dns.google/dns-query", Direct: "223.5.5.5"}
	if cfg.BootstrapAddr() != "223.5.5.5" {
		t.Fatalf("expected 223.5.5.5, got %q", cfg.BootstrapAddr())
	}
}

func TestExtractHostVariousFormats(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"tls://8.8.8.8", "8.8.8.8"},
		{"https://dns.google/dns-query", "dns.google"},
		{"https://1.1.1.1/dns-query", "1.1.1.1"},
		{"223.5.5.5", "223.5.5.5"},
		{"8.8.8.8:53", "8.8.8.8"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractHost(tt.input)
		if got != tt.want {
			t.Errorf("extractHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
