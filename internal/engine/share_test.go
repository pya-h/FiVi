package engine

import "testing"

func TestResolveHostForSystemProxy(t *testing.T) {
	ips := []string{"127.0.0.1", "", "192.168.1.40", "10.0.0.3"}
	if got := resolveHostForSystemProxyWithPreferred(ips, ""); got != "192.168.1.40" {
		t.Fatalf("resolveHostForSystemProxy()=%q", got)
	}
}

func TestResolveHostForSystemProxyNoLAN(t *testing.T) {
	ips := []string{"127.0.0.1", "::1"}
	if got := resolveHostForSystemProxyWithPreferred(ips, ""); got != "" {
		t.Fatalf("expected empty host, got %q", got)
	}
}

func TestResolveHostForSystemProxyPrefersOutboundIP(t *testing.T) {
	ips := []string{"192.168.1.40", "10.0.0.3"}
	if got := resolveHostForSystemProxyWithPreferred(ips, "10.0.0.3"); got != "10.0.0.3" {
		t.Fatalf("expected preferred outbound IP, got %q", got)
	}
}

func TestResolveSystemProxyAddrNonWildcard(t *testing.T) {
	addr := "127.0.0.1:10808"
	if got := resolveSystemProxyAddr(addr); got != addr {
		t.Fatalf("resolveSystemProxyAddr()=%q want %q", got, addr)
	}
}

func TestIsWildcardHost(t *testing.T) {
	if !isWildcardHost("0.0.0.0") || !isWildcardHost("::") {
		t.Fatal("expected wildcard hosts")
	}
	if isWildcardHost("127.0.0.1") {
		t.Fatal("unexpected wildcard host detection")
	}
}

func TestIsLikelyVirtualInterface(t *testing.T) {
	if !isLikelyVirtualInterface("docker0") || !isLikelyVirtualInterface("vethabc") || !isLikelyVirtualInterface("tailscale0") {
		t.Fatal("expected virtual interface detection for container/vpn interfaces")
	}
	if isLikelyVirtualInterface("eth0") || isLikelyVirtualInterface("wlan0") {
		t.Fatal("unexpected virtual detection for physical interfaces")
	}
}
