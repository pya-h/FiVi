//go:build !linux

package xray

import "fmt"

// SetupTUNRoutes is a no-op on non-Linux platforms.
func SetupTUNRoutes(tunName string, mark int) error {
	return fmt.Errorf("xray TUN routing is only supported on Linux")
}

// TeardownTUNRoutes is a no-op on non-Linux platforms.
func TeardownTUNRoutes(tunName string, mark int) error {
	return nil
}
