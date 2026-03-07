package config

import (
	"os"
	"path/filepath"
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
		{in: "hysteria2", want: "hysteria2"},
		{in: "hy2", want: "hysteria2"},
		{in: "hysteria-2", want: "hysteria2"},
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
		{in: "invalid", want: "proxy"},
		{in: "", want: "proxy"},
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

func TestSharedListenAddr(t *testing.T) {
	if got := sharedListenAddr("127.0.0.1:10808"); got != "0.0.0.0:10808" {
		t.Fatalf("sharedListenAddr()=%q", got)
	}
	if got := sharedListenAddr("bad-addr"); got != "bad-addr" {
		t.Fatalf("sharedListenAddr() should keep invalid addr, got=%q", got)
	}
}

func TestReadConfigLines(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "configs.txt")
	content := "\n# comment\n vmess://a\n\n vless://b \n#x\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := readConfigLines(path)
	if err != nil {
		t.Fatalf("readConfigLines: %v", err)
	}
	want := []string{"vmess://a", "vless://b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestUniqueStrings(t *testing.T) {
	got := uniqueStrings([]string{" vmess://a ", "", "vless://b", "vmess://a"})
	want := []string{"vmess://a", "vless://b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}
