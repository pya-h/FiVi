# FiVi Technical Design Document

## 1) Scope
Implement PRD requirements with focus on reliability, observability, and engine abstraction while preserving existing CLI usability. Phase 2 adds resilience, routing, DNS, monitoring, and cross-platform features inspired by oblivion-desktop's architecture.

## 2) Current Baseline (Post Milestone G + H)
- Go CLI with modules:
  - `fetch` for source pull and decode (with caching)
  - `parser` for normalization (random shuffle, dedup)
  - `xray` for runtime config/process (DNS, sniffing, TUN, private IP routing)
  - `engines` for adapter abstraction (xray + singbox, TUN-aware)
  - `probe` for SOCKS tests (latency, download speed)
  - `engine` for ranking, connect, monitor, failover, status events
  - `sources` for registry, health tracking, cooldown
  - `profile` for operational presets (iran, global)
  - `ui` for Bubble Tea TUI + log mode progress
  - `sysproxy` for GNOME system proxy
  - `config` for flag parsing and validation
  - `logx` for structured logging
- Completed milestones: A (sources), B (pipeline), C (TUI), D (engines), E (profiles), F (quality), G (V2rayN configs + status + UI), H (TUN + fetch + proxy)

## 3) Target Architecture (Phase 2)

### 3.1 New Modules

1. `internal/latency`
   - TCP connect latency pre-scanner
   - Parallel batch scanning with threshold cutoff
   - Extracted server address from URI for raw TCP check

2. `internal/routing`
   - Routing rule parser (domain, ip, cidr, negated domain)
   - Rule set loader (file or flag)
   - Profile-default rule presets
   - Integration with xray/singbox config builders

3. `internal/dns`
   - DNS config model (plain, DoH, direct resolver)
   - Loop prevention logic (resolve proxy server via direct)
   - Integration with xray/singbox config builders
   - User override via flags

4. `internal/speedtest`
   - Full speed test: download, upload, latency, jitter
   - Cloudflare speed.cloudflare.com integration
   - Standalone and connected modes
   - Result formatting

5. `internal/netstats`
   - OS network interface traffic counters
   - Real-time sent/recv speed calculation
   - Total usage tracking
   - Platform-specific implementations (Linux /proc/net/dev, macOS netstat)

6. `internal/pac`
   - PAC script generation from routing rules
   - Local HTTP server on configurable port
   - FindProxyForURL with domain/IP bypass logic

7. `internal/binmgr`
   - Binary download and version management
   - Platform/arch detection
   - Checksum verification
   - GitHub releases API integration

8. `internal/sysproxy` (expand existing)
   - Add KDE support (kwriteconfig5/6)
   - Add macOS support (networksetup)
   - Add Windows support (PowerShell registry)
   - Auto-detect desktop environment
   - Bypass list from routing rules

9. `internal/cleanup`
   - Signal handler for SIGINT/SIGTERM
   - Restore system proxy
   - Teardown TUN routes
   - Kill engine processes
   - Restart counter and crash protection

10. `internal/session`
    - Save/load last working config
    - Session state persistence
    - Auto-reconnect without full rediscovery

### 3.2 Core Interfaces (Existing + Extensions)

```go
// Existing
type EngineAdapter interface {
    Name() string
    EnsureBinary(path string) error
    BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr string, tunnelMode bool) ([]byte, Parsed, error)
    Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error)
}

// Extended for routing/DNS
type EngineAdapter interface {
    Name() string
    EnsureBinary(path string) error
    BuildRuntimeConfig(opts ConfigBuildOpts) ([]byte, Parsed, error)
    Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error)
}

type ConfigBuildOpts struct {
    RawURI       string
    SocksAddr    string
    HTTPAddr     string
    TunnelMode   bool
    RoutingRules []routing.Rule    // NEW: user routing rules
    DNS          dns.Config        // NEW: user DNS config
    GeoBypass    string            // NEW: country code for geo routing
    ShareMode    bool              // NEW: bind 0.0.0.0
}

type RoutingRule struct {
    Type     string // "domain", "ip", "cidr", "domain_suffix", "domain_negate"
    Value    string
}

type DNSConfig struct {
    RemoteDNS   string // DoH URL for proxy-routed resolution
    DirectDNS   string // Plain DNS for direct resolution
    PlainDNS    string // User override plain DNS
    DoHDNS      string // User override DoH DNS
}

// Latency pre-scanner
type LatencyScanner interface {
    ScanBatch(ctx context.Context, addrs []string, timeout time.Duration) []LatencyResult
}

type LatencyResult struct {
    Addr    string
    Latency time.Duration
    Err     error
}

// Speed test
type SpeedTestResult struct {
    Download float64       // Mbps
    Upload   float64       // Mbps
    Latency  time.Duration
    Jitter   time.Duration
}

// Network stats
type NetStats struct {
    SentSpeed  float64 // bytes/sec
    RecvSpeed  float64 // bytes/sec
    TotalSent  int64   // bytes
    TotalRecv  int64   // bytes
}
```

