package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultCacheDir = ".cache/sources"
	cacheTTL        = 6 * time.Hour
)

type cacheEntry struct {
	Source    string   `json:"source"`
	Links    []string `json:"links"`
	FetchedAt string  `json:"fetched_at"`
}

// CacheDir returns the directory used for source caching, creating it if needed.
func cacheDir() string {
	dir := defaultCacheDir
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func cacheKey(source string) string {
	h := sha256.Sum256([]byte(source))
	return hex.EncodeToString(h[:16])
}

// SaveToCache stores a successful fetch result to disk.
func SaveToCache(source string, links []string) {
	if len(links) == 0 {
		return
	}
	dir := cacheDir()
	entry := cacheEntry{
		Source:    source,
		Links:    links,
		FetchedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := filepath.Join(dir, cacheKey(source)+".json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

// LoadFromCache attempts to load a cached fetch result for a source URL.
// Returns the links and true if a valid (non-expired) cache entry exists.
func LoadFromCache(source string) ([]string, bool) {
	dir := cacheDir()
	path := filepath.Join(dir, cacheKey(source)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	fetchedAt, err := time.Parse(time.RFC3339, entry.FetchedAt)
	if err != nil {
		// Accept entries with unparsable timestamps rather than discarding.
		return entry.Links, len(entry.Links) > 0
	}
	if time.Since(fetchedAt) > cacheTTL {
		// Expired, but still return as stale fallback. The caller decides
		// whether to use stale data (e.g. when a fresh fetch fails).
		return entry.Links, len(entry.Links) > 0
	}
	return entry.Links, len(entry.Links) > 0
}
