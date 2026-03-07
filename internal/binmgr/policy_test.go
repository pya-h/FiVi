package binmgr

import "testing"

func TestSelectReleaseStableSkipsPrerelease(t *testing.T) {
	rels := []Release{
		{TagName: "v2.0.0-rc1", Prerelease: true, Assets: []Asset{{Name: "a"}}},
		{TagName: "v1.9.9", Assets: []Asset{{Name: "a"}}},
	}
	rel, err := selectRelease(rels, "stable", nil)
	if err != nil {
		t.Fatalf("selectRelease error: %v", err)
	}
	if rel.TagName != "v1.9.9" {
		t.Fatalf("got %q want v1.9.9", rel.TagName)
	}
}

func TestSelectReleaseLatestAllowsPrerelease(t *testing.T) {
	rels := []Release{
		{TagName: "v2.0.0-rc1", Prerelease: true, Assets: []Asset{{Name: "a"}}},
		{TagName: "v1.9.9", Assets: []Asset{{Name: "a"}}},
	}
	rel, err := selectRelease(rels, "latest", nil)
	if err != nil {
		t.Fatalf("selectRelease error: %v", err)
	}
	if rel.TagName != "v2.0.0-rc1" {
		t.Fatalf("got %q want v2.0.0-rc1", rel.TagName)
	}
}

func TestSelectReleaseAllowlist(t *testing.T) {
	rels := []Release{
		{TagName: "v1.9.9", Assets: []Asset{{Name: "a"}}},
		{TagName: "v1.9.8", Assets: []Asset{{Name: "a"}}},
	}
	rel, err := selectRelease(rels, "stable", map[string]bool{"v1.9.8": true})
	if err != nil {
		t.Fatalf("selectRelease error: %v", err)
	}
	if rel.TagName != "v1.9.8" {
		t.Fatalf("got %q want v1.9.8", rel.TagName)
	}
}

func TestSelectReleaseAllowlistNoMatch(t *testing.T) {
	rels := []Release{{TagName: "v1.9.9", Assets: []Asset{{Name: "a"}}}}
	if _, err := selectRelease(rels, "stable", map[string]bool{"v1.0.0": true}); err == nil {
		t.Fatal("expected error")
	}
}
