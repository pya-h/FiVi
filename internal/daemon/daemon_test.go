package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestWriteReadPIDMetadata(t *testing.T) {
	tmp := t.TempDir()
	metaFile := filepath.Join(tmp, "fivi.pid.json")
	meta := PIDMetadata{
		PID:       12345,
		State:     "started",
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := WritePIDMetadata(metaFile, meta); err != nil {
		t.Fatalf("WritePIDMetadata failed: %v", err)
	}
	got, err := ReadPIDMetadata(metaFile)
	if err != nil {
		t.Fatalf("ReadPIDMetadata failed: %v", err)
	}
	if got.PID != meta.PID || got.State != meta.State {
		t.Fatalf("unexpected metadata: %+v", got)
	}
}

func TestWriteHeartbeat(t *testing.T) {
	tmp := t.TempDir()
	healthFile := filepath.Join(tmp, "daemon.health.json")
	hb := Heartbeat{
		PID:                 999,
		Phase:               "monitoring",
		NodeName:            "node-a",
		Protocol:            "vmess",
		ProbeOK:             2,
		ProbeTotal:          3,
		LastSuccessfulProbe: time.Now().UTC(),
	}
	if err := WriteHeartbeat(healthFile, hb); err != nil {
		t.Fatalf("WriteHeartbeat failed: %v", err)
	}
	if _, err := os.Stat(healthFile); err != nil {
		t.Fatalf("expected heartbeat file: %v", err)
	}
}
