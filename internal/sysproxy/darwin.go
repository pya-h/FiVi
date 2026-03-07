package sysproxy

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type darwinServiceState struct {
	Service string
	Enabled bool
	Server  string
	Port    int
	Bypass  []string
}

func EnableDarwin(socksHost string, socksPort int, httpHost string, httpPort int) (restore func(), err error) {
	noop := func() {}
	if runtime.GOOS != "darwin" {
		return noop, nil
	}

	networksetup, err := exec.LookPath("networksetup")
	if err != nil {
		return noop, nil
	}

	services, err := listDarwinNetworkServices(networksetup)
	if err != nil {
		return noop, err
	}
	if len(services) == 0 {
		return noop, nil
	}

	prev := make([]darwinServiceState, 0, len(services))
	for _, service := range services {
		state, readErr := readDarwinServiceState(networksetup, service)
		if readErr != nil {
			return noop, readErr
		}
		prev = append(prev, state)
	}

	for _, service := range services {
		for _, args := range BuildDarwinSetCommands(networksetup, service, socksHost, socksPort) {
			if runErr := exec.Command(args[0], args[1:]...).Run(); runErr != nil {
				_ = restoreDarwinServices(networksetup, prev)
				return noop, fmt.Errorf("darwin proxy command failed for service %q: %w", service, runErr)
			}
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = restoreDarwinServices(networksetup, prev)
		})
	}, nil
}

func BuildDarwinSetCommands(networksetup, service, socksHost string, socksPort int) [][]string {
	return [][]string{
		{networksetup, "-setsocksfirewallproxy", service, socksHost, strconv.Itoa(socksPort)},
		{networksetup, "-setsocksfirewallproxystate", service, "on"},
		{networksetup, "-setproxybypassdomains", service, "localhost", "127.0.0.1", "::1"},
	}
}

func BuildDarwinRestoreCommands(networksetup string, state darwinServiceState) [][]string {
	cmds := make([][]string, 0, 4)
	if state.Server != "" && state.Port > 0 {
		cmds = append(cmds, []string{networksetup, "-setsocksfirewallproxy", state.Service, state.Server, strconv.Itoa(state.Port)})
	}
	if state.Enabled {
		cmds = append(cmds, []string{networksetup, "-setsocksfirewallproxystate", state.Service, "on"})
	} else {
		cmds = append(cmds, []string{networksetup, "-setsocksfirewallproxystate", state.Service, "off"})
	}

	bypassArgs := []string{networksetup, "-setproxybypassdomains", state.Service}
	if len(state.Bypass) == 0 {
		bypassArgs = append(bypassArgs, "")
	} else {
		bypassArgs = append(bypassArgs, state.Bypass...)
	}
	cmds = append(cmds, bypassArgs)
	return cmds
}

func restoreDarwinServices(networksetup string, states []darwinServiceState) error {
	var firstErr error
	for _, state := range states {
		for _, args := range BuildDarwinRestoreCommands(networksetup, state) {
			if err := exec.Command(args[0], args[1:]...).Run(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func listDarwinNetworkServices(networksetup string) ([]string, error) {
	out, err := exec.Command(networksetup, "-listallnetworkservices").Output()
	if err != nil {
		return nil, err
	}
	return parseDarwinServiceList(string(out)), nil
}

func parseDarwinServiceList(raw string) []string {
	lines := strings.Split(raw, "\n")
	services := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "An asterisk") {
			continue
		}
		if strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	return services
}

func readDarwinServiceState(networksetup, service string) (darwinServiceState, error) {
	socksOut, err := exec.Command(networksetup, "-getsocksfirewallproxy", service).Output()
	if err != nil {
		return darwinServiceState{}, err
	}
	bypassOut, err := exec.Command(networksetup, "-getproxybypassdomains", service).Output()
	if err != nil {
		return darwinServiceState{}, err
	}

	state := parseDarwinSOCKSState(string(socksOut))
	state.Service = service
	state.Bypass = parseDarwinBypassDomains(string(bypassOut))
	return state, nil
}

func parseDarwinSOCKSState(raw string) darwinServiceState {
	state := darwinServiceState{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "enabled":
			state.Enabled = strings.EqualFold(value, "yes")
		case "server":
			state.Server = value
		case "port":
			if p, err := strconv.Atoi(value); err == nil {
				state.Port = p
			}
		}
	}
	return state
}

func parseDarwinBypassDomains(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, "There aren't any bypass domains") {
		return nil
	}
	out := make([]string, 0, 4)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
