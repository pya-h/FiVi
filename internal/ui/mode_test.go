package ui

import "testing"

func TestSelectMode(t *testing.T) {
	tests := []struct {
		name      string
		noTUI     bool
		terminals TerminalState
		want      Mode
	}{
		{
			name:      "flag forces log mode",
			noTUI:     true,
			terminals: TerminalState{StdinTTY: true, StdoutTTY: true},
			want:      ModeLog,
		},
		{
			name:      "interactive terminal uses tui",
			terminals: TerminalState{StdinTTY: true, StdoutTTY: true},
			want:      ModeTUI,
		},
		{
			name:      "stdin not tty uses log",
			terminals: TerminalState{StdinTTY: false, StdoutTTY: true},
			want:      ModeLog,
		},
		{
			name:      "stdout not tty uses log",
			terminals: TerminalState{StdinTTY: true, StdoutTTY: false},
			want:      ModeLog,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SelectMode(tc.noTUI, tc.terminals)
			if got != tc.want {
				t.Fatalf("SelectMode(noTUI=%v, terminals=%+v)=%q, want %q", tc.noTUI, tc.terminals, got, tc.want)
			}
		})
	}
}
