package pac

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"fivi/internal/routing"
)

func TestGenerateDefault(t *testing.T) {
	script := Generate("127.0.0.1:10808", nil)
	// Must be a valid function.
	if !strings.Contains(script, "function FindProxyForURL(url, host)") {
		t.Fatal("missing FindProxyForURL function")
	}
	// Must have localhost bypass.
	if !strings.Contains(script, `host === "localhost"`) {
		t.Fatal("missing localhost bypass")
	}
	// Must have private range bypass.
	if !strings.Contains(script, `isInNet(host, "10.0.0.0"`) {
		t.Fatal("missing 10.0.0.0 private range")
	}
	if !strings.Contains(script, `isInNet(host, "192.168.0.0"`) {
		t.Fatal("missing 192.168.0.0 private range")
	}
	// Must have catch-all proxy.
	if !strings.Contains(script, `SOCKS5 127.0.0.1:10808`) {
		t.Fatal("missing SOCKS5 catch-all")
	}
}

func TestGenerateWithDomainRules(t *testing.T) {
	rules := []routing.Rule{
		{Type: routing.RuleDomain, Value: "example.com"},
		{Type: routing.RuleDomain, Value: "*.ir", Wildcard: true},
	}
	script := Generate("127.0.0.1:10808", rules)
	if !strings.Contains(script, `host === "example.com"`) {
		t.Fatal("missing exact domain bypass")
	}
	if !strings.Contains(script, `dnsDomainIs(host, ".ir")`) {
		t.Fatal("missing wildcard domain bypass")
	}
}

func TestGenerateWithIPRules(t *testing.T) {
	rules := []routing.Rule{
		{Type: routing.RuleIP, Value: "1.2.3.4"},
	}
	script := Generate("127.0.0.1:10808", rules)
	if !strings.Contains(script, `host === "1.2.3.4"`) {
		t.Fatal("missing IP bypass")
	}
}

func TestGenerateWithCIDRRule(t *testing.T) {
	rules := []routing.Rule{
		{Type: routing.RuleRange, Value: "10.0.0.0/8"},
	}
	script := Generate("127.0.0.1:10808", rules)
	if !strings.Contains(script, `isInNet(host, "10.0.0.0", "255.0.0.0")`) {
		t.Fatal("missing CIDR bypass")
	}
}

func TestGenerateNegatedRulesSkipped(t *testing.T) {
	rules := []routing.Rule{
		{Type: routing.RuleDomain, Value: "blocked.com", Negated: true},
		{Type: routing.RuleDomain, Value: "allowed.com"},
	}
	script := Generate("127.0.0.1:10808", rules)
	// Negated rules should NOT produce bypass entries.
	if strings.Contains(script, "blocked.com") {
		t.Fatal("negated rule should not produce a bypass entry")
	}
	if !strings.Contains(script, `host === "allowed.com"`) {
		t.Fatal("non-negated rule should produce bypass")
	}
}

func TestCIDRToIPMask(t *testing.T) {
	tests := []struct {
		cidr     string
		wantIP   string
		wantMask string
	}{
		{"10.0.0.0/8", "10.0.0.0", "255.0.0.0"},
		{"172.16.0.0/12", "172.16.0.0", "255.240.0.0"},
		{"192.168.1.0/24", "192.168.1.0", "255.255.255.0"},
	}
	for _, tt := range tests {
		ip, mask := cidrToIPMask(tt.cidr)
		if ip != tt.wantIP {
			t.Errorf("cidrToIPMask(%q) ip = %q, want %q", tt.cidr, ip, tt.wantIP)
		}
		if mask != tt.wantMask {
			t.Errorf("cidrToIPMask(%q) mask = %q, want %q", tt.cidr, mask, tt.wantMask)
		}
	}
}

func TestCIDRToIPMaskInvalid(t *testing.T) {
	ip, mask := cidrToIPMask("not-a-cidr")
	if ip != "" || mask != "" {
		t.Errorf("expected empty for invalid CIDR, got ip=%q mask=%q", ip, mask)
	}
}

func TestPACAddr(t *testing.T) {
	tests := []struct {
		httpAddr string
		want     string
	}{
		{"127.0.0.1:10809", "127.0.0.1:10810"},
		{"0.0.0.0:8080", "0.0.0.0:8081"},
	}
	for _, tt := range tests {
		got := PACAddr(tt.httpAddr)
		if got != tt.want {
			t.Errorf("PACAddr(%q) = %q, want %q", tt.httpAddr, got, tt.want)
		}
	}
}

func TestServerStartAndServe(t *testing.T) {
	rules := []routing.Rule{
		{Type: routing.RuleDomain, Value: "*.ir", Wildcard: true},
	}
	srv, err := Start("127.0.0.1:0", "127.0.0.1:10808", rules)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	url := srv.URL()
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("unexpected URL: %s", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/x-ns-proxy-autoconfig" {
		t.Fatalf("Content-Type = %q, want application/x-ns-proxy-autoconfig", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "FindProxyForURL") {
		t.Fatal("response body missing FindProxyForURL")
	}
	if !strings.Contains(string(body), ".ir") {
		t.Fatal("response body missing .ir rule")
	}
}

func TestServerStopIdempotent(t *testing.T) {
	srv, err := Start("127.0.0.1:0", "127.0.0.1:10808", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	srv.Stop()
	srv.Stop() // should not panic
}