## 4) Data Flow (Updated)

1. Load profile (`iran` default) and engine mode.
2. Load source registry and health cache.
3. Fetch feeds concurrently with bounded parallelism.
4. Decode feeds (text, base64, json list/object extraction).
5. Normalize/dedupe candidates (random shuffle for diversity).
6. **NEW: Latency pre-scan** — TCP-connect to server addresses, reject >threshold.
7. Worker pool validates candidates and emits `WorkerEvent`.
8. Rank candidates by weighted score.
9. **NEW: Post-connect verification** — retry connectivity check N times.
10. Connect best candidate through selected engine.
11. **NEW: Apply routing rules + DNS config** to engine runtime config.
12. **NEW: Start PAC server** (proxy mode).
13. Monitor; failover or rediscover as required.
14. **NEW: Warm standby** — keep backup configs ready for instant switch.
15. **NEW: Graceful cleanup** on exit (proxy restore, TUN teardown, process kill).

## 5) Key Design Decisions

### 5.1 Latency Pre-Scan (Milestone I)
- Extract server host:port from raw V2Ray URI (parse address + port fields).
- TCP dial with 2s timeout, measure connect latency.
- Run in batches of 50 with configurable parallelism.
- Default cutoff: 1500ms. Configurable via `--latency-cutoff`.
- Saves significant time by eliminating unreachable servers before spawning engine processes.
- Runs AFTER dedup/normalize, BEFORE worker validation.

