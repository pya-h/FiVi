package engines

import (
	"context"

	"fivi/internal/routing"
)

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
	BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr string, tunnelMode bool, rules []routing.Rule) ([]byte, Parsed, error)
	Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error)
}
