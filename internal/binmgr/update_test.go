package binmgr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

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

func TestUpdateAll_ClearsResolveErrorAfterSuccessfulInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses unix permission semantics")
	}

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	badXray := filepath.Join(tmpHome, ".fivi", "bins", "xray", runtime.GOARCH, "xray")
	if err := os.MkdirAll(filepath.Dir(badXray), 0o755); err != nil {
		t.Fatalf("mkdir bad xray: %v", err)
	}
	if err := os.WriteFile(badXray, []byte("not-executable"), 0o644); err != nil {
		t.Fatalf("write bad xray: %v", err)
	}

	xrayArchiveName := "Xray-linux-64.zip"
	if runtime.GOARCH == "arm64" {
		xrayArchiveName = "Xray-linux-arm64-v8a.zip"
	}
	xrayArchive := buildZipArchive(t, "xray", []byte("xray-new"))
	xrayHash := sha256.Sum256(xrayArchive)
	xrayDigest := "SHA256 (" + xrayArchiveName + ") = " + hex.EncodeToString(xrayHash[:]) + "\n"

	singArchiveName := "sing-box-1.2.3-linux-" + runtime.GOARCH + ".tar.gz"
	singArchive := buildTarGzArchive(t, "sing-box", []byte("singbox-new"))
	singHash := sha256.Sum256(singArchive)
	singDigest := hex.EncodeToString(singHash[:]) + "  " + singArchiveName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xray/latest":
			rel := Release{TagName: "v9.0.0", Assets: []Asset{
				{Name: xrayArchiveName, URL: "http://" + r.Host + "/assets/xray.zip"},
				{Name: "Xray-linux-64.zip.dgst", URL: "http://" + r.Host + "/assets/xray.dgst"},
			}}
			_ = json.NewEncoder(w).Encode(rel)
		case "/sing/latest":
			rel := Release{TagName: "v1.2.3", Assets: []Asset{
				{Name: singArchiveName, URL: "http://" + r.Host + "/assets/sing.tar.gz"},
				{Name: "sing-box-1.2.3-sha256sum.txt", URL: "http://" + r.Host + "/assets/sing.sha256"},
			}}
			_ = json.NewEncoder(w).Encode(rel)
		case "/assets/xray.zip":
			_, _ = w.Write(xrayArchive)
		case "/assets/xray.dgst":
			_, _ = w.Write([]byte(xrayDigest))
		case "/assets/sing.tar.gz":
			_, _ = w.Write(singArchive)
		case "/assets/sing.sha256":
			_, _ = w.Write([]byte(singDigest))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	installer := &Installer{
		Client:             &http.Client{Timeout: 5 * time.Second},
		InstallRoot:        filepath.Join(tmpHome, ".fivi", "bins"),
		XrayReleasesURL:    server.URL + "/xray/latest",
		SingboxReleasesURL: server.URL + "/sing/latest",
	}

	statuses, err := installer.UpdateAll(context.Background())
	if err != nil {
		t.Fatalf("UpdateAll error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	for _, st := range statuses {
		if st.Err != nil {
			t.Fatalf("unexpected status error for %s: %v", st.Engine, st.Err)
		}
		if !st.Updated {
			t.Fatalf("expected %s to be updated", st.Engine)
		}
	}
}
