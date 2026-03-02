package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadPID(t *testing.T) {
	tmp := t.TempDir()
	pidFile := filepath.Join(tmp, "fivi.pid")

	if err := WritePID(pidFile, 4242); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}
	pid, err := ReadPID(pidFile)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}
	if pid != 4242 {
		t.Fatalf("pid=%d, want 4242", pid)
	}
}

func TestRotateLogIfNeeded(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "fivi.log")

	if err := os.WriteFile(logFile, []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := rotateLogIfNeeded(logFile, 5); err != nil {
		t.Fatalf("rotateLogIfNeeded failed: %v", err)
	}

	if _, err := os.Stat(logFile + ".1"); err != nil {
		t.Fatalf("expected rotated file: %v", err)
	}
}
