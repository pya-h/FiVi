package xray

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParseURIUppercaseVMessPrefix(t *testing.T) {
	payload := `{"ps":"n1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "VMESS://" + base64.StdEncoding.EncodeToString([]byte(payload))

	parsed, err := ParseURI(link)
	if err != nil {
		t.Fatalf("ParseURI error: %v", err)
	}
	if parsed.Protocol != "vmess" {
		t.Fatalf("expected vmess protocol, got %q", parsed.Protocol)
	}
}

func TestParseURIUppercaseSSPrefix(t *testing.T) {
	link := "SS://YWVzLTI1Ni1nY206cGFzc0BleGFtcGxlLmNvbTo0NDM="

	parsed, err := ParseURI(link)
	if err != nil {
		t.Fatalf("ParseURI error: %v", err)
	}
	if parsed.Protocol != "ss" {
		t.Fatalf("expected ss protocol, got %q", parsed.Protocol)
	}
}

func TestBuildRuntimeConfigHasDNSSniffingAndRouting(t *testing.T) {
	payload := `{"ps":"n1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfgBytes, _, err := BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809")
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI error: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(cfgBytes, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// DNS section exists.
	if root["dns"] == nil {
		t.Fatal("missing dns section")
	}

	// Routing uses IPIfNonMatch.
	routing, ok := root["routing"].(map[string]any)
	if !ok {
		t.Fatal("missing routing section")
	}
	if routing["domainStrategy"] != "IPIfNonMatch" {
		t.Fatalf("expected domainStrategy=IPIfNonMatch, got %v", routing["domainStrategy"])
	}

	// First routing rule directs private IPs.
	rules, ok := routing["rules"].([]any)
	if !ok || len(rules) < 2 {
		t.Fatalf("expected at least 2 routing rules, got %d", len(rules))
	}
	directRule, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatal("first routing rule is not a map")
	}
	if directRule["outboundTag"] != "direct" {
		t.Fatalf("first rule outboundTag=%v, want direct", directRule["outboundTag"])
	}

	// Sniffing enabled on first inbound (socks).
	inbounds, ok := root["inbounds"].([]any)
	if !ok || len(inbounds) < 2 {
		t.Fatal("missing inbounds")
	}
	socksIn, ok := inbounds[0].(map[string]any)
	if !ok {
		t.Fatal("first inbound is not a map")
	}
	sniff, ok := socksIn["sniffing"].(map[string]any)
	if !ok {
		t.Fatal("missing sniffing on socks inbound")
	}
	if sniff["enabled"] != true {
		t.Fatal("sniffing not enabled on socks inbound")
	}

	// Sniffing enabled on second inbound (http).
	httpIn, ok := inbounds[1].(map[string]any)
	if !ok {
		t.Fatal("second inbound is not a map")
	}
	sniffHTTP, ok := httpIn["sniffing"].(map[string]any)
	if !ok {
		t.Fatal("missing sniffing on http inbound")
	}
	if sniffHTTP["enabled"] != true {
		t.Fatal("sniffing not enabled on http inbound")
	}
}