### 5.2 Connection Verification Loop (Milestone I)
- After engine start and proxy readiness, run a verification loop:
  - Fetch test URL through proxy (similar to oblivion's `checkConnectionStatus`)
  - Retry up to 5 times with 2s interval
  - Only declare "connected" after first successful verification
- This catches edge cases where proxy port is listening but tunnel not yet established.

### 5.3 Warm Standby (Milestone I)
- After ranking, keep top-3 configs in memory.
- On failover, try warm standby configs BEFORE full rediscovery.
- Reduces failover time from 30-60s (full rediscovery) to <5s.
- Warm configs expire after configurable TTL (default: 10min).

### 5.4 Graceful Cleanup (Milestone I)
- Register signal handlers for SIGINT, SIGTERM.
- Cleanup sequence: restore system proxy → teardown TUN routes → kill engine process → remove temp files.
- Use `sync.Once` to prevent double-cleanup.
- Restart counter: if cleanup fails 3 times, log error and exit hard.
- `defer` cleanup in main() as final safety net.

### 5.5 Routing Rules (Milestone J)
- Parse rules from `--routing-rules` flag or `--routing-rules-file` path.
- Rule format: one per line, `type:value` (same as oblivion-desktop).
  - `domain:example.com` — exact domain match
  - `domain:*.ir` — domain suffix (wildcard)
  - `domain:!forced.com` — negated: force through proxy (not direct)
  - `ip:1.2.3.4` — exact IP match
  - `range:10.0.0.0/8` — CIDR range match
- Negated rules have higher priority (processed first in route config).
- Profile defaults: `iran` profile includes `domain:*.ir` and common Iranian domains as direct.
- Rules injected into xray route rules array and singbox route rules array BEFORE catch-all proxy rule.

### 5.6 Configurable DNS (Milestone J)
- `--dns` flag: plain DNS server for direct resolution (e.g., `223.5.5.5`)
- `--doh` flag: DoH server for proxy-routed resolution (e.g., `https://1.1.1.1/dns-query`)
- DNS loop prevention (same approach as oblivion-desktop + existing singbox impl):
  - Proxy server's own address resolved via direct DNS
  - Remote DNS queries routed through proxy
  - Direct DNS for local/bypass traffic
- If user provides DoH with hostname (not IP), add bootstrap DNS to resolve it.
- Apply to both xray (DNS section) and singbox (dns.servers config).

### 5.7 Geo-Based Routing (Milestone J)
- `--geo-bypass` flag: country code (ir, cn, ru, etc.)
- For sing-box: download .srs rule set files from Chocolate4U/Iran-sing-box-rules.
- Cache in `~/.fivi/rulesets/` with 7-day refresh interval.
- Add geoip + geosite route rules for direct bypass.
- For xray: use geoip.dat/geosite.dat if available, or fall back to CIDR lists.

### 5.8 Speed Test (Milestone K)
- Subcommand: `./fivi speedtest`
- Test through active SOCKS proxy or specified address.
- Download test: multiple file sizes (100KB, 1MB, 10MB).
- Upload test: POST payloads of increasing size.
- Latency: multiple pings, calculate p50/p90.
- Jitter: variance of ping measurements.
- Output: formatted table in terminal.

### 5.9 Network Statistics (Milestone K)
- Read `/proc/net/dev` (Linux) or equivalent for real-time counters.
- Sample every 1s, calculate delta for speed.
- Display in TUI monitoring view: sent/recv speed + total usage.
- Use TUN interface name for TUN mode, default interface for proxy mode.
- No external binary — pure Go implementation.

### 5.10 PAC Script (Milestone K)
- Generate `FindProxyForURL` function from routing rules.
- Local HTTP server on `proxy_port + 1`.
- Default: route everything through `SOCKS5 127.0.0.1:<port>`.
- Bypass: localhost, private ranges, routing rule domains/IPs.
- Useful for proxy mode when system SOCKS proxy not respected by all apps.
- Auto-start when proxy mode is active.

### 5.11 Cross-Platform System Proxy (Milestone L)
- Auto-detect platform and desktop environment.
- Linux: detect GNOME vs KDE vs other (check `$XDG_CURRENT_DESKTOP`, `$DESKTOP_SESSION`).
- macOS: `networksetup` for all network services.
- Windows: PowerShell to modify registry `HKCU\...\Internet Settings`.
- Each implementation: Enable(host, port) → restore func.
- Bypass list constructed from routing rules.

### 5.12 LAN Sharing (Milestone L)
- `--share` flag: bind to `0.0.0.0:<port>` instead of `127.0.0.1:<port>`.
- Print local IP addresses and connection info on start.
- Warn user about security implications.
- When system proxy is also active, filter out 0.0.0.0 from proxy config (use actual local IP).

### 5.13 Binary Auto-Management (Milestone M)
- `./fivi install` command: download xray and singbox for current OS/arch.
- Use GitHub releases API to find latest versions.
- Download to `~/.fivi/bins/<engine>/<arch>/`.
- Verify SHA256 checksum from release assets.
- `./fivi update` command: check for newer versions and update.
- On startup, if binary not found, suggest `./fivi install`.

### 5.14 Auto-Connect / Daemon Mode (Milestone M)
- `./fivi daemon` command: run as background process.
- Persist last working config URI + score to `~/.fivi/session.json`.
- On restart, try persisted config first (skip discovery if still working).
- Auto-reconnect on failure: exponential backoff, max 5 retries before rediscovery.
- Write PID file for management.

## 6) Engine Support Plan

### 6.1 xray
- Keep existing implementation behind `EngineAdapter`.
- Extend config builder for routing rules and DNS overrides.
- TUN mode via native gVisor (implemented in H).

### 6.2 sing-box
- Config builder for vmess/vless/trojan/ss outbounds.
- Extend for routing rules, geo rule sets, DNS overrides.
- TUN mode with auto_route (existing).

### 6.3 clash-meta (optional, deferred)
- Only if adapter can be implemented with low complexity.

## 7) Failure Taxonomy
- `source_fetch_error`
- `source_decode_error`
- `candidate_parse_error`
- `candidate_latency_reject` (NEW)
- `engine_start_error`
- `probe_timeout`
- `probe_blocked_target_fail`
- `monitor_health_fail`
- `connection_verify_fail` (NEW)
- `tun_route_setup_fail` (NEW)
- `proxy_setup_fail` (NEW)

## 8) Observability
- Structured logs for each failure class.
- Worker events channel to UI/log reporters.
- Round summary including:
  - Source success ratio
  - Candidate throughput
  - Latency pre-scan reject count (NEW)
  - Failover count
  - Network stats (NEW)

## 9) Testing Strategy
1. Unit tests
   - Source decode and normalization
   - Profile selection defaults
   - Scoring function invariants
   - Adapter config generation
   - Routing rule parser (NEW)
   - DNS config builder (NEW)
   - Latency scanner mock (NEW)
   - PAC script generation (NEW)
   - Network stats parsing (NEW)

2. Integration tests
   - Mocked engine + mocked probe for failover state machine
   - Event stream ordering and completeness
   - Warm standby failover path (NEW)
   - Graceful cleanup sequence (NEW)

3. Smoke tests
   - `go test ./...`
   - `go build ./...`
   - Manual run in `--no-tui` and TUI modes

## 10) Migration and Compatibility
- Keep existing flags working.
- New flags are additive and optional:
  - `--latency-cutoff`, `--routing-rules`, `--routing-rules-file`
  - `--dns`, `--doh`, `--geo-bypass`
  - `--share`, `--stats`
  - `--auto-reconnect`
- New subcommands: `fetch` (done), `speedtest`, `install`, `update`, `daemon`

## 11) Security and Safety
- Do not execute arbitrary remote scripts.
- Sanitize and bound feed payload sizes.
- Isolate runtime config artifacts in temp dirs with restricted perms.
- Verify binary checksums on download (NEW).
- Graceful proxy cleanup on any exit path (NEW).
- Never leave system in broken proxy state (NEW).

## 12) Open Items
- Decide whether to include clash-meta in initial milestone after sing-box baseline lands.
- Evaluate adding custom subscription URL support (user's own V2Ray subscriptions).
- Consider adding config export/import for sharing working configs with other users.
