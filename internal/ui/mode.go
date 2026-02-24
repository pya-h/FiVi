package ui

import "os"

type Mode string

const (
	ModeTUI Mode = "tui"
	ModeLog Mode = "log"
)

type TerminalState struct {
	StdinTTY  bool
	StdoutTTY bool
}

func DetectMode(noTUI bool) Mode {
	return SelectMode(noTUI, TerminalState{
		StdinTTY:  isTerminal(os.Stdin),
		StdoutTTY: isTerminal(os.Stdout),
	})
}

func SelectMode(noTUI bool, terminals TerminalState) Mode {
	if noTUI {
		return ModeLog
	}
	if terminals.StdinTTY && terminals.StdoutTTY {
		return ModeTUI
	}
	return ModeLog
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
