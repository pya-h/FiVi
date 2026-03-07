package sysproxy

import "testing"

func TestBuildDarwinSetCommands(t *testing.T) {
	cmds := BuildDarwinSetCommands("networksetup", "Wi-Fi", "127.0.0.1", 10808)
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[0][1] != "-setsocksfirewallproxy" || cmds[0][2] != "Wi-Fi" || cmds[0][3] != "127.0.0.1" || cmds[0][4] != "10808" {
		t.Fatalf("unexpected set socks command: %v", cmds[0])
	}
	if cmds[1][1] != "-setsocksfirewallproxystate" || cmds[1][3] != "on" {
		t.Fatalf("unexpected set state command: %v", cmds[1])
	}
	if cmds[2][1] != "-setproxybypassdomains" {
		t.Fatalf("unexpected bypass command: %v", cmds[2])
	}
}

func TestBuildDarwinRestoreCommands(t *testing.T) {
	state := darwinServiceState{Service: "Wi-Fi", Enabled: false, Server: "10.0.0.2", Port: 1080}
	cmds := BuildDarwinRestoreCommands("networksetup", state)
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[1][1] != "-setsocksfirewallproxystate" || cmds[1][3] != "off" {
		t.Fatalf("unexpected restore state command: %v", cmds[1])
	}
}

func TestParseDarwinServiceList(t *testing.T) {
	raw := "An asterisk (*) denotes that a network service is disabled.\nWi-Fi\n*Bluetooth PAN\nThunderbolt Bridge\n"
	got := parseDarwinServiceList(raw)
	if len(got) != 2 || got[0] != "Wi-Fi" || got[1] != "Thunderbolt Bridge" {
		t.Fatalf("unexpected services: %v", got)
	}
}

func TestParseDarwinSOCKSState(t *testing.T) {
	raw := "Enabled: Yes\nServer: 127.0.0.1\nPort: 10808\nAuthenticated Proxy Enabled: 0\n"
	state := parseDarwinSOCKSState(raw)
	if !state.Enabled || state.Server != "127.0.0.1" || state.Port != 10808 {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestParseDarwinBypassDomains(t *testing.T) {
	raw := "localhost\n127.0.0.1\n::1\n"
	got := parseDarwinBypassDomains(raw)
	if len(got) != 3 {
		t.Fatalf("unexpected bypass domains: %v", got)
	}
}

func TestParseDarwinBypassDomainsEmpty(t *testing.T) {
	raw := "There aren't any bypass domains set on Wi-Fi.\n"
	got := parseDarwinBypassDomains(raw)
	if len(got) != 0 {
		t.Fatalf("expected empty bypass domains, got: %v", got)
	}
}
