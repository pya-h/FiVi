package binmgr

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseChecksumForAsset(t *testing.T) {
	asset := "Xray-linux-64.zip"
	hash := strings.Repeat("a", 64)

	v1 := []byte("SHA256 (Xray-linux-64.zip) = " + hash + "\n")
	got, err := parseChecksumForAsset(v1, asset)
	if err != nil {
		t.Fatalf("parse v1 failed: %v", err)
	}
	if got != hash {
		t.Fatalf("hash mismatch: got %q want %q", got, hash)
	}

	v2 := []byte(hash + "  " + asset + "\n")
	got, err = parseChecksumForAsset(v2, asset)
	if err != nil {
		t.Fatalf("parse v2 failed: %v", err)
	}
	if got != hash {
		t.Fatalf("hash mismatch: got %q want %q", got, hash)
	}
}

func TestPickArchiveAsset(t *testing.T) {
	assets := []Asset{
		{Name: "checksums.txt"},
		{Name: "Xray-linux-arm64-v8a.zip"},
		{Name: "Xray-linux-64.zip"},
	}

	asset, err := pickArchiveAsset("xray", "linux", "amd64", assets)
	if err != nil {
		t.Fatalf("pick archive failed: %v", err)
	}
	if asset.Name != "Xray-linux-64.zip" {
		t.Fatalf("picked %q", asset.Name)
	}
}

func TestInstallEngine_XrayEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path/executable mode differs on windows")
	}
	archiveName := "Xray-linux-64.zip"
	archive := buildZipArchive(t, "xray", []byte("xray-binary"))
	h := sha256.Sum256(archive)
	hash := hex.EncodeToString(h[:])
	checksum := []byte("SHA256 (" + archiveName + ") = " + hash + "\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xray/latest":
			rel := Release{
				TagName: "v1.2.3",
				Assets: []Asset{
					{Name: archiveName, URL: serverURL(r, "/assets/xray.zip")},
					{Name: "Xray-linux-64.zip.dgst", URL: serverURL(r, "/assets/xray.dgst")},
				},
			}
			_ = json.NewEncoder(w).Encode(rel)
		case "/assets/xray.zip":
			_, _ = w.Write(archive)
		case "/assets/xray.dgst":
			_, _ = w.Write(checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	installer := &Installer{
		Client:             &http.Client{Timeout: 5 * time.Second},
		InstallRoot:        t.TempDir(),
		XrayReleasesURL:    server.URL + "/xray/latest",
		SingboxReleasesURL: server.URL + "/singbox/latest",
	}

	res, err := installer.InstallEngine(context.Background(), "xray")
	if err != nil {
		t.Fatalf("InstallEngine failed: %v", err)
	}
	if res.Engine != "xray" {
		t.Fatalf("engine mismatch: %q", res.Engine)
	}
	if res.Version != "v1.2.3" {
		t.Fatalf("version mismatch: %q", res.Version)
	}

	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(body) != "xray-binary" {
		t.Fatalf("binary content mismatch: %q", string(body))
	}

	wantPath := filepath.Join(installer.InstallRoot, "xray", runtime.GOARCH, "xray")
	if res.Path != wantPath {
		t.Fatalf("path mismatch: got %q want %q", res.Path, wantPath)
	}
}

func TestInstallEngine_SingboxEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path/executable mode differs on windows")
	}
	archiveName := "sing-box-1.2.3-linux-amd64.tar.gz"
	archive := buildTarGzArchive(t, "sing-box", []byte("singbox-binary"))
	h := sha256.Sum256(archive)
	hash := hex.EncodeToString(h[:])
	checksum := []byte(hash + "  " + archiveName + "\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/singbox/latest":
			rel := Release{
				TagName: "v9.9.9",
				Assets: []Asset{
					{Name: archiveName, URL: serverURL(r, "/assets/sb.tar.gz")},
					{Name: "sing-box-1.2.3-sha256sum.txt", URL: serverURL(r, "/assets/sb.sha256")},
				},
			}
			_ = json.NewEncoder(w).Encode(rel)
		case "/assets/sb.tar.gz":
			_, _ = w.Write(archive)
		case "/assets/sb.sha256":
			_, _ = w.Write(checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	installer := &Installer{
		Client:             &http.Client{Timeout: 5 * time.Second},
		InstallRoot:        t.TempDir(),
		XrayReleasesURL:    server.URL + "/xray/latest",
		SingboxReleasesURL: server.URL + "/singbox/latest",
	}

	res, err := installer.InstallEngine(context.Background(), "singbox")
	if err != nil {
		t.Fatalf("InstallEngine failed: %v", err)
	}
	if res.Engine != "singbox" {
		t.Fatalf("engine mismatch: %q", res.Engine)
	}
	if res.Version != "v9.9.9" {
		t.Fatalf("version mismatch: %q", res.Version)
	}

	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(body) != "singbox-binary" {
		t.Fatalf("binary content mismatch: %q", string(body))
	}

	wantPath := filepath.Join(installer.InstallRoot, "singbox", runtime.GOARCH, "sing-box")
	if res.Path != wantPath {
		t.Fatalf("path mismatch: got %q want %q", res.Path, wantPath)
	}
}

func serverURL(r *http.Request, path string) string {
	return "http://" + r.Host + path
}

func buildZipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)
	fw, err := zw.Create(filepath.Join("pkg", name))
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func buildTarGzArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	buf := bytes.NewBuffer(nil)
	gzw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gzw)
	hdr := &tar.Header{
		Name: filepath.Join("pkg", name),
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
