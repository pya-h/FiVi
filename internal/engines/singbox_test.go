package engines

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSingboxAdapterName(t *testing.T) {
	adapter := NewSingboxAdapter()
	if adapter.Name() != "singbox" {
		t.Fatalf("unexpected adapter name: %q", adapter.Name())
	}
}

func TestSingboxBuildRuntimeConfigFromURI(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"sbox-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"example.com"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, parsed, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809")
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI error: %v", err)
	}
	if parsed.Name != "sbox-node" {
		t.Fatalf("unexpected parsed name: %q", parsed.Name)
	}
	if parsed.Protocol != "vmess" {
		t.Fatalf("unexpected parsed protocol: %q", parsed.Protocol)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("config is not valid json: %v", err)
	}
	outbounds, ok := root["outbounds"].([]any)
	if !ok || len(outbounds) < 1 {
		t.Fatalf("missing outbounds")
	}
	proxy, ok := outbounds[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid proxy outbound structure")
	}
	if got := proxy["type"]; got != "vmess" {
		t.Fatalf("unexpected proxy outbound type: %v", got)
	}
	if got := proxy["server"]; got != "example.com" {
		t.Fatalf("unexpected server: %v", got)
	}
}

func TestSingboxBuildRuntimeConfigFromURIInvalid(t *testing.T) {
	adapter := NewSingboxAdapter()
	if _, _, err := adapter.BuildRuntimeConfigFromURI("invalid://link", "127.0.0.1:10808", "127.0.0.1:10809"); err == nil {
		t.Fatalf("expected error for invalid URI")
	}
}
