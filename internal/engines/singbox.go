package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"fivi/internal/xray"
)

type SingboxAdapter struct{}

func NewSingboxAdapter() *SingboxAdapter {
	return &SingboxAdapter{}
}

func (a *SingboxAdapter) Name() string {
	return "singbox"
}

func (a *SingboxAdapter) EnsureBinary(path string) error {
	_, err := exec.LookPath(path)
	if err != nil {
		pathValue := os.Getenv("PATH")
		if strings.TrimSpace(pathValue) == "" {
			pathValue = "(empty)"
		}
		return fmt.Errorf(
			"sing-box binary not found (%s): %w; install sing-box and ensure it is on PATH, or run with --singbox-bin /path/to/sing-box, or set FIVI_SINGBOX_BIN=/path/to/sing-box (PATH=%s)",
			path,
			err,
			pathValue,
		)
	}
	return nil
}

func (a *SingboxAdapter) BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr string, tunnelMode bool) ([]byte, Parsed, error) {
	parsed, err := xray.ParseURI(rawURI)
	if err != nil {
		return nil, Parsed{}, err
	}
	proxyOutbound, err := buildSingboxOutbound(parsed)
	if err != nil {
		return nil, Parsed{}, err
	}

	inbounds := []any{
		map[string]any{
			"type":                       "socks",
			"tag":                        "socks-in",
			"listen":                     hostOnly(socksAddr),
			"listen_port":                portOnly(socksAddr),
			"sniff":                      true,
			"sniff_override_destination": true,
		},
		map[string]any{
			"type":                       "http",
			"tag":                        "http-in",
			"listen":                     hostOnly(httpAddr),
			"listen_port":                portOnly(httpAddr),
			"sniff":                      true,
			"sniff_override_destination": true,
		},
	}

	inboundTags := []string{"socks-in", "http-in"}

	if tunnelMode {
		tunInbound := map[string]any{
			"type":                       "tun",
			"tag":                        "tun-in",
			"address":                    []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
			"auto_route":                 true,
			"strict_route":               true,
			"stack":                      "mixed",
			"sniff":                      true,
			"sniff_override_destination": true,
			"endpoint_independent_nat":   true,
		}
		inbounds = append([]any{tunInbound}, inbounds...)
		inboundTags = append([]string{"tun-in"}, inboundTags...)
	}

	// DNS: remote through proxy, local through direct (prevents resolution loop).
	dnsConfig := map[string]any{
		"servers": []any{
			map[string]any{
				"tag":     "remote-dns",
				"address": "tls://8.8.8.8",
				"detour":  "proxy",
			},
			map[string]any{
				"tag":     "local-dns",
				"address": "223.5.5.5",
				"detour":  "direct",
			},
		},
		"rules": []any{
			map[string]any{
				"outbound": "any",
				"server":   "local-dns",
			},
		},
		"strategy": "prefer_ipv4",
	}

	route := map[string]any{
		"rules": []any{
			map[string]any{
				"protocol": "dns",
				"outbound": "dns-out",
			},
			map[string]any{
				"ip_is_private": true,
				"outbound":      "direct",
			},
			map[string]any{
				"inbound":  inboundTags,
				"outbound": "proxy",
			},
		},
	}
	if tunnelMode {
		route["auto_detect_interface"] = true
	}

	cfg := map[string]any{
		"log": map[string]any{
			"level": "warn",
		},
		"dns":      dnsConfig,
		"inbounds": inbounds,
		"outbounds": []any{
			proxyOutbound,
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "block", "tag": "blocked"},
			map[string]any{"type": "dns", "tag": "dns-out"},
		},
		"route": route,
	}

	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, Parsed{}, err
	}
	return body, Parsed{Name: parsed.Name, Protocol: parsed.Protocol}, nil
}

