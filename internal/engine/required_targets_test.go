package engine

import (
	"testing"

	"fivi/internal/probe"
)

func TestHasRequiredTargetsOK(t *testing.T) {
	results := []probe.TargetResult{
		{URL: "https://x.com/", OK: true},
		{URL: "https://www.youtube.com/", OK: true},
		{URL: "https://www.cloudflare.com/cdn-cgi/trace", OK: false},
	}

	if !hasRequiredTargetsOK(results, []string{"x.com", "youtube.com"}) {
		t.Fatalf("expected required targets to pass")
	}
	if hasRequiredTargetsOK(results, []string{"x.com", "cloudflare.com"}) {
		t.Fatalf("expected required target set to fail")
	}
	if !hasRequiredTargetsOK(results, nil) {
		t.Fatalf("nil required tokens should pass")
	}
}
