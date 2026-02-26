package engines

import (
	"context"

	"fivi/internal/xray"
)

type XrayAdapter struct{}

func NewXrayAdapter() *XrayAdapter {
	return &XrayAdapter{}
}

func (a *XrayAdapter) Name() string {
	return "xray"
}

func (a *XrayAdapter) EnsureBinary(path string) error {
	return xray.EnsureBinary(path)
}

func (a *XrayAdapter) BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr string, tunnelMode bool) ([]byte, Parsed, error) {
	// xray does not support TUN/tunnel mode natively; tunnelMode is ignored.
	cfgJSON, parsed, err := xray.BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr)
	if err != nil {
		return nil, Parsed{}, err
	}
	return cfgJSON, Parsed{
		Name:     parsed.Name,
		Protocol: parsed.Protocol,
	}, nil
}

func (a *XrayAdapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error) {
	return xray.Start(ctx, bin, cfgJSON, name)
}
