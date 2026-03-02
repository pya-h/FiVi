//go:build windows

package daemon

import "os/exec"

func ConfigureDetachedProcess(cmd *exec.Cmd) {}
