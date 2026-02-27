package parser

import "testing"

func TestNormalizeDedupsAndTruncates(t *testing.T) {
	input := []string{
		"vmess://z",
		"ss://a",
		"trojan://a",
		"vless://a",
		"vmess://z", // duplicate
	}
	got := Normalize(input, map[string]bool{
		"vmess":  true,
		"vless":  true,
		"trojan": true,
		"ss":     true,
	}, 2)

	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(got))
	}
	// With shuffle, order is random but all candidates should be valid.
	seen := map[string]bool{}
	for _, c := range got {
		if c.Scheme == "" {
			t.Fatalf("candidate has empty scheme: %+v", c)
		}
		if seen[c.Raw] {
			t.Fatalf("duplicate candidate after normalize: %s", c.Raw)
		}
		seen[c.Raw] = true
	}
}

func TestNormalizeNoLimitReturnsAll(t *testing.T) {
	input := []string{
		"vmess://a",
		"vless://b",
		"trojan://c",
		"ss://d",
	}
	got := Normalize(input, nil, 0)
	if len(got) != 4 {
		t.Fatalf("expected 4 candidates with no limit, got %d", len(got))
	}
}

func TestNormalizeFiltersSchemes(t *testing.T) {
	input := []string{
		"vmess://a",
		"vless://b",
		"trojan://c",
		"ss://d",
	}
	got := Normalize(input, map[string]bool{"vmess": true}, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	if got[0].Scheme != "vmess" {
		t.Fatalf("expected vmess, got %s", got[0].Scheme)
	}
}
