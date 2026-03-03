//go:build linux

package daemon

import (
	"fmt"
	"os"
	"strings"
)

func processLooksLikeFiViDaemon(pid int) bool {
	body, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	cmdline := strings.ReplaceAll(string(body), "\x00", " ")
	cmdline = strings.ToLower(strings.TrimSpace(cmdline))
	if cmdline == "" {
		return false
	}
	if strings.Contains(cmdline, "fivi") && strings.Contains(cmdline, "daemon") {
		return true
	}
	return false
}
