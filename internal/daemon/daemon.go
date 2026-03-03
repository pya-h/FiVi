package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const defaultLogRotateBytes int64 = 5 * 1024 * 1024

type Paths struct {
	RootDir string
	PIDFile string
	LogFile string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home dir: %w", err)
	}
	root := filepath.Join(home, ".fivi")
	return Paths{
		RootDir: root,
		PIDFile: filepath.Join(root, "fivi.pid"),
		LogFile: filepath.Join(root, "fivi.log"),
	}, nil
}

func EnsureDirs(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create daemon dir: %w", err)
	}
	return nil
}

func OpenLogFile(path string) (*os.File, error) {
	if err := EnsureDirs(path); err != nil {
		return nil, err
	}
	if err := rotateLogIfNeeded(path, defaultLogRotateBytes); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open daemon log: %w", err)
	}
	return f, nil
}

func rotateLogIfNeeded(path string, maxBytes int64) error {
	if maxBytes <= 0 {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if st.Size() < maxBytes {
		return nil
	}
	rotated := path + ".1"
	_ = os.Remove(rotated)
	if err := os.Rename(path, rotated); err != nil {
		return fmt.Errorf("rotate daemon log: %w", err)
	}
	return nil
}

func WritePID(path string, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if err := EnsureDirs(path); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace pid file: %w", err)
	}
	return nil
}

func ReadPID(path string) (int, error) {
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return 0, err
	}
	v := strings.TrimSpace(string(body))
	if v == "" {
		return 0, errors.New("empty pid file")
	}
	pid, err := strconv.Atoi(v)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid %q", v)
	}
	return pid, nil
}

func RemovePID(path string) {
	_ = os.Remove(path)
}

func ProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// ProcessLooksLikeFiViDaemon returns true when the process likely belongs to
// FiVi daemon mode. On platforms where process command line introspection is
// unavailable, this falls back to true.
func ProcessLooksLikeFiViDaemon(pid int) bool {
	if !ProcessRunning(pid) {
		return false
	}
	return processLooksLikeFiViDaemon(pid)
}
