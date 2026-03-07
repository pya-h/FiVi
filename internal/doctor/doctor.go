package doctor

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"fivi/internal/dns"
	"fivi/internal/routing"
)

type Severity string

const (
	SeverityPass Severity = "PASS"
	SeverityWarn Severity = "WARN"
	SeverityFail Severity = "FAIL"
	SeveritySkip Severity = "SKIP"
)

type Finding struct {
	Severity Severity
	Check    string
	Message  string
	Hint     string
}

type Report struct {
	Findings []Finding
}

func (r Report) HasFailures() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityFail {
			return true
		}
	}
	return false
}

func (r Report) Count(s Severity) int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == s {
			n++
		}
	}
	return n
}

type Options struct {
	Mode              string
	DNS               dns.Config
	RoutingRules      []routing.Rule
	RoutingRulesErr   error
	RuleInput         string
	BothRuleInputsSet bool
}

type Environment struct {
	GOOS     string
	LookPath func(string) (string, error)
	Getenv   func(string) string
}

func RuntimeEnvironment() Environment {
	return Environment{
		GOOS:     runtime.GOOS,
		LookPath: exec.LookPath,
		Getenv:   os.Getenv,
	}
}

func Diagnose(opts Options, env Environment) Report {
	if env.LookPath == nil {
		env.LookPath = exec.LookPath
	}
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}

	findings := []Finding{
		checkRoutingRules(opts),
	}
	findings = append(findings, checkDNSBootstrap(opts.DNS)...)
	findings = append(findings, checkSystemProxySupport(opts.Mode, env)...)

	return Report{Findings: findings}
}

func FormatReport(r Report) string {
	var b strings.Builder
	b.WriteString("FiVi doctor preflight\n")
	for _, f := range r.Findings {
		b.WriteString(fmt.Sprintf("- [%s] %s: %s\n", f.Severity, f.Check, f.Message))
		if strings.TrimSpace(f.Hint) != "" {
			b.WriteString(fmt.Sprintf("  hint: %s\n", f.Hint))
		}
	}
	b.WriteString(fmt.Sprintf("Summary: pass=%d warn=%d fail=%d skip=%d\n", r.Count(SeverityPass), r.Count(SeverityWarn), r.Count(SeverityFail), r.Count(SeveritySkip)))
	if r.HasFailures() {
		b.WriteString("Result: FAIL (fix failed checks before long discovery runs)\n")
	} else if r.Count(SeverityWarn) > 0 {
		b.WriteString("Result: WARN (can run, but some assumptions may cause degraded behavior)\n")
	} else {
		b.WriteString("Result: PASS\n")
	}
	return b.String()
}

func checkRoutingRules(opts Options) Finding {
	if opts.RoutingRulesErr != nil {
		return Finding{
			Severity: SeverityFail,
			Check:    "routing-rules",
			Message:  opts.RoutingRulesErr.Error(),
			Hint:     "Fix --routing-rules / --routing-rules-file syntax (examples: domain:*.ir, domain:!forced.com, range:10.0.0.0/8)",
		}
	}

	if opts.BothRuleInputsSet {
		return Finding{
			Severity: SeverityWarn,
			Check:    "routing-rules",
			Message:  "both --routing-rules and --routing-rules-file were provided; file input takes precedence",
			Hint:     "Use one input source to avoid confusion in troubleshooting",
		}
	}

	if len(opts.RoutingRules) == 0 {
		source := opts.RuleInput
		if source == "" {
			source = "none"
		}
		return Finding{
			Severity: SeverityPass,
			Check:    "routing-rules",
			Message:  fmt.Sprintf("no routing rules loaded (source=%s)", source),
		}
	}

	direct, proxy := routing.Split(opts.RoutingRules)
	return Finding{
		Severity: SeverityPass,
		Check:    "routing-rules",
		Message:  fmt.Sprintf("loaded %d rules (%d direct, %d forced-proxy)", len(opts.RoutingRules), len(direct), len(proxy)),
	}
}

func checkDNSBootstrap(cfg dns.Config) []Finding {
	out := make([]Finding, 0, 2)

	if msg, err := validateRemoteDNS(cfg.Remote); err != nil {
		out = append(out, Finding{
			Severity: SeverityFail,
			Check:    "dns-remote",
			Message:  err.Error(),
			Hint:     "Use --doh with tls://IP, https://host/dns-query, or a plain resolver address",
		})
	} else {
		out = append(out, Finding{Severity: SeverityPass, Check: "dns-remote", Message: msg})
	}

	if !cfg.NeedsBootstrap() {
		out = append(out, Finding{
			Severity: SeverityPass,
			Check:    "dns-bootstrap",
			Message:  "remote DNS does not require bootstrap lookup",
		})
		return out
	}

	bootstrapHost, err := extractHost(cfg.BootstrapAddr())
	if err != nil || bootstrapHost == "" {
		out = append(out, Finding{
			Severity: SeverityFail,
			Check:    "dns-bootstrap",
			Message:  "remote DNS uses hostname but bootstrap resolver is empty/invalid",
			Hint:     "Set --dns to a direct IP resolver like 223.5.5.5 or 1.1.1.1",
		})
		return out
	}
	if net.ParseIP(bootstrapHost) == nil {
		out = append(out, Finding{
			Severity: SeverityFail,
			Check:    "dns-bootstrap",
			Message:  fmt.Sprintf("remote DNS hostname requires bootstrap, but bootstrap resolver %q is not an IP", bootstrapHost),
			Hint:     "Use an IP address for --dns to avoid recursive resolution loops",
		})
		return out
	}

	out = append(out, Finding{
		Severity: SeverityPass,
		Check:    "dns-bootstrap",
		Message:  fmt.Sprintf("bootstrap resolver %s can resolve remote DNS hostname", bootstrapHost),
	})
	return out
}

