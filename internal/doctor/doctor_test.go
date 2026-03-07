package doctor

import (
	"errors"
	"strings"
	"testing"

	"fivi/internal/dns"
	"fivi/internal/routing"
)

func TestDiagnoseDNSBootstrapFailureForHostnameBootstrap(t *testing.T) {
	report := Diagnose(Options{
		Mode: "proxy",
		DNS: dns.Config{
			Remote: "https://dns.google/dns-query",
			Direct: "bootstrap.local",
		},
	}, Environment{
		GOOS:     "linux",
		LookPath: fakeLookPath("gsettings"),
		Getenv:   fakeEnv(map[string]string{"XDG_CURRENT_DESKTOP": "GNOME"}),
	})

	if !report.HasFailures() {
		t.Fatalf("expected failures, got report: %+v", report)
	}

	found := false
	for _, f := range report.Findings {
		if f.Check == "dns-bootstrap" && f.Severity == SeverityFail {
			found = true
			if !strings.Contains(f.Message, "not an IP") {
				t.Fatalf("unexpected message: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected dns-bootstrap failure finding, got: %+v", report.Findings)
	}
}

func TestDiagnoseRoutingRuleParseFailure(t *testing.T) {
	report := Diagnose(Options{
		Mode:            "proxy",
		DNS:             dns.Default(),
		RoutingRulesErr: errors.New("line 1: invalid rule format"),
	}, Environment{
		GOOS:     "linux",
		LookPath: fakeLookPath("gsettings"),
		Getenv:   fakeEnv(map[string]string{"XDG_CURRENT_DESKTOP": "GNOME"}),
	})

	if !report.HasFailures() {
		t.Fatalf("expected routing failure")
	}
}

func TestDiagnoseSystemProxySkippedInTunnelMode(t *testing.T) {
	report := Diagnose(Options{
		Mode:         "tunnel",
		DNS:          dns.Default(),
		RoutingRules: []routing.Rule{{Type: routing.RuleDomain, Value: "ir", Wildcard: true}},
	}, Environment{
		GOOS:     "linux",
		LookPath: fakeLookPath(),
		Getenv:   fakeEnv(map[string]string{"XDG_CURRENT_DESKTOP": "GNOME"}),
	})

	found := false
	for _, f := range report.Findings {
		if f.Check == "system-proxy" {
			found = true
			if f.Severity != SeveritySkip {
				t.Fatalf("expected skip severity, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected system-proxy finding")
	}
}

func TestDiagnoseLinuxGnomeMissingGsettingsWarns(t *testing.T) {
	report := Diagnose(Options{Mode: "proxy", DNS: dns.Default()}, Environment{
		GOOS:     "linux",
		LookPath: fakeLookPath(),
		Getenv:   fakeEnv(map[string]string{"XDG_CURRENT_DESKTOP": "GNOME"}),
	})

	assertFindingSeverity(t, report, "system-proxy", SeverityWarn)
}

func TestDiagnoseLinuxKDEWithCommandPasses(t *testing.T) {
	report := Diagnose(Options{Mode: "proxy", DNS: dns.Default()}, Environment{
		GOOS:     "linux",
		LookPath: fakeLookPath("kwriteconfig6"),
		Getenv:   fakeEnv(map[string]string{"XDG_CURRENT_DESKTOP": "KDE"}),
	})

	assertFindingSeverity(t, report, "system-proxy", SeverityPass)
}

func TestFormatReportIncludesSummary(t *testing.T) {
	report := Report{Findings: []Finding{{Severity: SeverityPass, Check: "routing-rules", Message: "ok"}}}
	out := FormatReport(report)
	if !strings.Contains(out, "Summary:") {
		t.Fatalf("expected summary in output: %s", out)
	}
}

func assertFindingSeverity(t *testing.T, r Report, check string, want Severity) {
	t.Helper()
	for _, f := range r.Findings {
		if f.Check == check {
			if f.Severity != want {
				t.Fatalf("check %q severity=%s want=%s", check, f.Severity, want)
			}
			return
		}
	}
	t.Fatalf("check %q not found", check)
}

func fakeLookPath(available ...string) func(string) (string, error) {
	allowed := map[string]bool{}
	for _, item := range available {
		allowed[item] = true
	}
	return func(name string) (string, error) {
		if allowed[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
