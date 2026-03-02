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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultXrayReleasesURL    = "https://api.github.com/repos/XTLS/Xray-core/releases/latest"
	defaultSingboxReleasesURL = "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Installer struct {
	Client             *http.Client
	InstallRoot        string
	XrayReleasesURL    string
	SingboxReleasesURL string
	Now                func() time.Time
}

type InstallResult struct {
	Engine  string
	Version string
	Path    string
}

func NewInstaller() (*Installer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &Installer{
		Client:             &http.Client{Timeout: 45 * time.Second},
		InstallRoot:        filepath.Join(home, ".fivi", "bins"),
		XrayReleasesURL:    defaultXrayReleasesURL,
		SingboxReleasesURL: defaultSingboxReleasesURL,
		Now:                time.Now,
	}, nil
}

func (i *Installer) InstallAll(ctx context.Context) ([]InstallResult, error) {
	if i == nil {
		return nil, errors.New("nil installer")
	}
	results := make([]InstallResult, 0, 2)
	xray, err := i.InstallEngine(ctx, "xray")
	if err != nil {
		return nil, err
	}
	results = append(results, xray)
	singbox, err := i.InstallEngine(ctx, "singbox")
	if err != nil {
		return nil, err
	}
	results = append(results, singbox)
	return results, nil
}

func (i *Installer) InstallEngine(ctx context.Context, engine string) (InstallResult, error) {
	if i == nil {
		return InstallResult{}, errors.New("nil installer")
	}
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine != "xray" && engine != "singbox" {
		return InstallResult{}, fmt.Errorf("unsupported engine %q", engine)
	}

	releaseURL := i.SingboxReleasesURL
	binaryName := "sing-box"
	if engine == "xray" {
		releaseURL = i.XrayReleasesURL
		binaryName = "xray"
	}

	rel, err := i.fetchRelease(ctx, releaseURL)
	if err != nil {
		return InstallResult{}, fmt.Errorf("fetch %s release: %w", engine, err)
	}

	archive, err := pickArchiveAsset(engine, runtime.GOOS, runtime.GOARCH, rel.Assets)
	if err != nil {
		return InstallResult{}, err
	}
	checksumAsset, err := pickChecksumAsset(engine, rel.Assets)
	if err != nil {
		return InstallResult{}, err
	}

	checksumBody, err := i.download(ctx, checksumAsset.URL)
	if err != nil {
		return InstallResult{}, fmt.Errorf("download checksum %s: %w", checksumAsset.Name, err)
	}
	wantHash, err := parseChecksumForAsset(checksumBody, archive.Name)
	if err != nil {
		return InstallResult{}, fmt.Errorf("parse checksum %s: %w", checksumAsset.Name, err)
	}

	archiveBytes, err := i.download(ctx, archive.URL)
	if err != nil {
		return InstallResult{}, fmt.Errorf("download archive %s: %w", archive.Name, err)
	}
	gotHashBytes := sha256.Sum256(archiveBytes)
	gotHash := strings.ToLower(hex.EncodeToString(gotHashBytes[:]))
	if gotHash != wantHash {
		return InstallResult{}, fmt.Errorf("checksum mismatch for %s: got %s want %s", archive.Name, gotHash, wantHash)
	}

	targetDir := filepath.Join(i.InstallRoot, engine, runtime.GOARCH)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return InstallResult{}, fmt.Errorf("create install dir: %w", err)
	}
	targetPath := filepath.Join(targetDir, binaryName)
	if runtime.GOOS == "windows" {
		targetPath += ".exe"
	}

	if err := extractBinary(archive.Name, archiveBytes, binaryName, targetPath); err != nil {
		return InstallResult{}, fmt.Errorf("extract %s: %w", archive.Name, err)
	}

	return InstallResult{
		Engine:  engine,
		Version: strings.TrimSpace(rel.TagName),
		Path:    targetPath,
	}, nil
}

func (i *Installer) fetchRelease(ctx context.Context, url string) (Release, error) {
	body, err := i.download(ctx, url)
	if err != nil {
		return Release{}, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return Release{}, fmt.Errorf("decode release payload: %w", err)
	}
	if len(rel.Assets) == 0 {
		return Release{}, errors.New("release has no assets")
	}
	return rel, nil
}

