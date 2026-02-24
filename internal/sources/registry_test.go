package sources

import "testing"

func TestDefaultRegistry(t *testing.T) {
	registry := DefaultRegistry()
	if len(registry) != 30 {
		t.Fatalf("expected 30 default sources, got %d", len(registry))
	}

	seen := make(map[string]struct{}, len(registry))
	for i, src := range registry {
		if src.URL == "" {
			t.Fatalf("source at index %d has empty URL", i)
		}
		if src.Category == "" {
			t.Fatalf("source at index %d has empty category", i)
		}
		if _, ok := seen[src.URL]; ok {
			t.Fatalf("duplicate URL in default registry: %s", src.URL)
		}
		seen[src.URL] = struct{}{}
	}
}
