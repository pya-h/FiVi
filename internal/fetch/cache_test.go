package fetch

import (
	"testing"
)

func TestCacheRoundTrip(t *testing.T) {
	// Use a temporary directory for cache.
	tmpDir := t.TempDir()
	t.Setenv("FIVI_CACHE_DIR", tmpDir)

	source := "https://example.com/test-source"
	links := []string{"vmess://aaa", "vless://bbb"}

	SaveToCache(source, links)

	got, ok := LoadFromCache(source)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != len(links) {
		t.Fatalf("expected %d links, got %d", len(links), len(got))
	}
	for i, l := range got {
		if l != links[i] {
			t.Fatalf("link mismatch at %d: got=%q want=%q", i, l, links[i])
		}
	}
}

func TestCacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FIVI_CACHE_DIR", tmpDir)

	_, ok := LoadFromCache("https://example.com/nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestCacheEmptyLinksNotSaved(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FIVI_CACHE_DIR", tmpDir)

	SaveToCache("https://example.com/empty", nil)
	_, ok := LoadFromCache("https://example.com/empty")
	if ok {
		t.Fatal("expected cache miss for empty links")
	}
}
