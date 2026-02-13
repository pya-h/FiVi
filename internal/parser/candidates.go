package parser

import (
	"sort"
	"strings"
)

type Candidate struct {
	Raw    string
	Scheme string
}

func Normalize(rawLinks []string, allowed map[string]bool, max int) []Candidate {
	seen := map[string]struct{}{}
	out := make([]Candidate, 0, min(max, len(rawLinks)))
	for _, link := range rawLinks {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		scheme := DetectScheme(link)
		if scheme == "" {
			continue
		}
		if len(allowed) > 0 && !allowed[scheme] {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, Candidate{Raw: link, Scheme: scheme})
		if max > 0 && len(out) >= max {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Scheme < out[j].Scheme
	})
	return out
}

func DetectScheme(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	schemes := []string{"vmess://", "vless://", "trojan://", "ss://"}
	for _, p := range schemes {
		if strings.HasPrefix(v, p) {
			return strings.TrimSuffix(p, "://")
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
