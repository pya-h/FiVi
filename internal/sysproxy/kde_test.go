package sysproxy

import "testing"

func TestBuildKDECommands(t *testing.T) {
	cmds := BuildKDECommands("kwriteconfig6", "/tmp/kioslaverc", "127.0.0.1", 10808, "127.0.0.1", 10809)
	if len(cmds) != 6 {
		t.Fatalf("expected 6 commands, got %d", len(cmds))
	}

	if cmds[0][0] != "kwriteconfig6" {
		t.Fatalf("unexpected binary: %q", cmds[0][0])
	}
	if cmds[0][6] != "ProxyType" || cmds[0][7] != "1" {
		t.Fatalf("unexpected ProxyType command: %v", cmds[0])
	}
	if cmds[4][6] != "socksProxy" || cmds[4][7] != "127.0.0.1 10808" {
		t.Fatalf("unexpected socksProxy command: %v", cmds[4])
	}
	if cmds[5][6] != "NoProxyFor" {
		t.Fatalf("unexpected NoProxyFor command: %v", cmds[5])
	}
}

func TestFormatKDEProxy(t *testing.T) {
	if got := formatKDEProxy("10.0.0.2", 8080); got != "10.0.0.2 8080" {
		t.Fatalf("formatKDEProxy=%q", got)
	}
}

func TestIsKDESession(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("DESKTOP_SESSION", "plasma")
	if !isKDESession() {
		t.Fatal("expected KDE session detection")
	}
}
