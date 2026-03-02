package session

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadFreshState(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.json")
	now := time.Now().UTC()

	state := State{
		RawURI:  "vmess://example",
		Score:   123.45,
		SavedAt: now,
	}
	if err := Save(path, state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, ok, err := Load(path, time.Hour, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected fresh session")
	}
	if loaded.RawURI != state.RawURI {
		t.Fatalf("raw mismatch: got %q want %q", loaded.RawURI, state.RawURI)
	}
	if loaded.Score != state.Score {
		t.Fatalf("score mismatch: got %v want %v", loaded.Score, state.Score)
	}
}

func TestLoadExpiredStateReturnsNotFresh(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.json")
	now := time.Now().UTC()

	state := State{
		RawURI:  "vless://example",
		Score:   99,
		SavedAt: now.Add(-2 * time.Hour),
	}
	if err := Save(path, state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, ok, err := Load(path, time.Hour, now)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ok {
		t.Fatalf("expected stale session")
	}
	if loaded.RawURI != state.RawURI {
		t.Fatalf("raw mismatch for stale load")
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "missing-session.json")

	_, ok, err := Load(path, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected not found to return ok=false")
	}
}
