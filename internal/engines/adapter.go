package engines

import (
	"context"

	"fivi/internal/dns"
	"fivi/internal/routing"
)

// ConfigBuildOpts bundles all parameters for building an engine runtime config.
type ConfigBuildOpts struct {
	RawURI     string
	SocksAddr  string
	HTTPAddr   string
	TunnelMode bool
	Rules      []routing.Rule
	DNS        dns.Config
	Geo        *routing.GeoRuleSets
}

type Parsed struct {
	Name     string
	Protocol string
}

type Instance interface {
	PID() int
	Stop()
}

type Adapter interface {
	Name() string
	EnsureBinary(path string) error
	BuildConfig(opts ConfigBuildOpts) ([]byte, Parsed, error)
	Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error)
}
