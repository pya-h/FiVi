package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type State struct {
	RawURI  string    `json:"raw_uri"`
	Score   float64   `json:"score"`
	SavedAt time.Time `json:"saved_at"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".fivi", "session.json"), nil
}

func SaveDefault(state State) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return Save(path, state)
}

func LoadDefault(maxAge time.Duration, now time.Time) (State, bool, error) {
	path, err := DefaultPath()
	if err != nil {
		return State{}, false, err
	}
	return Load(path, maxAge, now)
}

func Save(path string, state State) error {
	path = filepath.Clean(path)
	if strings.TrimSpace(state.RawURI) == "" {
		return errors.New("raw uri is required")
	}
	if state.SavedAt.IsZero() {
		state.SavedAt = time.Now()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return fmt.Errorf("write temp session file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace session file: %w", err)
	}
	return nil
}

func Load(path string, maxAge time.Duration, now time.Time) (State, bool, error) {
	if now.IsZero() {
		now = time.Now()
	}
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, false, nil
		}
		return State{}, false, fmt.Errorf("read session file: %w", err)
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, false, fmt.Errorf("decode session file: %w", err)
	}
	if strings.TrimSpace(state.RawURI) == "" {
		return State{}, false, nil
	}
	if state.SavedAt.IsZero() {
		return State{}, false, nil
	}
	if maxAge > 0 && now.Sub(state.SavedAt) > maxAge {
		return state, false, nil
	}
	return state, true, nil
}
