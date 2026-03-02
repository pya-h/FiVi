package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

func EnableKDE(socksHost string, socksPort int, httpHost string, httpPort int) (restore func(), err error) {
	noop := func() {}

	binary, ok := findKWriteConfigBinary()
	if !ok {
		return noop, nil
	}

	configPath, err := kdeConfigPath()
	if err != nil {
		return noop, err
	}

	prevContent, readErr := os.ReadFile(configPath)
	prevExists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return noop, readErr
	}

	for _, args := range BuildKDECommands(binary, configPath, socksHost, socksPort, httpHost, httpPort) {
		if runErr := exec.Command(args[0], args[1:]...).Run(); runErr != nil {
			if prevExists {
				_ = os.WriteFile(configPath, prevContent, 0o644)
			}
			return noop, fmt.Errorf("kde proxy command failed: %w", runErr)
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			if prevExists {
				_ = os.WriteFile(configPath, prevContent, 0o644)
				return
			}
			_ = os.Remove(configPath)
		})
	}, nil
}

func BuildKDECommands(binary, configPath, socksHost string, socksPort int, httpHost string, httpPort int) [][]string {
	proxy := formatKDEProxy(httpHost, httpPort)
	socks := formatKDEProxy(socksHost, socksPort)

	return [][]string{
		{binary, "--file", configPath, "--group", "Proxy Settings", "--key", "ProxyType", "1"},
		{binary, "--file", configPath, "--group", "Proxy Settings", "--key", "httpProxy", proxy},
		{binary, "--file", configPath, "--group", "Proxy Settings", "--key", "httpsProxy", proxy},
		{binary, "--file", configPath, "--group", "Proxy Settings", "--key", "ftpProxy", proxy},
		{binary, "--file", configPath, "--group", "Proxy Settings", "--key", "socksProxy", socks},
		{binary, "--file", configPath, "--group", "Proxy Settings", "--key", "NoProxyFor", "localhost,127.0.0.1,::1"},
	}
}

func formatKDEProxy(host string, port int) string {
	return host + " " + strconv.Itoa(port)
}

func findKWriteConfigBinary() (string, bool) {
	for _, cmd := range []string{"kwriteconfig6", "kwriteconfig5"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd, true
		}
	}
	return "", false
}

func isKDESession() bool {
	desktop := strings.ToLower(strings.TrimSpace(os.Getenv("XDG_CURRENT_DESKTOP") + ":" + os.Getenv("DESKTOP_SESSION")))
	return strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma")
}

func kdeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kioslaverc"), nil
}
