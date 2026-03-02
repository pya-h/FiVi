package sysproxy

import (
	"os"
	"strings"
)

type DesktopEnvironment string

const (
	DesktopUnknown DesktopEnvironment = "unknown"
	DesktopGNOME   DesktopEnvironment = "gnome"
	DesktopKDE     DesktopEnvironment = "kde"
)

func DetectLinuxDesktopEnvironment() DesktopEnvironment {
	return detectLinuxDesktopEnvironmentFromEnv(os.Getenv)
}

func detectLinuxDesktopEnvironmentFromEnv(getenv func(string) string) DesktopEnvironment {
	xdg := strings.ToLower(strings.TrimSpace(getenv("XDG_CURRENT_DESKTOP")))
	session := strings.ToLower(strings.TrimSpace(getenv("DESKTOP_SESSION")))
	gnomeSessionID := strings.TrimSpace(getenv("GNOME_DESKTOP_SESSION_ID"))

	combined := xdg + ":" + session
	if strings.Contains(combined, "kde") || strings.Contains(combined, "plasma") {
		return DesktopKDE
	}
	if gnomeSessionID != "" || strings.Contains(combined, "gnome") {
		return DesktopGNOME
	}
	return DesktopUnknown
}
