package sysproxy

import (
	"strings"
	"testing"
)

func TestBuildWindowsSetScript(t *testing.T) {
	script := BuildWindowsSetScript("127.0.0.1", 10808, "127.0.0.1", 10809)
	checks := []string{
		"HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings",
		"-Name ProxyServer",
		"http=127.0.0.1:10809;https=127.0.0.1:10809;socks=127.0.0.1:10808",
		"-Name ProxyOverride",
		"-Name ProxyEnable",
		"-Type DWord -Value 1",
	}
	for _, needle := range checks {
		if !strings.Contains(script, needle) {
			t.Fatalf("script missing %q: %s", needle, script)
		}
	}
}

func TestBuildWindowsRestoreScript(t *testing.T) {
	prev := windowsProxyState{
		ProxyEnable:   0,
		ProxyServer:   "http=old:80;socks=old:1080",
		ProxyOverride: "localhost",
	}
	script := BuildWindowsRestoreScript(prev)
	if !strings.Contains(script, "-Name ProxyEnable") || !strings.Contains(script, "-Value 0") {
		t.Fatalf("unexpected restore enable script: %s", script)
	}
	if !strings.Contains(script, prev.ProxyServer) {
		t.Fatalf("restore script missing ProxyServer value: %s", script)
	}
}

func TestEscapePS(t *testing.T) {
	if got := escapePS("a'b"); got != "a''b" {
		t.Fatalf("escapePS=%q", got)
	}
}
