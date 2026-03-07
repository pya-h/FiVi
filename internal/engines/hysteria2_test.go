package engines

import (
	"strings"
	"testing"
)

func TestHysteria2AdapterName(t *testing.T) {
	adapter := NewHysteria2Adapter()
	if adapter.Name() != "hysteria2" {
		t.Fatalf("unexpected adapter name: %q", adapter.Name())
	}
}

func TestHysteria2BuildConfig(t *testing.T) {
	adapter := NewHysteria2Adapter()
	uri := "hysteria2://secret@example.com:443/?sni=cdn.example.com&insecure=1&alpn=h3,h2#hy2-node"

	cfg, parsed, err := adapter.BuildConfig(ConfigBuildOpts{
		RawURI:    uri,
		SocksAddr: "127.0.0.1:10808",
		HTTPAddr:  "127.0.0.1:10809",
	})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	if parsed.Name != "hy2-node" {
		t.Fatalf("unexpected parsed name: %q", parsed.Name)
	}
	if parsed.Protocol != "hysteria2" {
		t.Fatalf("unexpected parsed protocol: %q", parsed.Protocol)
	}

	out := string(cfg)
	mustContain(t, out, "server: \"example.com:443\"")
	mustContain(t, out, "auth: \"secret\"")
	mustContain(t, out, "sni: \"cdn.example.com\"")
	mustContain(t, out, "insecure: true")
	mustContain(t, out, "- \"h3\"")
	mustContain(t, out, "- \"h2\"")
	mustContain(t, out, "socks5:")
	mustContain(t, out, "http:")
}

func TestHysteria2BuildConfigHY2Alias(t *testing.T) {
	adapter := NewHysteria2Adapter()
	uri := "hy2://pass@example.com:8443/#edge"

	_, parsed, err := adapter.BuildConfig(ConfigBuildOpts{
		RawURI:    uri,
		SocksAddr: "127.0.0.1:10808",
		HTTPAddr:  "127.0.0.1:10809",
	})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	if parsed.Protocol != "hysteria2" {
		t.Fatalf("unexpected parsed protocol: %q", parsed.Protocol)
	}
}

func TestHysteria2BuildConfigTunnelModeNotSupported(t *testing.T) {
	adapter := NewHysteria2Adapter()
	uri := "hysteria2://secret@example.com:443/#hy2-node"

	if _, _, err := adapter.BuildConfig(ConfigBuildOpts{
		RawURI:     uri,
		SocksAddr:  "127.0.0.1:10808",
		HTTPAddr:   "127.0.0.1:10809",
		TunnelMode: true,
	}); err == nil {
		t.Fatal("expected tunnel mode error")
	}
}

func TestHysteria2BuildConfigMissingAuth(t *testing.T) {
	adapter := NewHysteria2Adapter()
	uri := "hysteria2://example.com:443/#hy2-node"

	if _, _, err := adapter.BuildConfig(ConfigBuildOpts{
		RawURI:    uri,
		SocksAddr: "127.0.0.1:10808",
		HTTPAddr:  "127.0.0.1:10809",
	}); err == nil {
		t.Fatal("expected auth error")
	}
}

func mustContain(t *testing.T, body, fragment string) {
	t.Helper()
	if !strings.Contains(body, fragment) {
		t.Fatalf("expected fragment %q in config:\n%s", fragment, body)
	}
}
