package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

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
	cfgJSON, parsed, err := xray.BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr, tunnelMode)
	if err != nil {
		return nil, Parsed{}, err
	}
	return cfgJSON, Parsed{
		Name:     parsed.Name,
		Protocol: parsed.Protocol,
	}, nil
}

func (a *XrayAdapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error) {
	inst, err := xray.Start(ctx, bin, cfgJSON, name)
	if err != nil {
		return nil, err
	}

	if !configHasTUN(cfgJSON) {
		return inst, nil
	}

	// TUN mode: wait for the interface to appear, then set up routes.
	if err := waitForTUNInterface(ctx, xray.TUNName, 5*time.Second); err != nil {
		inst.Stop()
		return nil, fmt.Errorf("TUN interface %s did not appear: %w", xray.TUNName, err)
	}
	if err := xray.SetupTUNRoutes(xray.TUNName, xray.TUNMark); err != nil {
		inst.Stop()
		return nil, fmt.Errorf("TUN route setup: %w", err)
	}
	return &tunXrayInstance{inner: inst}, nil
}

// tunXrayInstance wraps an xray instance to teardown TUN routes on Stop.
type tunXrayInstance struct {
	inner *xray.Instance
}

func (t *tunXrayInstance) PID() int {
	if t == nil || t.inner == nil {
		return 0
	}
	return t.inner.PID()
}

func (t *tunXrayInstance) Stop() {
	if t == nil {
		return
	}
	_ = xray.TeardownTUNRoutes(xray.TUNName, xray.TUNMark)
	if t.inner != nil {
		t.inner.Stop()
	}
}

// configHasTUN checks if the config JSON contains a TUN inbound.
func configHasTUN(cfgJSON []byte) bool {
	var cfg struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return false
	}
	for _, ib := range cfg.Inbounds {
		if ib.Protocol == "tun" {
			return true
		}
	}
	return false
}

// waitForTUNInterface polls until the named network interface exists.
func waitForTUNInterface(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for interface %s", name)
		case <-ticker.C:
			ifaces, err := net.Interfaces()
			if err != nil {
				continue
			}
			for _, iface := range ifaces {
				if iface.Name == name {
					return nil
				}
			}
		}
	}
}
