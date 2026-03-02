package sysproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type windowsProxyState struct {
	ProxyEnable   int    `json:"ProxyEnable"`
	ProxyServer   string `json:"ProxyServer"`
	ProxyOverride string `json:"ProxyOverride"`
}

func EnableWindows(socksHost string, socksPort int, httpHost string, httpPort int) (restore func(), err error) {
	noop := func() {}
	if runtime.GOOS != "windows" {
		return noop, nil
	}

	powershell, ok := findPowerShell()
	if !ok {
		return noop, nil
	}

	prev, err := readWindowsProxyState(powershell)
	if err != nil {
		return noop, err
	}

	setScript := BuildWindowsSetScript(socksHost, socksPort, httpHost, httpPort)
	if err := runPowerShell(powershell, setScript); err != nil {
		return noop, fmt.Errorf("windows proxy set failed: %w", err)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			restoreScript := BuildWindowsRestoreScript(prev)
			_ = runPowerShell(powershell, restoreScript)
		})
	}, nil
}

func findPowerShell() (string, bool) {
	for _, bin := range []string{"powershell.exe", "powershell", "pwsh.exe", "pwsh"} {
		if _, err := exec.LookPath(bin); err == nil {
			return bin, true
		}
	}
	return "", false
}

func readWindowsProxyState(powershell string) (windowsProxyState, error) {
	script := "$p=Get-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings'; [PSCustomObject]@{ProxyEnable=($p.ProxyEnable);ProxyServer=($p.ProxyServer);ProxyOverride=($p.ProxyOverride)} | ConvertTo-Json -Compress"
	out, err := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return windowsProxyState{}, err
	}

	state := windowsProxyState{}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return state, errors.New("empty powershell output while reading proxy state")
	}
	if err := json.Unmarshal([]byte(trimmed), &state); err != nil {
		return windowsProxyState{}, err
	}
	return state, nil
}

func runPowerShell(powershell, script string) error {
	return exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script).Run()
}

func BuildWindowsSetScript(socksHost string, socksPort int, httpHost string, httpPort int) string {
	proxyServer := fmt.Sprintf("http=%s:%d;https=%s:%d;socks=%s:%d", httpHost, httpPort, httpHost, httpPort, socksHost, socksPort)
	proxyOverride := "<local>;localhost;127.0.0.1;::1"

	return "Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings' -Name ProxyServer -Value '" + escapePS(proxyServer) + "'; " +
		"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings' -Name ProxyOverride -Value '" + escapePS(proxyOverride) + "'; " +
		"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings' -Name ProxyEnable -Type DWord -Value 1"
}

func BuildWindowsRestoreScript(prev windowsProxyState) string {
	server := escapePS(prev.ProxyServer)
	override := escapePS(prev.ProxyOverride)
	enable := strconv.Itoa(prev.ProxyEnable)

	return "Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings' -Name ProxyServer -Value '" + server + "'; " +
		"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings' -Name ProxyOverride -Value '" + override + "'; " +
		"Set-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings' -Name ProxyEnable -Type DWord -Value " + enable
}

func escapePS(v string) string {
	return strings.ReplaceAll(v, "'", "''")
}
