package engines

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Hysteria2Adapter struct{}

func NewHysteria2Adapter() *Hysteria2Adapter {
	return &Hysteria2Adapter{}
}

func (a *Hysteria2Adapter) Name() string {
	return "hysteria2"
}

func (a *Hysteria2Adapter) EnsureBinary(path string) error {
	_, err := exec.LookPath(path)
	if err != nil {
		pathValue := os.Getenv("PATH")
		if strings.TrimSpace(pathValue) == "" {
			pathValue = "(empty)"
		}
		return fmt.Errorf(
			"hysteria2 binary not found (%s): %w; install hysteria and ensure it is on PATH, or run with --hysteria2-bin /path/to/hysteria, or set FIVI_HYSTERIA2_BIN=/path/to/hysteria (PATH=%s)",
			path,
			err,
			pathValue,
		)
	}
	return nil
}

func (a *Hysteria2Adapter) BuildConfig(opts ConfigBuildOpts) ([]byte, Parsed, error) {
	if opts.TunnelMode {
		return nil, Parsed{}, fmt.Errorf("hysteria2 adapter currently supports proxy mode only")
	}

	node, err := parseHysteria2URI(opts.RawURI)
	if err != nil {
		return nil, Parsed{}, err
	}

	cfg := buildHysteria2ClientConfig(node, opts.SocksAddr, opts.HTTPAddr)
	return []byte(cfg), Parsed{Name: node.Name, Protocol: node.Protocol}, nil
}

func (a *Hysteria2Adapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error) {
	tmpDir, err := os.MkdirTemp("", "fivi-hysteria2-")
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, cfgJSON, 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}
	logPath := filepath.Join(tmpDir, "hysteria2.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "client", "-c", cfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	return &hysteria2Instance{
		cmd:     cmd,
		tmpDir:  tmpDir,
		logPath: logPath,
		logFile: logFile,
	}, nil
}

type hysteria2Instance struct {
	cmd      *exec.Cmd
	tmpDir   string
	logPath  string
	logFile  *os.File
	stopOnce sync.Once
}

func (i *hysteria2Instance) PID() int {
	if i == nil || i.cmd == nil || i.cmd.Process == nil {
		return 0
	}
	return i.cmd.Process.Pid
}

func (i *hysteria2Instance) Stop() {
	if i == nil {
		return
	}
	i.stopOnce.Do(func() {
		if i.cmd != nil && i.cmd.Process != nil {
			_ = i.cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				_ = i.cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(1500 * time.Millisecond):
				_ = i.cmd.Process.Kill()
				<-done
			}
		}
		if i.logFile != nil {
			_ = i.logFile.Close()
		}
		if i.tmpDir != "" {
			_ = os.RemoveAll(i.tmpDir)
		}
	})
}

type hysteria2Node struct {
	Name     string
	Protocol string
	Server   string
	Auth     string
	SNI      string
	Insecure bool
	ALPN     []string
	Obfs     string
	ObfsPass string
}

func parseHysteria2URI(raw string) (hysteria2Node, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return hysteria2Node{}, fmt.Errorf("hysteria2 parse: %w", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "hysteria2" && scheme != "hy2" {
		return hysteria2Node{}, fmt.Errorf("unsupported hysteria2 scheme %q", u.Scheme)
	}
	if u.Hostname() == "" || u.Port() == "" {
		return hysteria2Node{}, fmt.Errorf("hysteria2 parse: host:port is required")
	}

	q := u.Query()
	auth := ""
	if u.User != nil {
		if p, ok := u.User.Password(); ok && strings.TrimSpace(p) != "" {
			auth = p
		} else if strings.TrimSpace(u.User.Username()) != "" {
			auth = u.User.Username()
		}
	}
	if auth == "" {
		auth = firstNonEmpty(q.Get("auth"), q.Get("password"), q.Get("passwd"))
	}
	if auth == "" {
		return hysteria2Node{}, fmt.Errorf("hysteria2 parse: auth/password is required")
	}

	name := strings.TrimSpace(u.Fragment)
	if name == "" {
		name = u.Hostname()
	}

	return hysteria2Node{
		Name:     name,
		Protocol: "hysteria2",
		Server:   netJoinHostPort(u.Hostname(), u.Port()),
		Auth:     auth,
		SNI:      firstNonEmpty(strings.TrimSpace(q.Get("sni")), strings.TrimSpace(q.Get("peer")), u.Hostname()),
		Insecure: parseTruthy(q.Get("insecure")),
		ALPN:     splitList(q.Get("alpn")),
		Obfs:     strings.TrimSpace(q.Get("obfs")),
		ObfsPass: strings.TrimSpace(firstNonEmpty(q.Get("obfs-password"), q.Get("obfs_password"), q.Get("obfspassword"))),
	}, nil
}

func buildHysteria2ClientConfig(node hysteria2Node, socksAddr, httpAddr string) string {
	var b strings.Builder
	b.WriteString("server: ")
	b.WriteString(yamlQuote(node.Server))
	b.WriteString("\n")
	b.WriteString("auth: ")
	b.WriteString(yamlQuote(node.Auth))
	b.WriteString("\n")
	b.WriteString("tls:\n")
	b.WriteString("  sni: ")
	b.WriteString(yamlQuote(node.SNI))
	b.WriteString("\n")
	b.WriteString("  insecure: ")
	b.WriteString(strconv.FormatBool(node.Insecure))
	b.WriteString("\n")
	if len(node.ALPN) > 0 {
		b.WriteString("  alpn:\n")
		for _, entry := range node.ALPN {
			b.WriteString("    - ")
			b.WriteString(yamlQuote(entry))
			b.WriteString("\n")
		}
	}
	if node.Obfs != "" {
		b.WriteString("obfs:\n")
		b.WriteString("  type: ")
		b.WriteString(yamlQuote(node.Obfs))
		b.WriteString("\n")
		if node.ObfsPass != "" {
			b.WriteString("  password: ")
			b.WriteString(yamlQuote(node.ObfsPass))
			b.WriteString("\n")
		}
	}
	b.WriteString("socks5:\n")
	b.WriteString("  listen: ")
	b.WriteString(yamlQuote(socksAddr))
	b.WriteString("\n")
	b.WriteString("http:\n")
	b.WriteString("  listen: ")
	b.WriteString(yamlQuote(httpAddr))
	b.WriteString("\n")
	return b.String()
}

func yamlQuote(v string) string {
	return strconv.Quote(strings.TrimSpace(v))
}

func parseTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func netJoinHostPort(host, port string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}
