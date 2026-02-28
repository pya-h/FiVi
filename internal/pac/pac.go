package pac

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"fivi/internal/routing"
)

// Server serves a PAC (Proxy Auto-Configuration) script over HTTP.
type Server struct {
	httpSrv *http.Server
	url     string
}

// Start creates and starts a PAC server on the given address. The server
// generates a FindProxyForURL script that directs browsers to use the
// specified SOCKS proxy, with bypass rules for localhost, private ranges,
// and any user-defined routing rules.
func Start(addr, socksAddr string, rules []routing.Rule) (*Server, error) {
	script := Generate(socksAddr, rules)

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy.pac", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		w.Write([]byte(script))
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("pac server listen %s: %w", addr, err)
	}

	srv := &Server{
		httpSrv: &http.Server{Handler: mux},
		url:     fmt.Sprintf("http://%s/proxy.pac", ln.Addr().String()),
	}
	go srv.httpSrv.Serve(ln)
	return srv, nil
}

// URL returns the full URL to the PAC script.
func (s *Server) URL() string {
	return s.url
}

// Stop gracefully shuts down the PAC server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*s.httpSrv.ReadTimeout+1)
	if s.httpSrv.ReadTimeout <= 0 {
		ctx, cancel = context.WithTimeout(context.Background(), 3)
	}
	defer cancel()
	s.httpSrv.Shutdown(ctx)
}

// Generate produces a PAC script body from the given proxy address and rules.
func Generate(socksAddr string, rules []routing.Rule) string {
	var b strings.Builder
	b.WriteString("function FindProxyForURL(url, host) {\n")

	// Localhost bypass.
	b.WriteString("  if (host === \"localhost\" || host === \"127.0.0.1\" || host === \"::1\")\n")
	b.WriteString("    return \"DIRECT\";\n")

	// Private IP ranges.
	for _, r := range privateRanges {
		fmt.Fprintf(&b, "  if (isInNet(host, %q, %q))\n    return \"DIRECT\";\n", r.ip, r.mask)
	}

	// Routing rules: non-negated domain/IP/range rules bypass the proxy.
	for _, rule := range rules {
		if rule.Negated {
			continue // negated rules force proxy — handled by the catch-all
		}
		switch rule.Type {
		case routing.RuleDomain:
			if rule.Wildcard {
				// *.ir → match .ir suffix
				suffix := rule.Value
				if strings.HasPrefix(suffix, "*") {
					suffix = suffix[1:]
				}
				fmt.Fprintf(&b, "  if (dnsDomainIs(host, %q))\n    return \"DIRECT\";\n", suffix)
			} else {
				fmt.Fprintf(&b, "  if (host === %q)\n    return \"DIRECT\";\n", rule.Value)
			}
		case routing.RuleIP:
			fmt.Fprintf(&b, "  if (host === %q)\n    return \"DIRECT\";\n", rule.Value)
		case routing.RuleRange:
			ip, mask := cidrToIPMask(rule.Value)
			if ip != "" {
				fmt.Fprintf(&b, "  if (isInNet(host, %q, %q))\n    return \"DIRECT\";\n", ip, mask)
			}
		}
	}

	// Catch-all: route through SOCKS proxy.
	fmt.Fprintf(&b, "  return \"SOCKS5 %s; SOCKS %s; DIRECT\";\n", socksAddr, socksAddr)
	b.WriteString("}\n")
	return b.String()
}

type ipRange struct {
	ip   string
	mask string
}

var privateRanges = []ipRange{
	{"10.0.0.0", "255.0.0.0"},
	{"172.16.0.0", "255.240.0.0"},
	{"192.168.0.0", "255.255.0.0"},
	{"169.254.0.0", "255.255.0.0"},
}

// cidrToIPMask converts a CIDR string to IP and dotted-decimal mask.
func cidrToIPMask(cidr string) (string, string) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", ""
	}
	ip := ipNet.IP.String()
	mask := net.IP(ipNet.Mask).String()
	return ip, mask
}

// PACAddr returns the PAC server address derived from the HTTP proxy address.
// It increments the port by 1 (e.g. 10809 → 10810).
func PACAddr(httpAddr string) string {
	host, portStr, err := net.SplitHostPort(httpAddr)
	if err != nil {
		return "127.0.0.1:10810"
	}
	var port int
	fmt.Sscan(portStr, &port)
	return fmt.Sprintf("%s:%d", host, port+1)
}
