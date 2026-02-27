package xray

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Parsed struct {
	Name     string
	Protocol string
	Outbound map[string]any
}

// TUNMark is the SO_MARK value applied to outbound sockets in tunnel mode
// so their traffic bypasses the TUN routing rule and avoids a loop.
const TUNMark = 255

// TUNName is the default TUN interface name created by xray.
const TUNName = "xray0"

func BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr string, tunnelMode bool) ([]byte, Parsed, error) {
	parsed, err := ParseURI(rawURI)
	if err != nil {
		return nil, Parsed{}, err
	}

	inbounds := []any{
		map[string]any{
			"tag":      "socks-in",
			"listen":   hostOnly(socksAddr),
			"port":     portOnly(socksAddr),
			"protocol": "socks",
			"settings": map[string]any{"auth": "noauth", "udp": true},
			"sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls"}},
		},
		map[string]any{
			"tag":      "http-in",
			"listen":   hostOnly(httpAddr),
			"port":     portOnly(httpAddr),
			"protocol": "http",
			"sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls"}},
		},
	}

	if tunnelMode {
		tunInbound := map[string]any{
			"port":     0,
			"protocol": "tun",
			"tag":      "tun-in",
			"settings": map[string]any{
				"name": TUNName,
				"MTU":  1492,
			},
			"sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls"}},
		}
		inbounds = append([]any{tunInbound}, inbounds...)
	}

	proxyOut := parsed.Outbound
	directOut := map[string]any{"tag": "direct", "protocol": "freedom"}

	if tunnelMode {
		// Add sockopt mark to proxy and direct outbounds so their packets
		// bypass the TUN routing rule and avoid a routing loop.
		sockopt := map[string]any{"mark": TUNMark}
		injectSockopt(proxyOut, sockopt)
		directOut["streamSettings"] = map[string]any{"sockopt": sockopt}
	}

	routingRules := []any{
		map[string]any{"type": "field", "outboundTag": "direct", "ip": privateIPRanges()},
		map[string]any{"type": "field", "outboundTag": "proxy", "network": "tcp,udp"},
	}

	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"address": "8.8.8.8", "port": 53},
			},
		},
		"inbounds": inbounds,
		"outbounds": []any{
			proxyOut,
			directOut,
			map[string]any{"tag": "blocked", "protocol": "blackhole"},
		},
		"routing": map[string]any{
			"domainStrategy": "IPIfNonMatch",
			"rules":          routingRules,
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, Parsed{}, err
	}
	return b, parsed, nil
}

// injectSockopt adds sockopt to an outbound's streamSettings.
func injectSockopt(outbound map[string]any, sockopt map[string]any) {
	stream, ok := outbound["streamSettings"].(map[string]any)
	if !ok {
		stream = map[string]any{}
	}
	stream["sockopt"] = sockopt
	outbound["streamSettings"] = stream
}

func privateIPRanges() []string {
	return []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
}

func ParseURI(raw string) (Parsed, error) {
	switch {
	case strings.HasPrefix(strings.ToLower(raw), "vmess://"):
		return parseVMess(raw)
	case strings.HasPrefix(strings.ToLower(raw), "vless://"):
		return parseVLESS(raw)
	case strings.HasPrefix(strings.ToLower(raw), "trojan://"):
		return parseTrojan(raw)
	case strings.HasPrefix(strings.ToLower(raw), "ss://"):
		return parseSS(raw)
	default:
		return Parsed{}, fmt.Errorf("unsupported scheme")
	}
}

