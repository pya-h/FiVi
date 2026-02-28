package dns

import (
	"net"
	"net/url"
	"strings"
)

// Config holds DNS resolver settings for engine config builders.
type Config struct {
	// Remote is the proxy-routed DNS resolver (e.g. "tls://8.8.8.8", "https://1.1.1.1/dns-query").
	Remote string
	// Direct is the plain DNS resolver used for local/bypass resolution (e.g. "223.5.5.5").
	Direct string
}

// Default returns the default DNS configuration.
func Default() Config {
	return Config{
		Remote: "tls://8.8.8.8",
		Direct: "223.5.5.5",
	}
}

// NeedsBootstrap returns true if the remote DNS address uses a hostname
// (not an IP) which itself needs DNS resolution, creating a loop.
// In that case, a bootstrap DNS resolver should be added.
func (c Config) NeedsBootstrap() bool {
	host := extractHost(c.Remote)
	if host == "" {
		return false
	}
	return net.ParseIP(host) == nil
}

// BootstrapAddr returns the Direct resolver address suitable for use as
// a bootstrap to resolve the Remote DNS hostname.
func (c Config) BootstrapAddr() string {
	return c.Direct
}

// RemoteIsDoH returns true if the remote DNS is DNS-over-HTTPS.
func (c Config) RemoteIsDoH() bool {
	return strings.HasPrefix(strings.ToLower(c.Remote), "https://")
}

// RemoteIsDoT returns true if the remote DNS is DNS-over-TLS.
func (c Config) RemoteIsDoT() bool {
	return strings.HasPrefix(strings.ToLower(c.Remote), "tls://")
}

// RemoteIsPlain returns true if the remote DNS is plain UDP DNS (IP address only).
func (c Config) RemoteIsPlain() bool {
	return !c.RemoteIsDoH() && !c.RemoteIsDoT()
}

// extractHost pulls the hostname from a DNS address string.
// Handles formats: "tls://8.8.8.8", "https://1.1.1.1/dns-query", "223.5.5.5".
func extractHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}

	// URL-style: tls://host, https://host/path
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil {
			return addr
		}
		return u.Hostname()
	}

	// Plain IP or host:port
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port — addr is the host itself.
		return addr
	}
	return host
}
