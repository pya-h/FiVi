package xray

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"fivi/internal/routing"
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

	cfgBytes, _, err := BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false, nil)
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

func TestBuildRuntimeConfigTUNMode(t *testing.T) {
	payload := `{"ps":"n1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfgBytes, _, err := BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", true, nil)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI error: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(cfgBytes, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	// TUN inbound should be the first inbound.
	inbounds, ok := root["inbounds"].([]any)
	if !ok || len(inbounds) < 3 {
		t.Fatalf("expected at least 3 inbounds (tun+socks+http), got %d", len(inbounds))
	}
	tunIn, ok := inbounds[0].(map[string]any)
	if !ok {
		t.Fatal("first inbound is not a map")
	}
	if tunIn["protocol"] != "tun" {
		t.Fatalf("first inbound protocol=%v, want tun", tunIn["protocol"])
	}
	if tunIn["tag"] != "tun-in" {
		t.Fatalf("tun inbound tag=%v, want tun-in", tunIn["tag"])
	}
	tunSettings, ok := tunIn["settings"].(map[string]any)
	if !ok {
		t.Fatal("missing tun settings")
	}
	if tunSettings["name"] != TUNName {
		t.Fatalf("tun name=%v, want %s", tunSettings["name"], TUNName)
	}

	// Proxy outbound should have sockopt mark.
	outbounds, ok := root["outbounds"].([]any)
	if !ok || len(outbounds) < 2 {
		t.Fatal("missing outbounds")
	}
	proxyOut, ok := outbounds[0].(map[string]any)
	if !ok {
		t.Fatal("first outbound is not a map")
	}
	stream, ok := proxyOut["streamSettings"].(map[string]any)
	if !ok {
		t.Fatal("proxy outbound missing streamSettings")
	}
	sockopt, ok := stream["sockopt"].(map[string]any)
	if !ok {
		t.Fatal("proxy outbound missing sockopt")
	}
	if mark, ok := sockopt["mark"].(float64); !ok || int(mark) != TUNMark {
		t.Fatalf("proxy sockopt mark=%v, want %d", sockopt["mark"], TUNMark)
	}

	// Direct outbound should also have sockopt mark.
	directOut, ok := outbounds[1].(map[string]any)
	if !ok {
		t.Fatal("second outbound is not a map")
	}
	directStream, ok := directOut["streamSettings"].(map[string]any)
	if !ok {
		t.Fatal("direct outbound missing streamSettings in tunnel mode")
	}
	directSockopt, ok := directStream["sockopt"].(map[string]any)
	if !ok {
		t.Fatal("direct outbound missing sockopt")
	}
	if mark, ok := directSockopt["mark"].(float64); !ok || int(mark) != TUNMark {
		t.Fatalf("direct sockopt mark=%v, want %d", directSockopt["mark"], TUNMark)
	}
}

func TestBuildRuntimeConfigWithRoutingRules(t *testing.T) {
	payload := `{"ps":"n1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	rules := []routing.Rule{
		{Type: routing.RuleDomain, Value: "ir", Wildcard: true},
		{Type: routing.RuleDomain, Value: "digikala.com", Wildcard: true},
		{Type: routing.RuleDomain, Value: "forced.com", Negated: true},
		{Type: routing.RuleIP, Value: "1.2.3.4"},
		{Type: routing.RuleRange, Value: "5.52.0.0/14"},
		{Type: routing.RuleRange, Value: "100.64.0.0/10", Negated: true},
	}

	cfgBytes, _, err := BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false, rules)
	if err != nil {
		t.Fatalf("BuildRuntimeConfigFromURI error: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(cfgBytes, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	routingSection := root["routing"].(map[string]any)
	routeRules := routingSection["rules"].([]any)

	// Expected order:
	// [0] private IP direct
	// [1] negated domain → proxy (domain:!forced.com)
	// [2] negated IP/CIDR → proxy (range:!100.64.0.0/10)
	// [3] direct domains (domain:*.ir, domain:*.digikala.com)
	// [4] direct IPs (ip:1.2.3.4 + range:5.52.0.0/14)
	// [5] catch-all proxy
	if len(routeRules) != 6 {
		t.Fatalf("expected 6 routing rules, got %d", len(routeRules))
	}

	// [0] Private IP → direct
	r0 := routeRules[0].(map[string]any)
	if r0["outboundTag"] != "direct" {
		t.Fatalf("rule[0] outboundTag=%v, want direct", r0["outboundTag"])
	}

	// [1] Negated domain → proxy
	r1 := routeRules[1].(map[string]any)
	if r1["outboundTag"] != "proxy" {
		t.Fatalf("rule[1] outboundTag=%v, want proxy", r1["outboundTag"])
	}
	domains := r1["domain"].([]any)
	if len(domains) != 1 || domains[0] != "full:forced.com" {
		t.Fatalf("rule[1] domains=%v, want [full:forced.com]", domains)
	}

	// [2] Negated CIDR → proxy
	r2 := routeRules[2].(map[string]any)
	if r2["outboundTag"] != "proxy" {
		t.Fatalf("rule[2] outboundTag=%v, want proxy", r2["outboundTag"])
	}
	ips := r2["ip"].([]any)
	if len(ips) != 1 || ips[0] != "100.64.0.0/10" {
		t.Fatalf("rule[2] ips=%v, want [100.64.0.0/10]", ips)
	}

	// [3] Direct domains (wildcard → "domain:" prefix)
	r3 := routeRules[3].(map[string]any)
	if r3["outboundTag"] != "direct" {
		t.Fatalf("rule[3] outboundTag=%v, want direct", r3["outboundTag"])
	}
	directDomains := r3["domain"].([]any)
	if len(directDomains) != 2 {
		t.Fatalf("expected 2 direct domains, got %d", len(directDomains))
	}
	if directDomains[0] != "domain:ir" || directDomains[1] != "domain:digikala.com" {
		t.Fatalf("rule[3] domains=%v, want [domain:ir domain:digikala.com]", directDomains)
	}

	// [4] Direct IPs + CIDRs
	r4 := routeRules[4].(map[string]any)
	if r4["outboundTag"] != "direct" {
		t.Fatalf("rule[4] outboundTag=%v, want direct", r4["outboundTag"])
	}
	directIPs := r4["ip"].([]any)
	if len(directIPs) != 2 {
		t.Fatalf("expected 2 direct IPs, got %d", len(directIPs))
	}

	// [5] Catch-all → proxy
	r5 := routeRules[5].(map[string]any)
	if r5["outboundTag"] != "proxy" || r5["network"] != "tcp,udp" {
		t.Fatalf("rule[5] not catch-all proxy: %v", r5)
	}
}

func TestBuildRuntimeConfigNoRulesDefaultBehavior(t *testing.T) {
	payload := `{"ps":"n1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfgBytes, _, err := BuildRuntimeConfigFromURI(link, "127.0.0.1:10808", "127.0.0.1:10809", false, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(cfgBytes, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	routingSection := root["routing"].(map[string]any)
	routeRules := routingSection["rules"].([]any)

	// With nil rules: private IP direct + catch-all proxy = 2 rules
	if len(routeRules) != 2 {
		t.Fatalf("expected 2 routing rules with nil input, got %d", len(routeRules))
	}
}