func parseVMess(raw string) (Parsed, error) {
	payload := raw[len("vmess://"):]
	decoded, err := decodeAnyBase64(payload)
	if err != nil {
		return Parsed{}, fmt.Errorf("vmess decode: %w", err)
	}
	type vmessNode struct {
		PS   string `json:"ps"`
		Add  string `json:"add"`
		Port string `json:"port"`
		ID   string `json:"id"`
		Aid  string `json:"aid"`
		Scy  string `json:"scy"`
		Net  string `json:"net"`
		Type string `json:"type"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
		ALPN string `json:"alpn"`
		FP   string `json:"fp"`
	}
	var n vmessNode
	if err := json.Unmarshal([]byte(decoded), &n); err != nil {
		return Parsed{}, fmt.Errorf("vmess parse: %w", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(n.Port))
	if err != nil {
		return Parsed{}, fmt.Errorf("vmess invalid port")
	}
	aid, _ := strconv.Atoi(strings.TrimSpace(n.Aid))
	if n.Scy == "" {
		n.Scy = "auto"
	}
	if n.Net == "" {
		n.Net = "tcp"
	}

	out := map[string]any{
		"tag":      "proxy",
		"protocol": "vmess",
		"settings": map[string]any{
			"vnext": []any{map[string]any{
				"address": n.Add,
				"port":    port,
				"users": []any{map[string]any{
					"id":       n.ID,
					"alterId":  aid,
					"security": n.Scy,
				}},
			}},
		},
		"streamSettings": buildStreamSettings(n.Net, n.TLS, n.SNI, n.Host, n.Path, n.ALPN, n.FP, "", "", ""),
	}
	return Parsed{Name: n.PS, Protocol: "vmess", Outbound: out}, nil
}

func parseVLESS(raw string) (Parsed, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Parsed{}, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Parsed{}, fmt.Errorf("vless invalid port")
	}
	q := u.Query()
	network := defaultIfEmpty(q.Get("type"), "tcp")
	security := q.Get("security")
	encryption := defaultIfEmpty(q.Get("encryption"), "none")
	name, _ := url.QueryUnescape(u.Fragment)
	out := map[string]any{
		"tag":      "proxy",
		"protocol": "vless",
		"settings": map[string]any{
			"vnext": []any{map[string]any{
				"address": u.Hostname(),
				"port":    port,
				"users": []any{map[string]any{
					"id":         u.User.Username(),
					"encryption": encryption,
					"flow":       q.Get("flow"),
				}},
			}},
		},
		"streamSettings": buildStreamSettings(network, security, q.Get("sni"), q.Get("host"), q.Get("path"), q.Get("alpn"), q.Get("fp"), q.Get("pbk"), q.Get("sid"), q.Get("spx")),
	}
	return Parsed{Name: name, Protocol: "vless", Outbound: out}, nil
}

func parseTrojan(raw string) (Parsed, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Parsed{}, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Parsed{}, fmt.Errorf("trojan invalid port")
	}
	q := u.Query()
	network := defaultIfEmpty(q.Get("type"), "tcp")
	security := defaultIfEmpty(q.Get("security"), "tls")
	name, _ := url.QueryUnescape(u.Fragment)
	out := map[string]any{
		"tag":      "proxy",
		"protocol": "trojan",
		"settings": map[string]any{
			"servers": []any{map[string]any{
				"address":  u.Hostname(),
				"port":     port,
				"password": u.User.Username(),
			}},
		},
		"streamSettings": buildStreamSettings(network, security, q.Get("sni"), q.Get("host"), q.Get("path"), q.Get("alpn"), q.Get("fp"), q.Get("pbk"), q.Get("sid"), q.Get("spx")),
	}
	return Parsed{Name: name, Protocol: "trojan", Outbound: out}, nil
}

func parseSS(raw string) (Parsed, error) {
	trimmed := raw[len("ss://"):]
	name := ""
	if idx := strings.Index(trimmed, "#"); idx >= 0 {
		name, _ = url.QueryUnescape(trimmed[idx+1:])
		trimmed = trimmed[:idx]
	}
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		trimmed = trimmed[:idx]
	}

	userinfoHost := trimmed
	if !strings.Contains(userinfoHost, "@") {
		decoded, err := decodeAnyBase64(trimmed)
		if err != nil {
			return Parsed{}, fmt.Errorf("ss decode: %w", err)
		}
		userinfoHost = decoded
	}

	at := strings.LastIndex(userinfoHost, "@")
	if at < 0 {
		return Parsed{}, fmt.Errorf("ss invalid payload")
	}
	userInfo := userinfoHost[:at]
	hostPort := userinfoHost[at+1:]
	if !strings.Contains(userInfo, ":") {
		decoded, err := decodeAnyBase64(userInfo)
		if err == nil {
			userInfo = decoded
		}
	}
	parts := strings.SplitN(userInfo, ":", 2)
	if len(parts) != 2 {
		return Parsed{}, fmt.Errorf("ss invalid userinfo")
	}
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return Parsed{}, fmt.Errorf("ss host/port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Parsed{}, fmt.Errorf("ss invalid port")
	}

	out := map[string]any{
		"tag":      "proxy",
		"protocol": "shadowsocks",
		"settings": map[string]any{
			"servers": []any{map[string]any{
				"address":  host,
				"port":     port,
				"method":   parts[0],
				"password": parts[1],
			}},
		},
	}
	return Parsed{Name: name, Protocol: "ss", Outbound: out}, nil
}

func buildStreamSettings(network, security, sni, host, path, alpn, fp, pbk, sid, spx string) map[string]any {
	network = defaultIfEmpty(network, "tcp")
	s := map[string]any{"network": network}

	switch strings.ToLower(security) {
	case "tls":
		tlsSettings := map[string]any{}
		if sni != "" {
			tlsSettings["serverName"] = sni
		}
		if fp != "" {
			tlsSettings["fingerprint"] = fp
		}
		if alpn != "" {
			tlsSettings["alpn"] = splitPipeOrComma(alpn)
		}
		s["security"] = "tls"
		s["tlsSettings"] = tlsSettings
	case "reality":
		reality := map[string]any{}
		if sni != "" {
			reality["serverName"] = sni
		}
		if fp != "" {
			reality["fingerprint"] = fp
		}
		if pbk != "" {
			reality["publicKey"] = pbk
		}
		if sid != "" {
			reality["shortId"] = sid
		}
		if spx != "" {
			reality["spiderX"] = spx
		}
		s["security"] = "reality"
		s["realitySettings"] = reality
	}

	path = pathOrEmpty(path)
	host = strings.TrimSpace(host)

	switch strings.ToLower(network) {
	case "ws":
		ws := map[string]any{}
		if path != "" {
			ws["path"] = path
		}
		if host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		s["wsSettings"] = ws
	case "grpc":
		service := strings.TrimPrefix(path, "/")
		if service != "" {
			s["grpcSettings"] = map[string]any{"serviceName": service}
		}
	case "httpupgrade":
		hu := map[string]any{}
		if host != "" {
			hu["host"] = host
		}
		if path != "" {
			hu["path"] = path
		}
		s["httpupgradeSettings"] = hu
	case "tcp":
		if host != "" {
			s["tcpSettings"] = map[string]any{"header": map[string]any{"type": "http", "request": map[string]any{"headers": map[string]any{"Host": []string{host}}}}}
		}
	}
	return s
}

func decodeAnyBase64(v string) (string, error) {
	v = strings.TrimSpace(v)
	decoders := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	for _, d := range decoders {
		b, err := d.DecodeString(v)
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("invalid base64")
}

func defaultIfEmpty(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
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

func splitPipeOrComma(v string) []string {
	v = strings.ReplaceAll(v, "|", ",")
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func pathOrEmpty(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	u, err := url.QueryUnescape(v)
	if err == nil {
		return u
	}
	return v
}
