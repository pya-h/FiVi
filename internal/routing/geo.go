package routing

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// GeoRuleSetMaxAge is the freshness threshold for cached rule set files.
	GeoRuleSetMaxAge = 7 * 24 * time.Hour
	geoCacheDirName  = ".fivi/rulesets"
)

// GeoRuleSets holds paths to downloaded geo rule set files for singbox.
type GeoRuleSets struct {
	Country     string
	GeoIPPath   string // absolute path to geoip-{cc}.srs
	GeositePath string // absolute path to geosite-{cc}.srs
}

type geoSource struct {
	GeoIPURL   string
	GeositeURL string
}

// geoSources maps country codes to rule set download URLs.
// Currently supports Iran via Chocolate4U/Iran-sing-box-rules.
var geoSources = map[string]geoSource{
	"ir": {
		GeoIPURL:   "https://github.com/Chocolate4U/Iran-sing-box-rules/releases/latest/download/geoip-ir.srs",
		GeositeURL: "https://github.com/Chocolate4U/Iran-sing-box-rules/releases/latest/download/geosite-ir.srs",
	},
}

// SupportedGeoCountries returns the sorted list of supported country codes.
func SupportedGeoCountries() []string {
	out := make([]string, 0, len(geoSources))
	for cc := range geoSources {
		out = append(out, cc)
	}
	sort.Strings(out)
	return out
}

// DefaultGeoCacheDir returns the default cache directory for geo rule sets (~/.fivi/rulesets/).
func DefaultGeoCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, geoCacheDirName), nil
}

// EnsureGeoRuleSets downloads/caches .srs rule set files for the given country.
// Files are cached in cacheDir and refreshed after GeoRuleSetMaxAge.
func EnsureGeoRuleSets(country, cacheDir string) (*GeoRuleSets, error) {
	country = strings.ToLower(strings.TrimSpace(country))
	src, ok := geoSources[country]
	if !ok {
		return nil, fmt.Errorf("unsupported geo-bypass country %q (supported: %v)", country, SupportedGeoCountries())
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create geo cache dir: %w", err)
	}

	geoipFile := filepath.Join(cacheDir, fmt.Sprintf("geoip-%s.srs", country))
	geositeFile := filepath.Join(cacheDir, fmt.Sprintf("geosite-%s.srs", country))

	if err := ensureGeoFile(geoipFile, src.GeoIPURL); err != nil {
		return nil, fmt.Errorf("geoip rule set: %w", err)
	}
	if err := ensureGeoFile(geositeFile, src.GeositeURL); err != nil {
		return nil, fmt.Errorf("geosite rule set: %w", err)
	}

	return &GeoRuleSets{
		Country:     country,
		GeoIPPath:   geoipFile,
		GeositePath: geositeFile,
	}, nil
}

func ensureGeoFile(path, url string) error {
	if isGeoFileFresh(path) {
		return nil
	}
	return downloadGeoFile(path, url)
}

func isGeoFileFresh(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < GeoRuleSetMaxAge
}

var geoHTTPClient = &http.Client{Timeout: 60 * time.Second}

func downloadGeoFile(path, url string) error {
	resp, err := geoHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
