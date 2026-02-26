package binresolver

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// binaryName maps engine name to the binary filename inside the arch directory.
var binaryName = map[string]string{
	"xray":    "xray",
	"singbox": "sing-box",
}

// Resolve locates the binary for the given engine using the current
// runtime architecture.  It probes several locations in order:
//
//  1. bins/{engine}/{GOARCH}/{binary}   relative to CWD
//  2. bins/{engine}/{GOARCH}/{binary}   next to the running executable
//  3. Falls back to the binary name alone (PATH lookup by exec.LookPath later)
func Resolve(engine string) (string, error) {
	engine = strings.ToLower(strings.TrimSpace(engine))
	bin, ok := binaryName[engine]
	if !ok {
		return "", fmt.Errorf("unknown engine %q for binary resolution", engine)
	}

	arch := mapArch(runtime.GOARCH)
	rel := filepath.Join("bins", engine, arch, bin)

	// Probe 1: relative to working directory.
	if info, err := os.Stat(rel); err == nil && !info.IsDir() {
		abs, _ := filepath.Abs(rel)
		return abs, nil
	}

	// Probe 2: relative to the running executable.
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "bins", engine, arch, bin)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	// Fallback: bare name (caller should validate with exec.LookPath).
	return bin, nil
}

// mapArch normalises GOARCH values to the directory names used under bins/.
func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}
