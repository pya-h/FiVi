package config

import (
	"os"
	"reflect"
	"testing"

	"fivi/internal/sources"
)

func TestDefaultSourcesUsesRegistry(t *testing.T) {
	got := DefaultSources()
	want := sources.DefaultURLs()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default sources do not match registry\n got=%v\nwant=%v", got, want)
	}
}

func TestNormalizeEngine(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "xray", want: "xray"},
		{in: "XRAY", want: "xray"},
		{in: "sing-box", want: "singbox"},
		{in: "singbox", want: "singbox"},
	}

	for _, tc := range tests {
		if got := normalizeEngine(tc.in); got != tc.want {
			t.Fatalf("normalizeEngine(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "proxy", want: "proxy"},
		{in: "tunnel", want: "tunnel"},
		{in: "TUNNEL", want: "tunnel"},
		{in: "PROXY", want: "proxy"},
		{in: "invalid", want: "tunnel"},
		{in: "", want: "tunnel"},
	}

	for _, tc := range tests {
		if got := normalizeMode(tc.in); got != tc.want {
			t.Fatalf("normalizeMode(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveBinaryEnvOverride(t *testing.T) {
	const key = "FIVI_SINGBOX_BIN"
	original, hadOriginal := os.LookupEnv(key)
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv(key, original)
			return
		}
		_ = os.Unsetenv(key)
	})

	_ = os.Setenv(key, "/tmp/sbox")
	got := resolveBinary("singbox", "", key, false)
	if got != "/tmp/sbox" {
		t.Fatalf("resolveBinary() env override failed, got=%q", got)
	}
}

func TestResolveBinaryExplicitFlag(t *testing.T) {
	got := resolveBinary("xray", "/my/custom/xray", "FIVI_XRAY_BIN", true)
	if got != "/my/custom/xray" {
		t.Fatalf("resolveBinary() explicit flag failed, got=%q", got)
	}
}
