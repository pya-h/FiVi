package routing

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSupportedGeoCountries(t *testing.T) {
	countries := SupportedGeoCountries()
	if len(countries) == 0 {
		t.Fatal("expected at least one supported country")
	}
	found := false
	for _, c := range countries {
		if c == "ir" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'ir' in supported countries")
	}
}

func TestEnsureGeoRuleSetsUnsupported(t *testing.T) {
	_, err := EnsureGeoRuleSets("xx", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unsupported country")
	}
}

func TestEnsureGeoRuleSetsDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-srs-data"))
	}))
	defer srv.Close()

	origSources := geoSources
	geoSources = map[string]geoSource{
		"test": {
			GeoIPURL:   srv.URL + "/geoip-test.srs",
			GeositeURL: srv.URL + "/geosite-test.srs",
		},
	}
	defer func() { geoSources = origSources }()

	dir := t.TempDir()
	rs, err := EnsureGeoRuleSets("test", dir)
	if err != nil {
		t.Fatalf("EnsureGeoRuleSets error: %v", err)
	}
	if rs.Country != "test" {
		t.Fatalf("country=%q, want test", rs.Country)
	}
	if rs.GeoIPPath != filepath.Join(dir, "geoip-test.srs") {
		t.Fatalf("GeoIPPath=%q, want %q", rs.GeoIPPath, filepath.Join(dir, "geoip-test.srs"))
	}
	if rs.GeositePath != filepath.Join(dir, "geosite-test.srs") {
		t.Fatalf("GeositePath=%q, want %q", rs.GeositePath, filepath.Join(dir, "geosite-test.srs"))
	}

	data, err := os.ReadFile(rs.GeoIPPath)
	if err != nil {
		t.Fatalf("read geoip file: %v", err)
	}
	if string(data) != "fake-srs-data" {
		t.Fatalf("geoip content=%q, want fake-srs-data", string(data))
	}
}

func TestEnsureGeoRuleSetsCache(t *testing.T) {
	downloads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		_, _ = w.Write([]byte("srs-data"))
	}))
	defer srv.Close()

	origSources := geoSources
	geoSources = map[string]geoSource{
		"test": {
			GeoIPURL:   srv.URL + "/geoip.srs",
			GeositeURL: srv.URL + "/geosite.srs",
		},
	}
	defer func() { geoSources = origSources }()

	dir := t.TempDir()

	// First call: downloads both files.
	_, err := EnsureGeoRuleSets("test", dir)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if downloads != 2 {
		t.Fatalf("expected 2 downloads, got %d", downloads)
	}

	// Second call: uses cache (files are fresh).
	_, err = EnsureGeoRuleSets("test", dir)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if downloads != 2 {
		t.Fatalf("expected 2 downloads (cached), got %d", downloads)
	}
}

func TestEnsureGeoRuleSetsStaleRefresh(t *testing.T) {
	downloads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		_, _ = w.Write([]byte("new-data"))
	}))
	defer srv.Close()

	origSources := geoSources
	geoSources = map[string]geoSource{
		"test": {
			GeoIPURL:   srv.URL + "/geoip.srs",
			GeositeURL: srv.URL + "/geosite.srs",
		},
	}
	defer func() { geoSources = origSources }()

	dir := t.TempDir()

	// Create stale files (8 days old).
	geoipFile := filepath.Join(dir, "geoip-test.srs")
	geositeFile := filepath.Join(dir, "geosite-test.srs")
	staleTime := time.Now().Add(-8 * 24 * time.Hour)
	_ = os.WriteFile(geoipFile, []byte("old"), 0o644)
	_ = os.WriteFile(geositeFile, []byte("old"), 0o644)
	_ = os.Chtimes(geoipFile, staleTime, staleTime)
	_ = os.Chtimes(geositeFile, staleTime, staleTime)

	// Call should re-download.
	_, err := EnsureGeoRuleSets("test", dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if downloads != 2 {
		t.Fatalf("expected 2 downloads for stale files, got %d", downloads)
	}

	// Verify new content.
	data, _ := os.ReadFile(geoipFile)
	if string(data) != "new-data" {
		t.Fatalf("expected refreshed content, got %q", string(data))
	}
}

func TestIsGeoFileFresh(t *testing.T) {
	// Non-existent file.
	if isGeoFileFresh("/nonexistent/file/path") {
		t.Fatal("non-existent file should not be fresh")
	}

	// Fresh file (just created).
	f := filepath.Join(t.TempDir(), "test.srs")
	_ = os.WriteFile(f, []byte("data"), 0o644)
	if !isGeoFileFresh(f) {
		t.Fatal("newly created file should be fresh")
	}

	// Stale file (8 days old).
	stale := time.Now().Add(-8 * 24 * time.Hour)
	_ = os.Chtimes(f, stale, stale)
	if isGeoFileFresh(f) {
		t.Fatal("8-day-old file should not be fresh")
	}
}

func TestDefaultGeoCacheDir(t *testing.T) {
	dir, err := DefaultGeoCacheDir()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty cache dir")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("expected absolute path")
	}
}

func TestDownloadGeoFileHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "test.srs")
	err := downloadGeoFile(path, srv.URL+"/missing.srs")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}

	// File should not exist.
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("file should not exist after failed download")
	}
}
