package xray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Instance struct {
	cmd      *exec.Cmd
	tmpDir   string
	cfgPath  string
	logPath  string
	logFile  *os.File
	stopOnce sync.Once
}

func EnsureBinary(bin string) error {
	_, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("xray binary not found (%s): %w", bin, err)
	}
	return nil
}

func Start(ctx context.Context, bin string, configJSON []byte, name string) (*Instance, error) {
	tmpDir, err := os.MkdirTemp("", "fivi-xray-")
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, configJSON, 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}
	logPath := filepath.Join(tmpDir, "xray.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "run", "-c", cfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	inst := &Instance{cmd: cmd, tmpDir: tmpDir, cfgPath: cfgPath, logPath: logPath, logFile: logFile}
	return inst, nil
}

func (i *Instance) PID() int {
	if i == nil || i.cmd == nil || i.cmd.Process == nil {
		return 0
	}
	return i.cmd.Process.Pid
}

func (i *Instance) Wait() error {
	if i == nil || i.cmd == nil {
		return nil
	}
	return i.cmd.Wait()
}

func (i *Instance) Stop() {
	if i == nil {
		return
	}
	i.stopOnce.Do(func() {
		if i.cmd != nil && i.cmd.Process != nil {
			_ = i.cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				_ = i.cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(1500 * time.Millisecond):
				_ = i.cmd.Process.Kill()
				<-done
			}
		}
		if i.logFile != nil {
			_ = i.logFile.Close()
		}
		if i.tmpDir != "" {
			_ = os.RemoveAll(i.tmpDir)
		}
	})
}

func (i *Instance) LogPath() string {
	if i == nil {
		return ""
	}
	return i.logPath
}
