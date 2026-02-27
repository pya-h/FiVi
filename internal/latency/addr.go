package latency

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ExtractServerAddr extracts the remote server host:port from a raw V2Ray URI.
// Supports vmess, vless, trojan, and ss schemes.
func ExtractServerAddr(rawURI string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(rawURI))
	switch {
	case strings.HasPrefix(lower, "vmess://"):
		return extractVMessAddr(rawURI)
	case strings.HasPrefix(lower, "vless://"):
		return extractURLAddr(rawURI)
	case strings.HasPrefix(lower, "trojan://"):
		return extractURLAddr(rawURI)
	case strings.HasPrefix(lower, "ss://"):
		return extractSSAddr(rawURI)
	default:
		return "", fmt.Errorf("unsupported scheme")
	}
}

func extractVMessAddr(raw string) (string, error) {
	payload := raw[len("vmess://"):]
	decoded, err := decodeAnyBase64(payload)
	if err != nil {
		return "", fmt.Errorf("vmess decode: %w", err)
	}
	var node struct {
		Add  string `json:"add"`
		Port any    `json:"port"`
	}
	if err := json.Unmarshal([]byte(decoded), &node); err != nil {
		return "", fmt.Errorf("vmess parse: %w", err)
	}
	addr := strings.TrimSpace(node.Add)
	if addr == "" {
		return "", fmt.Errorf("vmess empty address")
	}
	port, err := toPortString(node.Port)
	if err != nil {
		return "", fmt.Errorf("vmess port: %w", err)
	}
	return net.JoinHostPort(addr, port), nil
}

func extractURLAddr(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	port := u.Port()
	if host == "" {
		return "", fmt.Errorf("empty hostname")
	}
	if port == "" {
		return "", fmt.Errorf("empty port")
	}
	return net.JoinHostPort(host, port), nil
}

func extractSSAddr(raw string) (string, error) {
	trimmed := raw[len("ss://"):]
	// Strip fragment (#name) and query string.
	if idx := strings.Index(trimmed, "#"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		trimmed = trimmed[:idx]
	}

	userinfoHost := trimmed
	if !strings.Contains(userinfoHost, "@") {
		decoded, err := decodeAnyBase64(trimmed)
		if err != nil {
			return "", fmt.Errorf("ss decode: %w", err)
		}
		userinfoHost = decoded
	}

	at := strings.LastIndex(userinfoHost, "@")
	if at < 0 {
		return "", fmt.Errorf("ss invalid payload")
	}
	hostPort := userinfoHost[at+1:]
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", fmt.Errorf("ss host/port: %w", err)
	}
	return net.JoinHostPort(host, port), nil
}

func toPortString(v any) (string, error) {
	switch p := v.(type) {
	case string:
		p = strings.TrimSpace(p)
		if p == "" {
			return "", fmt.Errorf("empty port")
		}
		return p, nil
	case float64:
		return fmt.Sprintf("%d", int(p)), nil
	case json.Number:
		return p.String(), nil
	default:
		return "", fmt.Errorf("unexpected port type %T", v)
	}
}

func decodeAnyBase64(v string) (string, error) {
	v = strings.TrimSpace(v)
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, d := range decoders {
		b, err := d.DecodeString(v)
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("invalid base64")
}
