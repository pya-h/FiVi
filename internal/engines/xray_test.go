package engines

import (
	"encoding/base64"
	"testing"
)

func TestXrayAdapterName(t *testing.T) {
	adapter := NewXrayAdapter()
	if adapter.Name() != "xray" {
		t.Fatalf("unexpected adapter name: %q", adapter.Name())
	}
}

func TestXrayAdapterBuildConfig(t *testing.T) {
	adapter := NewXrayAdapter()
	payload := `{"ps":"node-a","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, parsed, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	if len(cfg) == 0 {
		t.Fatalf("expected config bytes")
	}
	if parsed.Name != "node-a" {
		t.Fatalf("expected parsed name node-a, got %q", parsed.Name)
	}
	if parsed.Protocol != "vmess" {
		t.Fatalf("expected parsed protocol vmess, got %q", parsed.Protocol)
	}
}
