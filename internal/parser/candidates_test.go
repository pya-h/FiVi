package parser

import "testing"

func TestNormalizeSortsBeforeMaxCutoff(t *testing.T) {
	input := []string{
		"vmess://z",
		"ss://a",
		"trojan://a",
		"vless://a",
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
	if got[0].Scheme != "ss" || got[1].Scheme != "trojan" {
		t.Fatalf("unexpected candidate order/cutoff: %+v", got)
	}
}
