//go:build !linux

package daemon

func processLooksLikeFiViDaemon(pid int) bool {
	return true
}