func validateRemoteDNS(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", errors.New("remote DNS resolver is empty")
	}
	host, err := extractHost(remote)
	if err != nil {
		return "", fmt.Errorf("invalid remote DNS resolver %q: %w", remote, err)
	}
	if host == "" {
		return "", fmt.Errorf("invalid remote DNS resolver %q: missing host", remote)
	}
	if strings.Contains(remote, "://") {
		u, _ := url.Parse(remote)
		scheme := strings.ToLower(u.Scheme)
		switch scheme {
		case "https", "tls":
		default:
			return "", fmt.Errorf("unsupported remote DNS scheme %q", scheme)
		}
	}
	return fmt.Sprintf("remote DNS resolver %q parsed successfully", remote), nil
}

func checkSystemProxySupport(mode string, env Environment) []Finding {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "proxy"
	}
	if mode == "tunnel" {
		return []Finding{{
			Severity: SeveritySkip,
			Check:    "system-proxy",
			Message:  "tunnel mode selected; system proxy tooling is not required",
		}}
	}

	switch env.GOOS {
	case "windows":
		if hasAny(env.LookPath, "powershell.exe", "powershell", "pwsh.exe", "pwsh") {
			return []Finding{{Severity: SeverityPass, Check: "system-proxy", Message: "PowerShell is available for Windows proxy automation"}}
		}
		return []Finding{{
			Severity: SeverityWarn,
			Check:    "system-proxy",
			Message:  "PowerShell was not found; automatic system proxy setup may fail",
			Hint:     "Install PowerShell or configure browser/system proxy manually",
		}}
	case "linux":
		de := detectLinuxDesktopEnvironment(env.Getenv)
		switch de {
		case "gnome":
			if hasAny(env.LookPath, "gsettings") {
				return []Finding{{Severity: SeverityPass, Check: "system-proxy", Message: "GNOME detected and gsettings command is available"}}
			}
			return []Finding{{
				Severity: SeverityWarn,
				Check:    "system-proxy",
				Message:  "GNOME detected but gsettings command is missing",
				Hint:     "Install gsettings/glib tooling or configure browser proxy manually",
			}}
		case "kde":
			if hasAny(env.LookPath, "kwriteconfig6", "kwriteconfig5") {
				return []Finding{{Severity: SeverityPass, Check: "system-proxy", Message: "KDE detected and kwriteconfig command is available"}}
			}
			return []Finding{{
				Severity: SeverityWarn,
				Check:    "system-proxy",
				Message:  "KDE detected but kwriteconfig5/6 command is missing",
				Hint:     "Install KDE config tools or configure browser proxy manually",
			}}
		default:
			if hasAny(env.LookPath, "gsettings", "kwriteconfig6", "kwriteconfig5") {
				return []Finding{{
					Severity: SeverityWarn,
					Check:    "system-proxy",
					Message:  "desktop environment not recognized; fallback proxy tooling is present",
					Hint:     "FiVi will try GNOME/KDE fallback behavior at runtime",
				}}
			}
			return []Finding{{
				Severity: SeverityWarn,
				Check:    "system-proxy",
				Message:  "desktop environment not recognized and no GNOME/KDE proxy command found",
				Hint:     "Use tunnel mode or configure browser/system proxy manually",
			}}
		}
	case "darwin":
		if hasAny(env.LookPath, "networksetup") {
			return []Finding{{
				Severity: SeverityPass,
				Check:    "system-proxy",
				Message:  "networksetup command is available for macOS proxy automation",
			}}
		}
		return []Finding{{
			Severity: SeverityWarn,
			Check:    "system-proxy",
			Message:  "networksetup command is missing; macOS automatic system-proxy setup may fail",
			Hint:     "Install macOS network command-line tools or configure system proxy manually",
		}}
	default:
		return []Finding{{
			Severity: SeverityWarn,
			Check:    "system-proxy",
			Message:  fmt.Sprintf("OS %q has no explicit system-proxy backend", env.GOOS),
			Hint:     "Use tunnel mode or configure applications to use SOCKS/HTTP proxy manually",
		}}
	}
}

func hasAny(lookPath func(string) (string, error), names ...string) bool {
	for _, name := range names {
		if _, err := lookPath(name); err == nil {
			return true
		}
	}
	return false
}

func detectLinuxDesktopEnvironment(getenv func(string) string) string {
	xdg := strings.ToLower(strings.TrimSpace(getenv("XDG_CURRENT_DESKTOP")))
	session := strings.ToLower(strings.TrimSpace(getenv("DESKTOP_SESSION")))
	gnomeSessionID := strings.TrimSpace(getenv("GNOME_DESKTOP_SESSION_ID"))

	combined := xdg + ":" + session
	if strings.Contains(combined, "kde") || strings.Contains(combined, "plasma") {
		return "kde"
	}
	if gnomeSessionID != "" || strings.Contains(combined, "gnome") {
		return "gnome"
	}
	return "unknown"
}

func extractHost(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("empty address")
	}
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil {
			return "", err
		}
		return u.Hostname(), nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No explicit port; treat whole input as host.
		return addr, nil
	}
	return host, nil
}
