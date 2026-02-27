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

	cfg, parsed, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false, nil)
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
	if _, _, err := adapter.BuildRuntimeConfigFromURI("invalid://link", "127.0.0.1:10808", "127.0.0.1:10809", false, nil); err == nil {
		t.Fatalf("expected error for invalid URI")
	}
}

func TestSingboxTunnelModeAddsTUNInbound(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"tunnel-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", true, nil)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI tunnel error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("config is not valid json: %v", err)
	}

	inbounds, ok := root["inbounds"].([]any)
	if !ok {
		t.Fatalf("missing inbounds")
	}

	// Tunnel mode should have 3 inbounds: tun, socks, http
	if len(inbounds) != 3 {
		t.Fatalf("expected 3 inbounds in tunnel mode, got %d", len(inbounds))
	}

	tun, ok := inbounds[0].(map[string]any)
	if !ok {
		t.Fatal("first inbound is not a map")
	}
	if tun["type"] != "tun" {
		t.Fatalf("first inbound type should be tun, got %v", tun["type"])
	}
	if tun["auto_route"] != true {
		t.Fatal("tun inbound should have auto_route=true")
	}

	// Route should have auto_detect_interface in tunnel mode.
	route, ok := root["route"].(map[string]any)
	if !ok {
		t.Fatal("missing route")
	}
	if route["auto_detect_interface"] != true {
		t.Fatal("route should have auto_detect_interface=true in tunnel mode")
	}
}

func TestSingboxProxyModeNoTUN(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"proxy-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false, nil)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI proxy error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("config is not valid json: %v", err)
	}

	inbounds, ok := root["inbounds"].([]any)
	if !ok {
		t.Fatalf("missing inbounds")
	}

	// Proxy mode should have 2 inbounds: socks, http (no tun)
	if len(inbounds) != 2 {
		t.Fatalf("expected 2 inbounds in proxy mode, got %d", len(inbounds))
	}

	first, ok := inbounds[0].(map[string]any)
	if !ok {
		t.Fatal("first inbound is not a map")
	}
	if first["type"] == "tun" {
		t.Fatal("proxy mode should not have tun inbound")
	}
}

func TestSingboxProxyModeHasDNSAndSniffing(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"dns-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false, nil)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// DNS section exists with servers and strategy.
	dns, ok := root["dns"].(map[string]any)
	if !ok {
		t.Fatal("missing dns section")
	}
	if dns["strategy"] != "prefer_ipv4" {
		t.Fatalf("expected dns strategy=prefer_ipv4, got %v", dns["strategy"])
	}
	servers, ok := dns["servers"].([]any)
	if !ok || len(servers) < 2 {
		t.Fatalf("expected at least 2 dns servers, got %d", len(servers))
	}

	// Sniffing on socks inbound.
	inbounds := root["inbounds"].([]any)
	socksIn := inbounds[0].(map[string]any)
	if socksIn["sniff"] != true {
		t.Fatal("socks inbound missing sniff=true")
	}
	if socksIn["sniff_override_destination"] != true {
		t.Fatal("socks inbound missing sniff_override_destination=true")
	}

	// Sniffing on http inbound.
	httpIn := inbounds[1].(map[string]any)
	if httpIn["sniff"] != true {
		t.Fatal("http inbound missing sniff=true")
	}

	// dns-out outbound exists.
	outbounds := root["outbounds"].([]any)
	hasDNSOut := false
	for _, ob := range outbounds {
		m := ob.(map[string]any)
		if m["type"] == "dns" && m["tag"] == "dns-out" {
			hasDNSOut = true
			break
		}
	}
	if !hasDNSOut {
		t.Fatal("missing dns-out outbound")
	}

	// Route has dns protocol rule and ip_is_private rule.
	route := root["route"].(map[string]any)
	rules := route["rules"].([]any)
	if len(rules) < 3 {
		t.Fatalf("expected at least 3 route rules, got %d", len(rules))
	}
	dnsRule := rules[0].(map[string]any)
	if dnsRule["protocol"] != "dns" || dnsRule["outbound"] != "dns-out" {
		t.Fatalf("first route rule should be dns->dns-out, got %v", dnsRule)
	}
	privateRule := rules[1].(map[string]any)
	if privateRule["ip_is_private"] != true || privateRule["outbound"] != "direct" {
		t.Fatalf("second route rule should be ip_is_private->direct, got %v", privateRule)
	}
}

func TestSingboxTunnelModeHasNATAndSniffing(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"tun-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", true, nil)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI tunnel error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	inbounds := root["inbounds"].([]any)
	tun := inbounds[0].(map[string]any)
	if tun["type"] != "tun" {
		t.Fatal("first inbound should be tun")
	}
	if tun["endpoint_independent_nat"] != true {
		t.Fatal("tun inbound missing endpoint_independent_nat=true")
	}
	if tun["sniff"] != true {
		t.Fatal("tun inbound missing sniff=true")
	}
	if tun["sniff_override_destination"] != true {
		t.Fatal("tun inbound missing sniff_override_destination=true")
	}

	// DNS section exists in tunnel mode.
	if root["dns"] == nil {
		t.Fatal("missing dns section in tunnel mode")
	}
}