func (a *SingboxAdapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error) {
	tmpDir, err := os.MkdirTemp("", "fivi-singbox-")
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, cfgJSON, 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}
	logPath := filepath.Join(tmpDir, "singbox.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "run", "-c", cfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}

	return &singboxInstance{
		cmd:     cmd,
		tmpDir:  tmpDir,
		logPath: logPath,
		logFile: logFile,
	}, nil
}

type singboxInstance struct {
	cmd      *exec.Cmd
	tmpDir   string
	logPath  string
	logFile  *os.File
	stopOnce sync.Once
}

func (i *singboxInstance) PID() int {
	if i == nil || i.cmd == nil || i.cmd.Process == nil {
		return 0
	}
	return i.cmd.Process.Pid
}

func (i *singboxInstance) Stop() {
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

func buildSingboxOutbound(parsed xray.Parsed) (map[string]any, error) {
	switch strings.ToLower(parsed.Protocol) {
	case "vmess", "vless":
		return buildSingboxVNextOutbound(parsed)
	case "trojan":
		return buildSingboxTrojanOutbound(parsed)
	case "ss":
		return buildSingboxShadowsocksOutbound(parsed)
	default:
		return nil, fmt.Errorf("unsupported sing-box protocol: %s", parsed.Protocol)
	}
}

func buildSingboxVNextOutbound(parsed xray.Parsed) (map[string]any, error) {
	settings := mapValue(parsed.Outbound["settings"])
	vnext := sliceValue(settings["vnext"])
	if len(vnext) == 0 {
		return nil, fmt.Errorf("invalid outbound: missing vnext")
	}
	entry := mapValue(vnext[0])
	users := sliceValue(entry["users"])
	if len(users) == 0 {
		return nil, fmt.Errorf("invalid outbound: missing users")
	}
	user := mapValue(users[0])

	server := stringValue(entry["address"])
	port := intValue(entry["port"])
	uuid := stringValue(user["id"])
	if server == "" || port == 0 || uuid == "" {
		return nil, fmt.Errorf("invalid outbound: missing server/port/id")
	}

	outbound := map[string]any{
		"type":        strings.ToLower(parsed.Protocol),
		"tag":         "proxy",
		"server":      server,
		"server_port": port,
		"uuid":        uuid,
	}
	if strings.EqualFold(parsed.Protocol, "vmess") {
		if security := stringValue(user["security"]); security != "" {
			outbound["security"] = security
		}
		if alterID := intValue(user["alterId"]); alterID > 0 {
			outbound["alter_id"] = alterID
		}
	}
	if strings.EqualFold(parsed.Protocol, "vless") {
		if flow := stringValue(user["flow"]); flow != "" {
			outbound["flow"] = flow
		}
	}
	applySingboxStreamSettings(outbound, mapValue(parsed.Outbound["streamSettings"]))
	return outbound, nil
}

func buildSingboxTrojanOutbound(parsed xray.Parsed) (map[string]any, error) {
	settings := mapValue(parsed.Outbound["settings"])
	servers := sliceValue(settings["servers"])
	if len(servers) == 0 {
		return nil, fmt.Errorf("invalid outbound: missing servers")
	}
	server0 := mapValue(servers[0])
	server := stringValue(server0["address"])
	port := intValue(server0["port"])
	password := stringValue(server0["password"])
	if server == "" || port == 0 || password == "" {
		return nil, fmt.Errorf("invalid outbound: missing server/port/password")
	}

	outbound := map[string]any{
		"type":        "trojan",
		"tag":         "proxy",
		"server":      server,
		"server_port": port,
		"password":    password,
	}
	applySingboxStreamSettings(outbound, mapValue(parsed.Outbound["streamSettings"]))
	return outbound, nil
}

func buildSingboxShadowsocksOutbound(parsed xray.Parsed) (map[string]any, error) {
	settings := mapValue(parsed.Outbound["settings"])
	servers := sliceValue(settings["servers"])
	if len(servers) == 0 {
		return nil, fmt.Errorf("invalid outbound: missing servers")
	}
	server0 := mapValue(servers[0])
	server := stringValue(server0["address"])
	port := intValue(server0["port"])
	method := stringValue(server0["method"])
	password := stringValue(server0["password"])
	if server == "" || port == 0 || method == "" || password == "" {
		return nil, fmt.Errorf("invalid outbound: missing server/port/method/password")
	}

	return map[string]any{
		"type":        "shadowsocks",
		"tag":         "proxy",
		"server":      server,
		"server_port": port,
		"method":      method,
		"password":    password,
	}, nil
}

func applySingboxStreamSettings(outbound map[string]any, stream map[string]any) {
	if len(stream) == 0 {
		return
	}
	if tls := singboxTLSOptions(stream); len(tls) > 0 {
		outbound["tls"] = tls
	}
	if transport := singboxTransportOptions(stream); len(transport) > 0 {
		outbound["transport"] = transport
	}
}

func singboxTLSOptions(stream map[string]any) map[string]any {
	security := strings.ToLower(stringValue(stream["security"]))
	if security != "tls" && security != "reality" {
		return nil
	}

	tls := map[string]any{"enabled": true}
	if security == "tls" {
		tlsSettings := mapValue(stream["tlsSettings"])
		if serverName := stringValue(tlsSettings["serverName"]); serverName != "" {
			tls["server_name"] = serverName
		}
		if alpn := stringSliceValue(tlsSettings["alpn"]); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		if fp := stringValue(tlsSettings["fingerprint"]); fp != "" {
			tls["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": fp,
			}
		}
		return tls
	}

	reality := mapValue(stream["realitySettings"])
	if serverName := stringValue(reality["serverName"]); serverName != "" {
		tls["server_name"] = serverName
	}
	if fp := stringValue(reality["fingerprint"]); fp != "" {
		tls["utls"] = map[string]any{
			"enabled":     true,
			"fingerprint": fp,
		}
	}
	realityOpt := map[string]any{"enabled": true}
	if pbk := stringValue(reality["publicKey"]); pbk != "" {
		realityOpt["public_key"] = pbk
	}
	if sid := stringValue(reality["shortId"]); sid != "" {
		realityOpt["short_id"] = sid
	}
	if spx := stringValue(reality["spiderX"]); spx != "" {
		realityOpt["spider_x"] = spx
	}
	if len(realityOpt) > 1 {
		tls["reality"] = realityOpt
	}
	return tls
}

func singboxTransportOptions(stream map[string]any) map[string]any {
	switch strings.ToLower(stringValue(stream["network"])) {
	case "ws":
		ws := mapValue(stream["wsSettings"])
		transport := map[string]any{"type": "ws"}
		if path := stringValue(ws["path"]); path != "" {
			transport["path"] = path
		}
		headers := mapValue(ws["headers"])
		if host := stringValue(headers["Host"]); host != "" {
			transport["headers"] = map[string]any{"Host": host}
		}
		return transport
	case "grpc":
		grpc := mapValue(stream["grpcSettings"])
		transport := map[string]any{"type": "grpc"}
		if service := stringValue(grpc["serviceName"]); service != "" {
			transport["service_name"] = service
		}
		return transport
	case "httpupgrade":
		hu := mapValue(stream["httpupgradeSettings"])
		transport := map[string]any{"type": "httpupgrade"}
		if host := stringValue(hu["host"]); host != "" {
			transport["host"] = host
		}
		if path := stringValue(hu["path"]); path != "" {
			transport["path"] = path
		}
		return transport
	default:
		return nil
	}
}

func hostOnly(addr string) string {
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "127.0.0.1"
	}
	return h
}

func portOnly(addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 10808
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 10808
	}
	return n
}

func mapValue(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func sliceValue(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case fmt.Stringer:
		return strings.TrimSpace(x.String())
	default:
		return ""
	}
}

func stringSliceValue(v any) []string {
	values := sliceValue(v)
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		s := stringValue(value)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func intValue(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	default:
		return 0
	}
}
