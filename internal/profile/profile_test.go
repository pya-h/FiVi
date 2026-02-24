package profile

import "testing"

func TestResolveIranDefault(t *testing.T) {
	got, err := Resolve("iran")
	if err != nil {
		t.Fatalf("Resolve(iran) error: %v", err)
	}
	if got.Name != "iran" {
		t.Fatalf("expected profile name iran, got %q", got.Name)
	}
	if len(got.Targets) == 0 {
		t.Fatalf("iran profile must include targets")
	}
	if len(got.RequiredTargetTokens) == 0 {
		t.Fatalf("iran profile must include required target tokens")
	}
}

func TestResolveUnknown(t *testing.T) {
	if _, err := Resolve("unknown-profile"); err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}

func TestNamesIncludesIran(t *testing.T) {
	names := Names()
	found := false
	for _, name := range names {
		if name == "iran" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected iran profile in Names()")
	}
}
