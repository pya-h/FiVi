package engines

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"fivi/internal/routing"
)

func TestSingboxAdapterName(t *testing.T) {
	adapter := NewSingboxAdapter()
	if adapter.Name() != "singbox" {
		t.Fatalf("unexpected adapter name: %q", adapter.Name())
	}
}

func TestSingboxBuildConfig(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"sbox-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"example.com"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, parsed, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
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

func TestSingboxBuildConfigInvalid(t *testing.T) {
	adapter := NewSingboxAdapter()
	if _, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: "invalid://link", SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"}); err == nil {
		t.Fatalf("expected error for invalid URI")
	}
}

func TestSingboxTunnelModeAddsTUNInbound(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"tunnel-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809", TunnelMode: true})
	if err != nil {
		t.Fatalf("BuildConfig tunnel error: %v", err)
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

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"})
	if err != nil {
		t.Fatalf("BuildConfig proxy error: %v", err)
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

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
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

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809", TunnelMode: true})
	if err != nil {
		t.Fatalf("BuildConfig tunnel error: %v", err)
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

func TestSingboxRoutingRulesIntegration(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"route-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	rules := []routing.Rule{
		{Type: routing.RuleDomain, Value: "ir", Wildcard: true},
		{Type: routing.RuleDomain, Value: "digikala.com"},
		{Type: routing.RuleDomain, Value: "forced.com", Negated: true},
		{Type: routing.RuleIP, Value: "1.2.3.4"},
		{Type: routing.RuleRange, Value: "5.52.0.0/14"},
		{Type: routing.RuleRange, Value: "100.64.0.0/10", Negated: true},
	}

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809", Rules: rules})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	route := root["route"].(map[string]any)
	routeRules := route["rules"].([]any)

	// Expected order:
	// [0] dns → dns-out
	// [1] ip_is_private → direct
	// [2] negated domain (forced.com) → proxy
	// [3] negated IP/CIDR (100.64.0.0/10) → proxy
	// [4] direct domains (*.ir, digikala.com)
	// [5] direct IPs (1.2.3.4/32 + 5.52.0.0/14)
	// [6] catch-all → proxy
	if len(routeRules) != 7 {
		t.Fatalf("expected 7 route rules, got %d", len(routeRules))
	}

	// [0] DNS → dns-out
	r0 := routeRules[0].(map[string]any)
	if r0["protocol"] != "dns" || r0["outbound"] != "dns-out" {
		t.Fatalf("rule[0] should be dns→dns-out, got %v", r0)
	}

	// [1] Private IP → direct
	r1 := routeRules[1].(map[string]any)
	if r1["ip_is_private"] != true || r1["outbound"] != "direct" {
		t.Fatalf("rule[1] should be ip_is_private→direct, got %v", r1)
	}

	// [2] Negated domain → proxy
	r2 := routeRules[2].(map[string]any)
	if r2["outbound"] != "proxy" {
		t.Fatalf("rule[2] outbound=%v, want proxy", r2["outbound"])
	}
	domains := r2["domain"].([]any)
	if len(domains) != 1 || domains[0] != "forced.com" {
		t.Fatalf("rule[2] domain=%v, want [forced.com]", domains)
	}

	// [3] Negated CIDR → proxy
	r3 := routeRules[3].(map[string]any)
	if r3["outbound"] != "proxy" {
		t.Fatalf("rule[3] outbound=%v, want proxy", r3["outbound"])
	}
	cidrs := r3["ip_cidr"].([]any)
	if len(cidrs) != 1 || cidrs[0] != "100.64.0.0/10" {
		t.Fatalf("rule[3] ip_cidr=%v, want [100.64.0.0/10]", cidrs)
	}

	// [4] Direct domains — domain_suffix for wildcard, domain for exact
	r4 := routeRules[4].(map[string]any)
	if r4["outbound"] != "direct" {
		t.Fatalf("rule[4] outbound=%v, want direct", r4["outbound"])
	}
	exactDomains := r4["domain"].([]any)
	if len(exactDomains) != 1 || exactDomains[0] != "digikala.com" {
		t.Fatalf("rule[4] domain=%v, want [digikala.com]", exactDomains)
	}
	suffixDomains := r4["domain_suffix"].([]any)
	if len(suffixDomains) != 1 || suffixDomains[0] != "ir" {
		t.Fatalf("rule[4] domain_suffix=%v, want [ir]", suffixDomains)
	}

	// [5] Direct IPs
	r5 := routeRules[5].(map[string]any)
	if r5["outbound"] != "direct" {
		t.Fatalf("rule[5] outbound=%v, want direct", r5["outbound"])
	}
	directCidrs := r5["ip_cidr"].([]any)
	if len(directCidrs) != 2 {
		t.Fatalf("expected 2 direct ip_cidr entries, got %d", len(directCidrs))
	}

	// [6] Catch-all → proxy
	r6 := routeRules[6].(map[string]any)
	if r6["outbound"] != "proxy" {
		t.Fatalf("rule[6] outbound=%v, want proxy", r6["outbound"])
	}
}

func TestSingboxGeoBypassAddsRuleSets(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"geo-node","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	geo := &routing.GeoRuleSets{
		Country:     "ir",
		GeoIPPath:   "/tmp/geoip-ir.srs",
		GeositePath: "/tmp/geosite-ir.srs",
	}

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809", Geo: geo})
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	route := root["route"].(map[string]any)

	// Route rules should include geo rule set references.
	routeRules := route["rules"].([]any)
	// Expected: dns + ip_is_private + geosite-ir + geoip-ir + catch-all = 5
	if len(routeRules) != 5 {
		t.Fatalf("expected 5 route rules with geo bypass, got %d", len(routeRules))
	}

	// [2] geosite rule set → direct
	r2 := routeRules[2].(map[string]any)
	if r2["outbound"] != "direct" {
		t.Fatalf("rule[2] outbound=%v, want direct", r2["outbound"])
	}
	ruleSet2 := r2["rule_set"].([]any)
	if len(ruleSet2) != 1 || ruleSet2[0] != "geosite-ir" {
		t.Fatalf("rule[2] rule_set=%v, want [geosite-ir]", ruleSet2)
	}

	// [3] geoip rule set → direct
	r3 := routeRules[3].(map[string]any)
	if r3["outbound"] != "direct" {
		t.Fatalf("rule[3] outbound=%v, want direct", r3["outbound"])
	}
	ruleSet3 := r3["rule_set"].([]any)
	if len(ruleSet3) != 1 || ruleSet3[0] != "geoip-ir" {
		t.Fatalf("rule[3] rule_set=%v, want [geoip-ir]", ruleSet3)
	}

	// Rule set definitions should exist.
	ruleSets := route["rule_set"].([]any)
	if len(ruleSets) != 2 {
		t.Fatalf("expected 2 rule set definitions, got %d", len(ruleSets))
	}

	geoipDef := ruleSets[0].(map[string]any)
	if geoipDef["type"] != "local" || geoipDef["tag"] != "geoip-ir" || geoipDef["format"] != "binary" {
		t.Fatalf("unexpected geoip rule set def: %v", geoipDef)
	}
	if geoipDef["path"] != "/tmp/geoip-ir.srs" {
		t.Fatalf("geoip path=%v, want /tmp/geoip-ir.srs", geoipDef["path"])
	}

	geositeDef := ruleSets[1].(map[string]any)
	if geositeDef["type"] != "local" || geositeDef["tag"] != "geosite-ir" || geositeDef["format"] != "binary" {
		t.Fatalf("unexpected geosite rule set def: %v", geositeDef)
	}
	if geositeDef["path"] != "/tmp/geosite-ir.srs" {
		t.Fatalf("geosite path=%v, want /tmp/geosite-ir.srs", geositeDef["path"])
	}
}

func TestSingboxGeoBypassWithRoutingRules(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"combo","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	rules := []routing.Rule{
		{Type: routing.RuleDomain, Value: "forced.com", Negated: true},
		{Type: routing.RuleDomain, Value: "ir", Wildcard: true},
	}
	geo := &routing.GeoRuleSets{
		Country:     "ir",
		GeoIPPath:   "/tmp/geoip-ir.srs",
		GeositePath: "/tmp/geosite-ir.srs",
	}

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809", Rules: rules, Geo: geo})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	route := root["route"].(map[string]any)
	routeRules := route["rules"].([]any)

	// Expected order:
	// [0] dns → dns-out
	// [1] ip_is_private → direct
	// [2] negated domain (forced.com) → proxy
	// [3] geosite-ir → direct
	// [4] geoip-ir → direct
	// [5] direct domain_suffix (*.ir) → direct
	// [6] catch-all → proxy
	if len(routeRules) != 7 {
		t.Fatalf("expected 7 route rules, got %d", len(routeRules))
	}

	// Negated rule comes before geo rules.
	r2 := routeRules[2].(map[string]any)
	if r2["outbound"] != "proxy" {
		t.Fatalf("rule[2] should be negated→proxy, got %v", r2)
	}

	// Geo rules come after negated, before normal direct.
	r3 := routeRules[3].(map[string]any)
	if r3["rule_set"] == nil {
		t.Fatalf("rule[3] should be a geo rule_set rule")
	}

	// Normal direct rules come after geo.
	r5 := routeRules[5].(map[string]any)
	if r5["outbound"] != "direct" || r5["domain_suffix"] == nil {
		t.Fatalf("rule[5] should be direct domain_suffix rule, got %v", r5)
	}
}

func TestSingboxNoGeoBypassNoRuleSetDefs(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"no-geo","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	route := root["route"].(map[string]any)
	if route["rule_set"] != nil {
		t.Fatal("route should not have rule_set when geo bypass is not set")
	}
}

func TestSingboxNoRoutingRulesDefaultBehavior(t *testing.T) {
	adapter := NewSingboxAdapter()
	payload := `{"ps":"no-rules","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	cfg, _, err := adapter.BuildConfig(ConfigBuildOpts{RawURI: link, SocksAddr: "127.0.0.1:10808", HTTPAddr: "127.0.0.1:10809"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	route := root["route"].(map[string]any)
	routeRules := route["rules"].([]any)

	// With nil rules: dns + ip_is_private + catch-all = 3 rules
	if len(routeRules) != 3 {
		t.Fatalf("expected 3 route rules with nil input, got %d", len(routeRules))
	}
}
