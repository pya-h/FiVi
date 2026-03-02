package binresolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFindsLocalBinary(t *testing.T) {
	// Create a temporary bins structure.
	tmpDir := t.TempDir()
	arch := mapArch("amd64")
	binDir := filepath.Join(tmpDir, "bins", "xray", arch)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "xray")
	if err := os.WriteFile(binPath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Change to tmpDir so relative probe finds it.
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("xray")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if filepath.Base(got) != "xray" {
		t.Fatalf("expected xray binary, got %q", got)
	}
}

func TestResolveUnknownEngine(t *testing.T) {
	_, err := Resolve("unknown-engine")
	if err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestResolveFindsHomeInstalledBinary(t *testing.T) {
	tmpHome := t.TempDir()
	arch := mapArch("amd64")
	binDir := filepath.Join(tmpHome, ".fivi", "bins", "xray", arch)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "xray")
	if err := os.WriteFile(binPath, []byte("fake-home"), 0o755); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("xray")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if got != binPath {
		t.Fatalf("expected %q, got %q", binPath, got)
	}
}

func TestMapArch(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"amd64", "amd64"},
		{"arm64", "arm64"},
		{"386", "386"},
	}
	for _, tc := range tests {
		if got := mapArch(tc.in); got != tc.want {
			t.Fatalf("mapArch(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
