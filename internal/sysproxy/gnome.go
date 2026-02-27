package sysproxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Enable configures the GNOME system proxy to use the given SOCKS5 and HTTP
// addresses. It returns a restore function that resets the proxy to its
// previous state. If gsettings is not available, Enable returns a no-op
// restore and nil error.
func Enable(socksHost string, socksPort int, httpHost string, httpPort int) (restore func(), err error) {
	noop := func() {}

	if _, err := exec.LookPath("gsettings"); err != nil {
		return noop, nil
	}

	// Save current state.
	prevMode := gsGet("org.gnome.system.proxy", "mode")

	// Set manual proxy.
	cmds := []gsCmd{
		{"org.gnome.system.proxy", "mode", "'manual'"},
		{"org.gnome.system.proxy.socks", "host", quote(socksHost)},
		{"org.gnome.system.proxy.socks", "port", strconv.Itoa(socksPort)},
		{"org.gnome.system.proxy.http", "host", quote(httpHost)},
		{"org.gnome.system.proxy.http", "port", strconv.Itoa(httpPort)},
		{"org.gnome.system.proxy.https", "host", quote(httpHost)},
		{"org.gnome.system.proxy.https", "port", strconv.Itoa(httpPort)},
	}
	for _, c := range cmds {
		if err := gsSet(c.schema, c.key, c.value); err != nil {
			// Best-effort: restore what we can and return error.
			restoreMode(prevMode)
			return noop, fmt.Errorf("gsettings set %s %s: %w", c.schema, c.key, err)
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			restoreMode(prevMode)
		})
	}, nil
}

type gsCmd struct {
	schema, key, value string
}

func gsGet(schema, key string) string {
	out, err := exec.Command("gsettings", "get", schema, key).Output()
	if err != nil {
		return "'none'"
	}
	return strings.TrimSpace(string(out))
}

func gsSet(schema, key, value string) error {
	return exec.Command("gsettings", "set", schema, key, value).Run()
}

func restoreMode(prevMode string) {
	_ = gsSet("org.gnome.system.proxy", "mode", prevMode)
}

func quote(s string) string {
	return "'" + s + "'"
}

// BuildCommands returns the gsettings commands that Enable would run,
// without executing them. Useful for testing.
func BuildCommands(socksHost string, socksPort int, httpHost string, httpPort int) [][]string {
	return [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "'manual'"},
		{"gsettings", "set", "org.gnome.system.proxy.socks", "host", quote(socksHost)},
		{"gsettings", "set", "org.gnome.system.proxy.socks", "port", strconv.Itoa(socksPort)},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", quote(httpHost)},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", strconv.Itoa(httpPort)},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", quote(httpHost)},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", strconv.Itoa(httpPort)},
	}
}
