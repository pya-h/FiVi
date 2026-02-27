package sysproxy

import (
	"testing"
)

func TestBuildCommandsGeneratesCorrectCommands(t *testing.T) {
	cmds := BuildCommands("127.0.0.1", 10808, "127.0.0.1", 10809)
	if len(cmds) != 7 {
		t.Fatalf("expected 7 commands, got %d", len(cmds))
	}

	// First command sets mode to manual.
	if cmds[0][4] != "'manual'" {
		t.Fatalf("first command should set mode to manual, got %v", cmds[0])
	}

	// SOCKS host/port.
	if cmds[1][4] != "'127.0.0.1'" {
		t.Fatalf("socks host=%v, want '127.0.0.1'", cmds[1][4])
	}
	if cmds[2][4] != "10808" {
		t.Fatalf("socks port=%v, want 10808", cmds[2][4])
	}

	// HTTP host/port.
	if cmds[3][4] != "'127.0.0.1'" {
		t.Fatalf("http host=%v, want '127.0.0.1'", cmds[3][4])
	}
	if cmds[4][4] != "10809" {
		t.Fatalf("http port=%v, want 10809", cmds[4][4])
	}

	// HTTPS uses same as HTTP.
	if cmds[5][4] != "'127.0.0.1'" {
		t.Fatalf("https host=%v, want '127.0.0.1'", cmds[5][4])
	}
	if cmds[6][4] != "10809" {
		t.Fatalf("https port=%v, want 10809", cmds[6][4])
	}
}
