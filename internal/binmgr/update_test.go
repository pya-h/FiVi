package binmgr

import "testing"

func TestParseVersionToken(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "xray output", out: "Xray 25.3.6 (Xray, Penetrates Everything.)", want: "v25.3.6"},
		{name: "singbox output", out: "sing-box version 1.11.0", want: "v1.11.0"},
		{name: "already prefixed", out: "version: v1.10.1", want: "v1.10.1"},
		{name: "none", out: "unknown", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseVersionToken(tc.out); got != tc.want {
				t.Fatalf("parseVersionToken(%q) = %q, want %q", tc.out, got, tc.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		{a: "v1.2.3", b: "v1.2.3", want: 0},
		{a: "v1.2.3", b: "v1.2.4", want: -1},
		{a: "v1.10.0", b: "v1.9.9", want: 1},
		{a: "v25.3.6", b: "v25.3.5", want: 1},
		{a: "v1.2", b: "v1.2.0", want: 0},
	}
	for _, tc := range tests {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("compareVersions(%q, %q)=%d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestShouldUpdate(t *testing.T) {
	tests := []struct {
		installed string
		latest    string
		want      bool
	}{
		{installed: "", latest: "v1.0.0", want: true},
		{installed: "v1.0.0", latest: "v1.0.1", want: true},
		{installed: "v1.0.1", latest: "v1.0.1", want: false},
		{installed: "v1.1.0", latest: "v1.0.9", want: false},
		{installed: "v1.0.0", latest: "", want: false},
	}
	for _, tc := range tests {
		if got := shouldUpdate(tc.installed, tc.latest); got != tc.want {
			t.Fatalf("shouldUpdate(%q, %q)=%v, want %v", tc.installed, tc.latest, got, tc.want)
		}
	}
}
