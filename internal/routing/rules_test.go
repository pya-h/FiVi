package routing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRuleDomainExact(t *testing.T) {
	r, err := ParseRule("domain:example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleDomain || r.Value != "example.com" || r.Negated || r.Wildcard {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleDomainWildcard(t *testing.T) {
	r, err := ParseRule("domain:*.ir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleDomain || r.Value != "ir" || r.Negated || !r.Wildcard {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleDomainNegated(t *testing.T) {
	r, err := ParseRule("domain:!forced.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleDomain || r.Value != "forced.com" || !r.Negated || r.Wildcard {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleDomainNegatedWildcard(t *testing.T) {
	r, err := ParseRule("domain:!*.blocked.net")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleDomain || r.Value != "blocked.net" || !r.Negated || !r.Wildcard {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleIP(t *testing.T) {
	r, err := ParseRule("ip:1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleIP || r.Value != "1.2.3.4" || r.Negated {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleIPv6(t *testing.T) {
	r, err := ParseRule("ip:2001:db8::1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleIP || r.Value != "2001:db8::1" || r.Negated {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleIPNegated(t *testing.T) {
	r, err := ParseRule("ip:!5.6.7.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleIP || r.Value != "5.6.7.8" || !r.Negated {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleRange(t *testing.T) {
	r, err := ParseRule("range:10.0.0.0/8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleRange || r.Value != "10.0.0.0/8" || r.Negated {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleRangeNegated(t *testing.T) {
	r, err := ParseRule("range:!100.64.0.0/10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleRange || r.Value != "100.64.0.0/10" || !r.Negated {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleCaseInsensitiveType(t *testing.T) {
	r, err := ParseRule("DOMAIN:Example.COM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != RuleDomain || r.Value != "example.com" {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseRuleErrors(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"", "empty string"},
		{"# comment", "comment line"},
		{"nodomain", "missing colon"},
		{"domain:", "empty value"},
		{"domain:!", "empty negated value"},
		{"ip:notanip", "invalid IP"},
		{"range:notacidr", "invalid CIDR"},
		{"unknown:value", "unknown type"},
	}
	for _, tt := range tests {
		_, err := ParseRule(tt.input)
		if err == nil {
			t.Errorf("expected error for %q (%s), got nil", tt.input, tt.desc)
		}
	}
}

func TestParseRules(t *testing.T) {
	lines := []string{
		"# Comment line",
		"domain:example.com",
		"",
		"domain:*.ir",
		"ip:1.2.3.4",
		"range:10.0.0.0/8",
		"domain:!forced.com",
	}
	rules, err := ParseRules(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 5 {
		t.Fatalf("expected 5 rules, got %d", len(rules))
	}
	// Verify negated rule
	if !rules[4].Negated || rules[4].Value != "forced.com" {
		t.Fatalf("expected negated forced.com, got %+v", rules[4])
	}
}

func TestParseRulesError(t *testing.T) {
	lines := []string{
		"domain:ok.com",
		"badformat",
	}
	_, err := ParseRules(lines)
	if err == nil {
		t.Fatal("expected error for bad format line")
	}
}

func TestParseCSV(t *testing.T) {
	csv := "domain:a.com, domain:*.ir, ip:1.2.3.4"
	rules, err := ParseCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
}

func TestParseCSVEmpty(t *testing.T) {
	rules, err := ParseCSV("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.txt")
	content := "# Routing rules\ndomain:example.com\ndomain:*.ir\nip:8.8.8.8\nrange:10.0.0.0/8\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	rules, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(rules))
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/rules.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSplit(t *testing.T) {
	rules := []Rule{
		{Type: RuleDomain, Value: "example.com", Negated: false},
		{Type: RuleDomain, Value: "forced.com", Negated: true},
		{Type: RuleIP, Value: "1.2.3.4", Negated: false},
		{Type: RuleRange, Value: "10.0.0.0/8", Negated: true},
	}
	direct, proxy := Split(rules)
	if len(direct) != 2 {
		t.Fatalf("expected 2 direct, got %d", len(direct))
	}
	if len(proxy) != 2 {
		t.Fatalf("expected 2 proxy, got %d", len(proxy))
	}
}

func TestDomainRules(t *testing.T) {
	rules := []Rule{
		{Type: RuleDomain, Value: "a.com"},
		{Type: RuleIP, Value: "1.2.3.4"},
		{Type: RuleDomain, Value: "b.com"},
	}
	got := DomainRules(rules)
	if len(got) != 2 {
		t.Fatalf("expected 2 domain rules, got %d", len(got))
	}
}

func TestIPRulesFilter(t *testing.T) {
	rules := []Rule{
		{Type: RuleDomain, Value: "a.com"},
		{Type: RuleIP, Value: "1.2.3.4"},
		{Type: RuleRange, Value: "10.0.0.0/8"},
	}
	got := IPRules(rules)
	if len(got) != 1 {
		t.Fatalf("expected 1 IP rule, got %d", len(got))
	}
}

func TestRangeRulesFilter(t *testing.T) {
	rules := []Rule{
		{Type: RuleRange, Value: "10.0.0.0/8"},
		{Type: RuleDomain, Value: "a.com"},
		{Type: RuleRange, Value: "172.16.0.0/12"},
	}
	got := RangeRules(rules)
	if len(got) != 2 {
		t.Fatalf("expected 2 range rules, got %d", len(got))
	}
}

func TestProfileDefaultsIran(t *testing.T) {
	rules := ProfileDefaults("iran")
	if len(rules) == 0 {
		t.Fatal("expected non-empty iran defaults")
	}
	// First rule should be *.ir wildcard
	if rules[0].Type != RuleDomain || rules[0].Value != "ir" || !rules[0].Wildcard {
		t.Fatalf("expected first rule to be domain:*.ir, got %+v", rules[0])
	}
	// Verify we have both domain and range rules
	hasDomain, hasRange := false, false
	for _, r := range rules {
		if r.Type == RuleDomain {
			hasDomain = true
		}
		if r.Type == RuleRange {
			hasRange = true
		}
	}
	if !hasDomain || !hasRange {
		t.Fatal("expected both domain and range rules in iran defaults")
	}
}

func TestProfileDefaultsGlobal(t *testing.T) {
	rules := ProfileDefaults("global")
	if rules != nil {
		t.Fatalf("expected nil for global profile, got %d rules", len(rules))
	}
}

func TestProfileDefaultsUnknown(t *testing.T) {
	rules := ProfileDefaults("nonexistent")
	if rules != nil {
		t.Fatalf("expected nil for unknown profile, got %d rules", len(rules))
	}
}

func TestCIDRNormalization(t *testing.T) {
	// range:10.1.2.3/8 should normalize to 10.0.0.0/8
	r, err := ParseRule("range:10.1.2.3/8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Value != "10.0.0.0/8" {
		t.Fatalf("expected CIDR normalized to 10.0.0.0/8, got %s", r.Value)
	}
}

func TestWhitespaceHandling(t *testing.T) {
	r, err := ParseRule("  domain : example.com  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Value != "example.com" {
		t.Fatalf("expected example.com, got %q", r.Value)
	}
}
