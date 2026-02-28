//go:build linux

package netstats

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const procNetDev = "/proc/net/dev"

// ReadCounters reads interface counters from /proc/net/dev.
func ReadCounters() ([]InterfaceCounters, error) {
	f, err := os.Open(procNetDev)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseNetDev(f)
}

// DetectInterface picks the best interface to monitor.
// For tunnel mode, it looks for known TUN interfaces.
// For proxy mode, it picks the most active non-loopback interface.
func DetectInterface(tunnelMode bool) (string, error) {
	counters, err := ReadCounters()
	if err != nil {
		return "", err
	}

	if tunnelMode {
		tunNames := []string{"tun0", "xray0"}
		for _, c := range counters {
			for _, name := range tunNames {
				if c.Interface == name {
					return c.Interface, nil
				}
			}
		}
		// Fallback: any tun* interface.
		for _, c := range counters {
			if strings.HasPrefix(c.Interface, "tun") {
				return c.Interface, nil
			}
		}
	}

	// Pick the non-loopback interface with most traffic.
	var best InterfaceCounters
	for _, c := range counters {
		if c.Interface == "lo" {
			continue
		}
		if (c.BytesRecv + c.BytesSent) > (best.BytesRecv + best.BytesSent) {
			best = c
		}
	}
	if best.Interface != "" {
		return best.Interface, nil
	}
	return "", fmt.Errorf("no suitable network interface found")
}

// StartMonitor samples /proc/net/dev at the given interval for the specified
// interface and sends rate Stats on the returned channel. If iface is empty,
// each sample auto-detects the most active non-loopback interface.
// The channel is closed when ctx is cancelled.
func StartMonitor(ctx context.Context, iface string, interval time.Duration) <-chan Stats {
	ch := make(chan Stats, 4)
	go func() {
		defer close(ch)
		var prev *InterfaceCounters
		var prevTime time.Time

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counters, err := ReadCounters()
				if err != nil {
					continue
				}
				now := time.Now()

				target := iface
				if target == "" {
					// Auto-detect: most active non-lo interface.
					var best InterfaceCounters
					for _, c := range counters {
						if c.Interface == "lo" {
							continue
						}
						if (c.BytesRecv + c.BytesSent) > (best.BytesRecv + best.BytesSent) {
							best = c
						}
					}
					target = best.Interface
				}
				if target == "" {
					continue
				}

				var current *InterfaceCounters
				for i := range counters {
					if counters[i].Interface == target {
						current = &counters[i]
						break
					}
				}
				if current == nil {
					continue
				}

				if prev != nil && prev.Interface == target {
					elapsed := now.Sub(prevTime).Seconds()
					if elapsed > 0 {
						stats := Stats{
							Interface:  target,
							RecvSpeed:  uint64(float64(current.BytesRecv-prev.BytesRecv) / elapsed),
							SendSpeed:  uint64(float64(current.BytesSent-prev.BytesSent) / elapsed),
							TotalRecv:  current.BytesRecv,
							TotalSent:  current.BytesSent,
							SampleTime: now,
						}
						select {
						case ch <- stats:
						default:
						}
					}
				}
				prev = current
				prevTime = now
			}
		}
	}()
	return ch
}
