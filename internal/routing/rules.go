package routing

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// RuleType classifies a routing rule.
type RuleType string

const (
	RuleDomain RuleType = "domain"
	RuleIP     RuleType = "ip"
	RuleRange  RuleType = "range"
)

// Rule represents a single routing rule.
// Normal rules route traffic directly (bypass proxy).
// Negated rules force traffic through the proxy.
type Rule struct {
	Type     RuleType
	Value    string // domain, IP, or CIDR
	Negated  bool   // true = force proxy; false = direct
	Wildcard bool   // true for domain:*.ir style suffix matches
}

// ParseRule parses a single rule string in the format "type:value".
// Supported formats:
//   - domain:example.com       — exact domain, direct
//   - domain:*.ir              — suffix match, direct
//   - domain:!forced.com       — exact domain, force proxy
//   - domain:!*.blocked.net    — suffix match, force proxy
//   - ip:1.2.3.4              — exact IP, direct
//   - ip:!5.6.7.8             — exact IP, force proxy
//   - range:10.0.0.0/8        — CIDR, direct
//   - range:!100.64.0.0/10    — CIDR, force proxy
func ParseRule(s string) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "#") {
		return Rule{}, fmt.Errorf("empty or comment line")
	}

	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return Rule{}, fmt.Errorf("invalid rule format (missing ':'): %q", s)
	}
	typePart := strings.ToLower(strings.TrimSpace(s[:idx]))
	valuePart := strings.TrimSpace(s[idx+1:])
	if valuePart == "" {
		return Rule{}, fmt.Errorf("empty rule value: %q", s)
	}

	negated := false
	if strings.HasPrefix(valuePart, "!") {
		negated = true
		valuePart = strings.TrimSpace(valuePart[1:])
		if valuePart == "" {
			return Rule{}, fmt.Errorf("empty negated rule value: %q", s)
		}
	}

	switch RuleType(typePart) {
	case RuleDomain:
		return parseDomainRule(valuePart, negated)
	case RuleIP:
		return parseIPRule(valuePart, negated)
	case RuleRange:
		return parseRangeRule(valuePart, negated)
	default:
		return Rule{}, fmt.Errorf("unknown rule type %q in %q", typePart, s)
	}
}

func parseDomainRule(value string, negated bool) (Rule, error) {
	wildcard := false
	if strings.HasPrefix(value, "*.") {
		wildcard = true
		value = value[2:] // store "ir" for "*.ir"
	}
	if value == "" {
		return Rule{}, fmt.Errorf("empty domain value")
	}
	return Rule{
		Type:     RuleDomain,
		Value:    strings.ToLower(value),
		Negated:  negated,
		Wildcard: wildcard,
	}, nil
}

func parseIPRule(value string, negated bool) (Rule, error) {
	ip := net.ParseIP(value)
	if ip == nil {
		return Rule{}, fmt.Errorf("invalid IP address: %q", value)
	}
	return Rule{
		Type:    RuleIP,
		Value:   ip.String(),
		Negated: negated,
	}, nil
}

func parseRangeRule(value string, negated bool) (Rule, error) {
	_, cidr, err := net.ParseCIDR(value)
	if err != nil {
		return Rule{}, fmt.Errorf("invalid CIDR: %q: %w", value, err)
	}
	return Rule{
		Type:    RuleRange,
		Value:   cidr.String(),
		Negated: negated,
	}, nil
}

// ParseRules parses multiple rule strings (one per element).
// Empty lines and lines starting with '#' are skipped.
func ParseRules(lines []string) ([]Rule, error) {
	var rules []Rule
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r, err := ParseRule(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// ParseCSV parses a comma-separated list of rules.
func ParseCSV(csv string) ([]Rule, error) {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil, nil
	}
	parts := strings.Split(csv, ",")
	lines := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			lines = append(lines, p)
		}
	}
	return ParseRules(lines)
}

// LoadFromFile reads routing rules from a file (one rule per line).
func LoadFromFile(path string) ([]Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open routing rules file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read routing rules file: %w", err)
	}
	return ParseRules(lines)
}

// Split categorizes rules into direct and proxy (negated) groups.
func Split(rules []Rule) (direct, proxy []Rule) {
	for _, r := range rules {
		if r.Negated {
			proxy = append(proxy, r)
		} else {
			direct = append(direct, r)
		}
	}
	return direct, proxy
}

// DomainRules returns only domain-type rules.
func DomainRules(rules []Rule) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Type == RuleDomain {
			out = append(out, r)
		}
	}
	return out
}

// IPRules returns only ip-type rules.
func IPRules(rules []Rule) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Type == RuleIP {
			out = append(out, r)
		}
	}
	return out
}

// RangeRules returns only range-type (CIDR) rules.
func RangeRules(rules []Rule) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Type == RuleRange {
			out = append(out, r)
		}
	}
	return out
}

// IranDefaults returns the default routing rules for the Iran profile.
// Iranian domains are routed directly (bypass proxy) since they are
// accessible without a proxy and routing them through adds unnecessary latency.
var IranDefaults = []Rule{
	{Type: RuleDomain, Value: "ir", Wildcard: true},
	{Type: RuleDomain, Value: "digikala.com", Wildcard: true},
	{Type: RuleDomain, Value: "divar.ir", Wildcard: true},
	{Type: RuleDomain, Value: "shaparak.ir", Wildcard: true},
	{Type: RuleDomain, Value: "snapp.ir", Wildcard: true},
	{Type: RuleDomain, Value: "aparat.com", Wildcard: true},
	{Type: RuleDomain, Value: "filimo.com", Wildcard: true},
	{Type: RuleDomain, Value: "bale.ai", Wildcard: true},
	{Type: RuleDomain, Value: "rubika.ir", Wildcard: true},
	{Type: RuleDomain, Value: "irancell.ir", Wildcard: true},
	{Type: RuleDomain, Value: "mci.ir", Wildcard: true},
	{Type: RuleDomain, Value: "rightel.ir", Wildcard: true},
	// Iranian IP ranges — direct.
	{Type: RuleRange, Value: "5.52.0.0/14"},   // MTN Irancell
	{Type: RuleRange, Value: "2.176.0.0/12"},   // MCI
	{Type: RuleRange, Value: "91.92.0.0/16"},   // Pars Online
	{Type: RuleRange, Value: "31.40.0.0/16"},   // Shatel
	{Type: RuleRange, Value: "178.22.0.0/16"},  // Asiatech
	{Type: RuleRange, Value: "185.55.224.0/24"}, // Shaparak
}

// ProfileDefaults returns default routing rules for a given profile name.
// Returns nil for profiles with no defaults.
func ProfileDefaults(profileName string) []Rule {
	switch strings.ToLower(strings.TrimSpace(profileName)) {
	case "iran":
		out := make([]Rule, len(IranDefaults))
		copy(out, IranDefaults)
		return out
	default:
		return nil
	}
}