func (i *Installer) download(ctx context.Context, url string) ([]byte, error) {
	client := i.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func pickArchiveAsset(engine, goos, goarch string, assets []Asset) (Asset, error) {
	osTokens := []string{strings.ToLower(goos)}
	archTokens := archTokensFor(engine, goarch)

	for _, asset := range assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if name == "" || looksLikeChecksum(name) {
			continue
		}
		if !strings.Contains(name, osTokens[0]) {
			continue
		}
		if !matchesArch(engine, goarch, name, archTokens) {
			continue
		}
		if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		return asset, nil
	}
	return Asset{}, fmt.Errorf("no archive asset found for %s on %s/%s", engine, goos, goarch)
}

func matchesArch(engine, goarch, assetName string, tokens []string) bool {
	if !containsAny(assetName, tokens) {
		return false
	}
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	if goarch == "amd64" && strings.Contains(assetName, "arm64") {
		return false
	}
	if goarch == "arm64" && strings.Contains(assetName, "amd64") {
		return false
	}
	return true
}

func pickChecksumAsset(engine string, assets []Asset) (Asset, error) {
	for _, asset := range assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if engine == "xray" {
			if strings.Contains(name, "dgst") {
				return asset, nil
			}
			continue
		}
		if strings.Contains(name, "sha256") {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("no checksum asset found for %s", engine)
}

func parseChecksumForAsset(content []byte, assetName string) (string, error) {
	needle := strings.TrimSpace(assetName)
	if needle == "" {
		return "", errors.New("empty asset name")
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "=") {
			// SHA256 (name) = hash
			parts := strings.Split(line, "=")
			if len(parts) != 2 {
				continue
			}
			left := strings.TrimSpace(parts[0])
			hash := strings.ToLower(strings.TrimSpace(parts[1]))
			if strings.Contains(left, needle) && validSHA256(hash) {
				return hash, nil
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(strings.TrimSpace(fields[0]))
		name := strings.TrimLeft(strings.TrimSpace(fields[len(fields)-1]), "*")
		if name == needle && validSHA256(hash) {
			return hash, nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", needle)
}

func validSHA256(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, ch := range v {
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') {
			continue
		}
		return false
	}
	return true
}

func extractBinary(archiveName string, archive []byte, binaryName, targetPath string) error {
	archiveName = strings.ToLower(strings.TrimSpace(archiveName))
	if strings.HasSuffix(archiveName, ".zip") {
		return extractFromZip(archive, binaryName, targetPath)
	}
	if strings.HasSuffix(archiveName, ".tar.gz") {
		return extractFromTarGz(archive, binaryName, targetPath)
	}
	return fmt.Errorf("unsupported archive type: %s", archiveName)
}

func extractFromZip(body []byte, binaryName, targetPath string) error {
	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}
	expected := binaryName
	if runtime.GOOS == "windows" {
		expected += ".exe"
	}
	for _, f := range r.File {
		if strings.EqualFold(filepath.Base(f.Name), expected) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeExecutable(targetPath, rc)
		}
	}
	return fmt.Errorf("binary %s not found in zip", expected)
}

func extractFromTarGz(body []byte, binaryName, targetPath string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	expected := binaryName
	if runtime.GOOS == "windows" {
		expected += ".exe"
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if strings.EqualFold(filepath.Base(hdr.Name), expected) {
			return writeExecutable(targetPath, tr)
		}
	}
	return fmt.Errorf("binary %s not found in tar.gz", expected)
}

func writeExecutable(targetPath string, body io.Reader) error {
	tmp := targetPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, targetPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func looksLikeChecksum(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "sha256") || strings.Contains(name, "sha512") || strings.Contains(name, "md5") || strings.Contains(name, "dgst")
}

func archTokensFor(engine, goarch string) []string {
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	if engine == "xray" {
		switch goarch {
		case "amd64":
			return []string{"64", "amd64", "x64"}
		case "arm64":
			return []string{"arm64", "arm64-v8a", "aarch64"}
		default:
			return []string{goarch}
		}
	}
	switch goarch {
	case "amd64":
		return []string{"amd64", "x86_64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	default:
		return []string{goarch}
	}
}
