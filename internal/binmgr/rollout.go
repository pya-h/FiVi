package binmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fivi/internal/binresolver"
)

type EngineRolloutState struct {
	CurrentVersion             string    `json:"current_version,omitempty"`
	LastKnownGoodVersion       string    `json:"last_known_good_version,omitempty"`
	PreviousVersion            string    `json:"previous_version,omitempty"`
	PreviousBackupPath         string    `json:"previous_backup_path,omitempty"`
	ConsecutiveStartupFailures int       `json:"consecutive_startup_failures,omitempty"`
	LastFailure                string    `json:"last_failure,omitempty"`
	LastFailureAt              time.Time `json:"last_failure_at,omitempty"`
	LastRollbackAt             time.Time `json:"last_rollback_at,omitempty"`
}

type RolloutState struct {
	UpdatedAt time.Time                     `json:"updated_at"`
	Engines   map[string]EngineRolloutState `json:"engines"`
}

type StartupRecordResult struct {
	RolledBack  bool
	Engine      string
	FromVersion string
	ToVersion   string
	Path        string
}

func (i *Installer) RecordStartupSuccess(ctx context.Context, engine string) error {
	if i == nil {
		return fmt.Errorf("nil installer")
	}
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" {
		return nil
	}
	path, version, _ := resolveInstalledVersion(ctx, engine)
	if strings.TrimSpace(version) == "" {
		return nil
	}

	state, err := i.loadRolloutState()
	if err != nil {
		return err
	}
	es := state.engineState(engine)
	es.CurrentVersion = version
	es.LastKnownGoodVersion = version
	es.ConsecutiveStartupFailures = 0
	es.LastFailure = ""
	state.Engines[engine] = es

	if path != "" {
		_ = path
	}
	return i.saveRolloutState(state)
}

func (i *Installer) RecordStartupFailure(ctx context.Context, engine string, cause error, threshold int) (StartupRecordResult, error) {
	res := StartupRecordResult{Engine: strings.ToLower(strings.TrimSpace(engine))}
	if i == nil {
		return res, fmt.Errorf("nil installer")
	}
	if res.Engine == "" {
		return res, nil
	}
	if threshold <= 0 {
		threshold = 3
	}

	state, err := i.loadRolloutState()
	if err != nil {
		return res, err
	}
	es := state.engineState(res.Engine)

	path, version, _ := resolveInstalledVersion(ctx, res.Engine)
	if strings.TrimSpace(path) == "" {
		resolved, resolveErr := binresolver.Resolve(res.Engine)
		if resolveErr == nil {
			path = resolved
		}
	}
	res.Path = path

	if strings.TrimSpace(version) == "" {
		version = strings.TrimSpace(es.CurrentVersion)
	}
	if strings.TrimSpace(version) != "" && version != es.CurrentVersion {
		es.CurrentVersion = version
		es.ConsecutiveStartupFailures = 0
	}

	es.ConsecutiveStartupFailures++
	if cause != nil {
		es.LastFailure = cause.Error()
	}
	es.LastFailureAt = time.Now()

	if es.ConsecutiveStartupFailures >= threshold && es.PreviousBackupPath != "" && path != "" {
		if restoreErr := restoreBackupBinary(es.PreviousBackupPath, path); restoreErr == nil {
			res.RolledBack = true
			res.FromVersion = es.CurrentVersion
			res.ToVersion = es.PreviousVersion
			es.CurrentVersion = es.PreviousVersion
			es.LastKnownGoodVersion = es.PreviousVersion
			es.ConsecutiveStartupFailures = 0
			es.LastRollbackAt = time.Now()
		} else {
			return res, fmt.Errorf("rollback failed for %s: %w", res.Engine, restoreErr)
		}
	}

	state.Engines[res.Engine] = es
	if err := i.saveRolloutState(state); err != nil {
		return res, err
	}
	return res, nil
}

func backupBinary(sourcePath, backupPath string) error {
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}
	tmp := backupPath + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, backupPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func restoreBackupBinary(backupPath, targetPath string) error {
	return backupBinary(backupPath, targetPath)
}

func (i *Installer) loadRolloutState() (RolloutState, error) {
	path, err := i.rolloutStatePath()
	if err != nil {
		return RolloutState{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RolloutState{Engines: map[string]EngineRolloutState{}}, nil
		}
		return RolloutState{}, err
	}
	var state RolloutState
	if err := json.Unmarshal(body, &state); err != nil {
		return RolloutState{}, err
	}
	if state.Engines == nil {
		state.Engines = map[string]EngineRolloutState{}
	}
	return state, nil
}

func (i *Installer) saveRolloutState(state RolloutState) error {
	path, err := i.rolloutStatePath()
	if err != nil {
		return err
	}
	if state.Engines == nil {
		state.Engines = map[string]EngineRolloutState{}
	}
	state.UpdatedAt = time.Now()
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (i *Installer) rolloutStatePath() (string, error) {
	if strings.TrimSpace(i.InstallRoot) != "" {
		return filepath.Join(i.InstallRoot, "rollout_state.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fivi", "bins", "rollout_state.json"), nil
}

func (s RolloutState) engineState(engine string) EngineRolloutState {
	if s.Engines == nil {
		return EngineRolloutState{}
	}
	return s.Engines[strings.ToLower(strings.TrimSpace(engine))]
}
