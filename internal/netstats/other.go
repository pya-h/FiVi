//go:build !linux

package netstats

import (
	"context"
	"fmt"
	"time"
)

// ReadCounters is not supported on this platform.
func ReadCounters() ([]InterfaceCounters, error) {
	return nil, fmt.Errorf("netstats: not supported on this platform")
}

// DetectInterface is not supported on this platform.
func DetectInterface(tunnelMode bool) (string, error) {
	return "", fmt.Errorf("netstats: not supported on this platform")
}

// StartMonitor returns a closed channel on unsupported platforms.
func StartMonitor(ctx context.Context, iface string, interval time.Duration) <-chan Stats {
	ch := make(chan Stats)
	close(ch)
	return ch
}
