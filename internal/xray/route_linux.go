//go:build linux

package xray

import (
	"fmt"
	"os/exec"
	"strconv"
)

const (
	tunAddr4   = "172.19.0.1/30"
	routeTable = 100
)

// SetupTUNRoutes configures Linux routing rules so all non-marked traffic
// is routed through the TUN interface. Packets with the given fwmark bypass
// the TUN route to prevent a routing loop.
func SetupTUNRoutes(tunName string, mark int) error {
	markStr := strconv.Itoa(mark)
	tableStr := strconv.Itoa(routeTable)

	cmds := [][]string{
		{"ip", "addr", "add", tunAddr4, "dev", tunName},
		{"ip", "link", "set", tunName, "up"},
		{"ip", "route", "add", "default", "dev", tunName, "table", tableStr},
		{"ip", "rule", "add", "not", "fwmark", markStr, "table", tableStr},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("route setup %v: %w (%s)", args, err, out)
		}
	}
	return nil
}

// TeardownTUNRoutes removes the routing rules created by SetupTUNRoutes.
func TeardownTUNRoutes(tunName string, mark int) error {
	markStr := strconv.Itoa(mark)
	tableStr := strconv.Itoa(routeTable)

	// Best-effort removal — continue even if individual commands fail.
	var firstErr error
	cmds := [][]string{
		{"ip", "rule", "del", "not", "fwmark", markStr, "table", tableStr},
		{"ip", "route", "del", "default", "dev", tunName, "table", tableStr},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("route teardown %v: %w (%s)", args, err, out)
		}
	}
	return firstErr
}
