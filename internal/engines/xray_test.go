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

func TestXrayAdapterBuildRuntimeConfigFromURI(t *testing.T) {
	adapter := NewXrayAdapter()
	payload := `{"ps":"node-a","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, parsed, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI error: %v", err)
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
