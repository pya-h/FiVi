package binmgr

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"fivi/internal/binresolver"
)

type UpdateStatus struct {
	Engine          string
	Path            string
	Installed       string
	Latest          string
	UpdateAvailable bool
	Updated         bool
	Err             error
}

var versionTokenRe = regexp.MustCompile(`v?\d+\.\d+(?:\.\d+)?`)

func (i *Installer) CheckForUpdates(ctx context.Context) ([]UpdateStatus, error) {
	return i.checkOrUpdate(ctx, false)
}

func (i *Installer) UpdateAll(ctx context.Context) ([]UpdateStatus, error) {
	return i.checkOrUpdate(ctx, true)
}

func (i *Installer) checkOrUpdate(ctx context.Context, apply bool) ([]UpdateStatus, error) {
	if i == nil {
		return nil, fmt.Errorf("nil installer")
	}
	engines := []string{"xray", "singbox"}
	results := make([]UpdateStatus, 0, len(engines))

	for _, engine := range engines {
		st := UpdateStatus{Engine: engine}

		path, installed, err := resolveInstalledVersion(ctx, engine)
		if err != nil {
			st.Err = err
		}
		st.Path = path
		st.Installed = installed

		latest, err := i.fetchLatestVersion(ctx, engine)
		if err != nil {
			st.Err = err
			results = append(results, st)
			continue
		}
		st.Latest = latest

		st.UpdateAvailable = shouldUpdate(installed, latest)
		if apply && st.UpdateAvailable {
			res, installErr := i.InstallEngine(ctx, engine)
			if installErr != nil {
				st.Err = installErr
			} else {
				st.Err = nil
				st.Updated = true
				st.Path = res.Path
				st.Installed = res.Version
			}
		}
		results = append(results, st)
	}

	return results, nil
}

func (i *Installer) fetchLatestVersion(ctx context.Context, engine string) (string, error) {
	releaseURL := i.SingboxReleasesURL
	if engine == "xray" {
		releaseURL = i.XrayReleasesURL
	}
	rel, err := i.fetchRelease(ctx, releaseURL)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(rel.TagName), nil
}

func resolveInstalledVersion(ctx context.Context, engine string) (string, string, error) {
	path, err := binresolver.Resolve(engine)
	if err != nil {
		return "", "", err
	}
	if !strings.Contains(path, "/") {
		resolved, lookErr := exec.LookPath(path)
		if lookErr != nil {
			return "", "", nil
		}
		path = resolved
	}
	v, err := detectBinaryVersion(ctx, path)
	if err != nil {
		return path, "", err
	}
	return path, v, nil
}

func detectBinaryVersion(ctx context.Context, path string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, path, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run version for %s: %w", path, err)
	}
	v := parseVersionToken(string(out))
	if v == "" {
		return "", fmt.Errorf("could not parse version for %s", path)
	}
	return v, nil
}

func parseVersionToken(out string) string {
	m := versionTokenRe.FindString(strings.ToLower(out))
	if m == "" {
		return ""
	}
	if !strings.HasPrefix(m, "v") {
		return "v" + m
	}
	return m
}

func shouldUpdate(installed, latest string) bool {
	installed = normalizeVersion(installed)
	latest = normalizeVersion(latest)
	if latest == "" {
		return false
	}
	if installed == "" {
		return true
	}
	cmp := compareVersions(installed, latest)
	return cmp < 0
}

func normalizeVersion(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

func compareVersions(a, b string) int {
	ap := parseVersionParts(a)
	bp := parseVersionParts(b)
	max := len(ap)
	if len(bp) > max {
		max = len(bp)
	}
	for idx := 0; idx < max; idx++ {
		av := 0
		if idx < len(ap) {
			av = ap[idx]
		}
		bv := 0
		if idx < len(bp) {
			bv = bp[idx]
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
}

func parseVersionParts(v string) []int {
	parts := strings.Split(normalizeVersion(v), ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			out = append(out, 0)
			continue
		}
		n := 0
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out = append(out, n)
	}
	return out
}
