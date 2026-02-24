package profile

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type Settings struct {
	Name                 string
	Targets              []string
	RequiredTargetTokens []string
	ProbeTimeout         time.Duration
	ProbeMinOK           int
	RetryBackoff         time.Duration
	MonitorPeriod        time.Duration
}

var profiles = map[string]Settings{
	"iran": {
		Name: "iran",
		Targets: []string{
			"https://x.com/",
			"https://www.youtube.com/",
			"https://www.cloudflare.com/cdn-cgi/trace",
		},
		RequiredTargetTokens: []string{"x.com", "youtube.com"},
		ProbeTimeout:         8 * time.Second,
		ProbeMinOK:           2,
		RetryBackoff:         14 * time.Second,
		MonitorPeriod:        45 * time.Second,
	},
	"global": {
		Name: "global",
		Targets: []string{
			"https://www.google.com/generate_204",
			"https://www.cloudflare.com/cdn-cgi/trace",
		},
		RequiredTargetTokens: nil,
		ProbeTimeout:         5 * time.Second,
		ProbeMinOK:           1,
		RetryBackoff:         8 * time.Second,
		MonitorPeriod:        30 * time.Second,
	},
}

func Resolve(name string) (Settings, error) {
	name = normalize(name)
	if p, ok := profiles[name]; ok {
		return p, nil
	}
	return Settings{}, fmt.Errorf("unknown profile %q (supported: %s)", name, strings.Join(Names(), ", "))
}

func MustResolve(name string) Settings {
	p, err := Resolve(name)
	if err != nil {
		panic(err)
	}
	return p
}

func Names() []string {
	out := make([]string, 0, len(profiles))
	for name := range profiles {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func normalize(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "iran"
	}
	return name
}
