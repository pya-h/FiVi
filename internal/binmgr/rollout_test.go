package binmgr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRecordStartupSuccessSetsLastKnownGood(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses unix executable permission semantics")
	}

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	binaryPath := filepath.Join(tmpHome, ".fivi", "bins", "xray", runtime.GOARCH, "xray")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho 'Xray 1.2.3'\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	installer := &Installer{InstallRoot: filepath.Join(tmpHome, ".fivi", "bins"), Now: time.Now}
	if err := installer.RecordStartupSuccess(context.Background(), "xray"); err != nil {
		t.Fatalf("RecordStartupSuccess: %v", err)
	}

	state, err := installer.loadRolloutState()
	if err != nil {
		t.Fatalf("loadRolloutState: %v", err)
	}
	es := state.engineState("xray")
	if es.LastKnownGoodVersion != "v1.2.3" {
		t.Fatalf("last known good=%q want v1.2.3", es.LastKnownGoodVersion)
	}
}

func TestRecordStartupFailureRollsBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses unix executable permission semantics")
	}

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	installRoot := filepath.Join(tmpHome, ".fivi", "bins")
	binaryPath := filepath.Join(installRoot, "xray", runtime.GOARCH, "xray")
	backupPath := filepath.Join(installRoot, "xray", runtime.GOARCH, "rollback", "1.9.0", "xray")

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir binary dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("new"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho 'Xray 2.0.0'\n"), 0o755); err != nil {
		t.Fatalf("write executable binary: %v", err)
	}

	installer := &Installer{InstallRoot: installRoot, Now: time.Now}
	state := RolloutState{Engines: map[string]EngineRolloutState{
		"xray": {
			CurrentVersion:             "v2.0.0",
			PreviousVersion:            "v1.9.0",
			PreviousBackupPath:         backupPath,
			ConsecutiveStartupFailures: 1,
		},
	}}
	if err := installer.saveRolloutState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	res, err := installer.RecordStartupFailure(context.Background(), "xray", errors.New("boom"), 2)
	if err != nil {
		t.Fatalf("RecordStartupFailure: %v", err)
	}
	if !res.RolledBack {
		t.Fatal("expected rollback")
	}

	body, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(body) != "old" {
		t.Fatalf("binary content=%q want old", string(body))
	}
}
