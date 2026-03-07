package binmgr

import (
	"fmt"
	"strings"
)

const (
	ChannelStable = "stable"
	ChannelLatest = "latest"
)

func normalizeChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	switch channel {
	case "", ChannelStable:
		return ChannelStable
	case ChannelLatest:
		return ChannelLatest
	default:
		return ChannelStable
	}
}

func parseTagAllowlist(csv string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(csv, ",") {
		tag := strings.TrimSpace(p)
		if tag == "" {
			continue
		}
		out[strings.ToLower(tag)] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func selectRelease(releases []Release, channel string, allowlist map[string]bool) (Release, error) {
	if len(releases) == 0 {
		return Release{}, fmt.Errorf("no releases found")
	}
	channel = normalizeChannel(channel)

	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		if channel == ChannelStable && rel.Prerelease {
			continue
		}
		tag := strings.TrimSpace(rel.TagName)
		if tag == "" {
			continue
		}
		if len(allowlist) > 0 && !allowlist[strings.ToLower(tag)] {
			continue
		}
		if len(rel.Assets) == 0 {
			continue
		}
		return rel, nil
	}

	if len(allowlist) > 0 {
		return Release{}, fmt.Errorf("no release matched channel=%s and allowlist", channel)
	}
	return Release{}, fmt.Errorf("no release matched channel=%s", channel)
}
