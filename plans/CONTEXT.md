# FiVi Session Context

## Project Snapshot
- **Project**: `FiVi` — Go CLI for discovering, validating, ranking, connecting to, and maintaining public V2Ray links with automatic failover
- **Engines**: `xray` (default), `sing-box` — abstracted via `engines.Adapter` interface
- **Modes**: `proxy` (SOCKS5/HTTP on localhost), `tunnel` (TUN system-wide)
- **Branch**: `staging` (main: `main`)
- **Go**: 1.23+, module name: `fivi`
- **Latest commit**: `3f0d757` — feat: speed test + network stats + pac script
- **Test status**: 19/19 packages pass (`go test ./...`)

## Codebase Structure

```
fivi/
├── main.go                           # Entry point, subcommands (fetch/speedtest), flag parsing, mode dispatch
├── go.mod / go.sum                   # Module: fivi, Go 1.23+
├── plans/                            # PRD.md, TDD.md, TASKS.md, CONTEXT.md
├── bins/                             # Engine binaries (xray/, singbox/ by arch)
├── output/                           # Runtime artifacts (all_configs.txt, working/probable, source_report.json)
├── internal/
│   ├── config/config.go              # AppConfig struct, flag parsing, binary resolution
│   ├── engine/
│   │   ├── engine.go                 # Core loop: discover → rank → connect → monitor → failover
│   │   ├── events.go                 # WorkerEvent, StatusUpdate, Phase types, emitStatus
│   │   ├── fetch_only.go             # ./fivi fetch subcommand
│   │   └── *_test.go                 # scoring, failover integration, benchmark, readiness
│   ├── engines/
│   │   ├── adapter.go                # Adapter interface, ConfigBuildOpts struct, Instance interface
│   │   ├── xray.go                   # XrayAdapter (TUN-aware with route management)
│   │   ├── singbox.go                # SingboxAdapter (DNS, sniffing, TUN/proxy, NAT, geo rule sets)
│   │   └── *_test.go
│   ├── xray/
│   │   ├── config.go                 # Xray JSON config builder (DNS, sniffing, routing rules, TUN)
│   │   ├── process.go                # Start/stop xray subprocess
│   │   ├── route_linux.go            # Linux TUN route setup (ip rule/route, fwmark 255)
│   │   └── route_other.go            # Stub for non-Linux
│   ├── cleanup/cleanup.go            # LIFO cleanup manager (sync.Once, panic recovery)
│   ├── dns/config.go                 # DNS config (Remote/Direct), DoH/DoT detection, bootstrap
│   ├── fetch/
│   │   ├── fetch.go                  # FetchAll from URLs, decode (text, base64, JSON)
│   │   └── cache.go                  # File-based cache for source resilience
│   ├── latency/
│   │   ├── addr.go                   # Extract host:port from vmess/vless/trojan/ss URIs
│   │   └── scanner.go               # Parallel TCP connect pre-scanner (ScanBatch)
│   ├── netstats/
│   │   ├── netstats.go               # Types, /proc/net/dev parser, format helpers
│   │   ├── linux.go                  # ReadCounters, DetectInterface, StartMonitor
│   │   └── other.go                  # Non-Linux stubs
│   ├── pac/pac.go                    # PAC script generator + HTTP server
│   ├── parser/candidates.go          # Normalize: dedup, scheme filter, shuffle
│   ├── probe/probe.go                # SOCKS proxy testing (CheckThroughSOCKS, MeasureDownloadSpeed)
│   ├── profile/profile.go            # iran/global profiles (targets, timeouts, required tokens)
│   ├── routing/
│   │   ├── rules.go                  # Rule model, parser, CSV/file loading, profile defaults
│   │   └── geo.go                    # Geo rule set download/cache (singbox .srs format)
│   ├── sources/
│   │   ├── registry.go               # 30 default source URLs
│   │   └── health.go                 # HealthTracker (cooldown, backoff, yield)
│   ├── speedtest/speedtest.go        # Download/upload/latency/jitter through SOCKS proxy
│   ├── sysproxy/gnome.go             # GNOME gsettings proxy (Enable → restore func)
│   ├── ui/
│   │   ├── mode.go                   # TTY detection, --no-tui, TUI vs log mode
│   │   ├── dashboard.go              # Bubble Tea TUI (discovery + monitoring views, netstats)
│   │   └── progress.go               # Log mode reporter (phase + health + netstats)
│   ├── binresolver/resolver.go       # Auto-detect engine binaries from bins/ directory
│   └── logx/logx.go                  # Structured logging with output redirection
```

## Completion Status

### Done (A–K)
- **A** (A1–A5): Source expansion (30 URLs), health tracking, cooldown, JSON feeds, reports
- **B** (B1–B5): Per-candidate timeout, readiness checks, failure taxonomy, probable bucket, cancellation safety
- **C** (C1–C4): TUI mode detection, worker events, Bubble Tea dashboard, log progress
- **D** (D1–D4): Engine adapter interface, xray/singbox adapters, engine flags (clash-meta deferred)
- **E** (E1–E3): Profile system (iran default), tuned targets/timeouts
- **F** (F1–F4): Scoring tests, failover integration tests, benchmarks
- **G** (G1–G7): V2rayN-quality configs (DNS, sniffing, private IP routing), status events, TUI redesign
- **H** (H1–H4): Xray TUN mode, shuffle (no max-candidates), `./fivi fetch`, GNOME system proxy
- **I** (I1–I5): Latency pre-scan, post-connect verify loop, warm standby (top-3), graceful cleanup, auto-reconnect
- **J** (J1–J6): Routing rules, DNS config, geo routing (singbox .srs), ConfigBuildOpts refactor
- **K** (K1–K3): Speed test subcommand, real-time netstats, PAC script server

### Pending
- **L** (L1–L5): Cross-platform proxy (KDE, macOS, Windows), LAN sharing
- **M** (M1–M4): Binary auto-management, session persistence, daemon mode

## Key Interfaces and Patterns

### Adapter Interface (`engines/adapter.go`)
```go
type Adapter interface {
    Name() string
    EnsureBinary() error
    BuildConfig(opts ConfigBuildOpts) (string, error)
    Start(configPath string) (Instance, error)
}

type ConfigBuildOpts struct {
    RawURI     string
    SocksAddr  string
    HTTPAddr   string
    TunnelMode bool
    Rules      []routing.Rule
    DNS        dns.Config
    Geo        *routing.GeoRuleSets
}
```

### AppConfig Key Fields (`config/config.go`)
```go
type AppConfig struct {
    Profile, Engine, Mode          string   // "iran"/"global", "xray"/"singbox", "proxy"/"tunnel"
    ListenSocksAddr, ListenHTTPAddr string  // "127.0.0.1:10808", "127.0.0.1:10809"
    Workers, TopCount              int      // concurrency, top-N for ranking
    LatencyCutoff                  time.Duration // TCP pre-scan cutoff (default 1500ms, 0=off)
    VerifyRetries                  int      // post-connect verify attempts (default 5)
    StandbyTTL                     time.Duration // warm standby TTL (default 10m, 0=off)
    AutoReconnect                  bool     // retry same config before failover (default true)
    RoutingRules                   []routing.Rule
    DNS                            dns.Config
    GeoBypass                      string   // country code (e.g. "ir")
    GeoRuleSets                    *routing.GeoRuleSets // resolved at startup
    NetStats                       bool     // --stats flag
    NetStatsIface                  string   // --stats-iface override
    OnlySchemes                    map[string]bool
}
```

### Event System (`engine/events.go`)
- **WorkerEvent**: per-candidate progress (WorkerID, Candidate, Stage, Status, Attempt, FailureReason)
- **StatusUpdate**: engine-level phases (Phase, NodeName, Protocol, SocksAddr, HTTPAddr, Ping, ProbeOK/Total, Failures)
- **Phases**: `PhaseDiscovering`, `PhaseConnecting`, `PhaseMonitoring`, `PhaseFailover`, `PhaseWarmFailover`
- **Worker stages**: `WorkerStageParse`, `WorkerStageSpawn`, `WorkerStageProbe`, `WorkerStageDone`
- Non-blocking emission via `select-default`

### Engine Core Flow (`engine/engine.go`)
```
RunWithEvents → RunWithAdapter (loop):
  1. discoverAndRank: fetch → normalize → latency pre-scan → worker pool validate → score → rank
  2. connectAndMonitor (ranked list):
     for each candidate:
       connectAndMonitorOne:
         startCandidate (spawn engine + verify loop) →
         setup sysproxy + PAC →
         monitorConnection (health checks) →
         on failure: auto-reconnect (3x exp backoff) or next candidate
  3. On all exhausted: try warm standbys → full rediscovery
```

### Failover Chain
`auto-reconnect same config (3x)` → `next ranked candidate` → `warm standby (top-3, 10min TTL)` → `full rediscovery`

### Routing Rule Priority (both engines)
1. Private IPs (direct)
2. Negated rules (force proxy)
3. Geo rule sets — singbox only (direct)
4. User domain/IP/CIDR rules (direct)
5. Catch-all (proxy)

### DNS Configuration
- Default remote: `tls://8.8.8.8` (via proxy), default direct: `223.5.5.5`
- Override: `--doh tls://1.1.1.1` (remote), `--dns 4.2.2.4` (direct)
- Bootstrap DNS auto-added when remote uses hostname (singbox)

### Subcommands
- `./fivi` — main mode (discover → connect → monitor)
- `./fivi fetch` — fetch sources, save cache, exit
- `./fivi speedtest` — download/upload/latency/jitter through SOCKS proxy (own FlagSet, no engine binary needed)

### PAC Script (`pac/pac.go`)
- Auto-starts in proxy mode at `http://localhost:<http_port+1>/proxy.pac`
- Bypasses: localhost, private IPs, non-negated routing rules (domains, IPs, CIDRs)
- Catch-all: `SOCKS5 <addr>; SOCKS <addr>; DIRECT`

### Network Stats (`netstats/`)
- Linux: reads `/proc/net/dev`, calculates per-second deltas
- Auto-detects interface: `tun0`/`xray0` for tunnel, most-active for proxy
- `--stats` flag (off by default), `--stats-iface` override
- Displayed in TUI monitoring view and log mode snapshots

### Cleanup (`cleanup/cleanup.go`)
- LIFO order, sync.Once, panic recovery per hook
- Registered hooks: stop engine, restore system proxy, stop PAC server, teardown TUN routes
- Wired in `main.go` as global `cleanupMgr` with deferred `Run()`

## Verified Environment
- Go 1.23+
- Xray binary: `Xray 25.10.15` (Linux ELF 64-bit, static)
- Singbox binary: `sing-box 1.12.22`
- Platform: Linux (Debian-based, GNOME)
- `go test ./...` and `go build ./...` both pass

## Next: Milestone L (Cross-Platform Proxy + LAN Sharing)

Tasks from `TASKS.md`:
- **L1**: KDE system proxy (`kwriteconfig5`/`kwriteconfig6`, `kioslaverc`)
- **L2**: macOS system proxy (`networksetup`)
- **L3**: Windows system proxy (PowerShell, registry)
- **L4**: Linux DE auto-detection (`$XDG_CURRENT_DESKTOP`, dispatch to GNOME/KDE)
- **L5**: LAN sharing (`--share` flag, bind `0.0.0.0`)

Current `sysproxy/gnome.go` pattern to follow: `Enable(socksAddr, httpAddr) (restoreFunc, error)`

## Context Update Protocol (mandatory)
After each completed task, append a section to this file:
- Task ID and date
- Summary of changes
- Files touched
- Behavior impact
- Validation performed (`go test`, `go build`, manual)
- Next task or blocker

## Context Update — 2026-03-02 (Debug Sweep: cache + connect defaults)

- **Task ID and date**: `DEBUG-2026-03-02`
- **Summary of changes**:
  - Fixed source cache persistence path to be stable across runs/working directories (`~/.fivi/cache/sources` by default, overridable via `FIVI_CACHE_DIR`).
  - Fixed cache fallback behavior so cached links are used when a source fetch fails **or** returns zero links.
  - Added cache reuse for sources skipped in cooldown during discovery rounds.
  - Adjusted safer runtime defaults for higher connection success on mixed environments: default mode is now `proxy`, startup readiness timeout increased to `8s`, and implicit tunnel→singbox engine override removed.
  - Updated docs to reflect default mode and cache location semantics.
- **Files touched**:
  - `internal/fetch/cache.go`
  - `internal/fetch/cache_test.go`
  - `internal/engine/engine.go`
  - `internal/engine/fetch_only.go`
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `README.md`
- **Behavior impact**:
  - FiVi now reliably reuses last known-good source data even when sources are blocked, empty, or temporarily skipped by cooldown.
  - Cache persistence is no longer tied to invocation directory.
  - Default startup path favors compatibility (`xray + proxy`) while preserving explicit user control for tunnel and engine selection.
- **Validation performed**:
  - `go test ./...` → pass (`174` tests)
  - `go build ./...` → pass
  - Manual binary checks:
    - `bins/xray/amd64/xray version` → `Xray 26.2.6`
    - `bins/singbox/amd64/sing-box version` → `sing-box 1.12.22`
    - Both binaries are executable Linux `ELF 64-bit`.
- **Next task or blocker**:
  - Next planned implementation remains Milestone L (`L1` KDE proxy support).

## Context Update — 2026-03-02 (Milestone L1: KDE system proxy)

- **Task ID and date**: `L1-2026-03-02`
- **Summary of changes**:
  - Added KDE system proxy backend using `kwriteconfig6/kwriteconfig5` targeting `~/.config/kioslaverc` `[Proxy Settings]` keys.
  - Added safe restore behavior: previous `kioslaverc` content is restored on disconnect/exit (or removed if it did not exist before).
  - Updated `sysproxy.Enable` to prefer KDE backend on KDE/Plasma sessions and to fall back to KDE backend when GNOME `gsettings` is unavailable.
  - Added unit tests for KDE command generation, proxy formatting, and KDE session detection.
  - Marked `L1` as completed in `plans/TASKS.md`.
- **Files touched**:
  - `internal/sysproxy/kde.go`
  - `internal/sysproxy/kde_test.go`
  - `internal/sysproxy/gnome.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - FiVi can now configure system proxy on KDE Plasma environments and restore prior settings on teardown.
  - Existing GNOME proxy behavior remains unchanged for GNOME sessions.
- **Validation performed**:
  - `go test ./internal/sysproxy` (targeted via test tool) → pass
  - `go test ./...` → pass (`177` tests)
  - `go build ./...` → pass
- **Next task or blocker**:
  - Next milestone item is `L2` (macOS system proxy support via `networksetup`).

## Context Update — 2026-03-02 (Milestone L3: Windows system proxy)

- **Task ID and date**: `L3-2026-03-02`
- **Summary of changes**:
  - Added Windows system proxy backend that uses PowerShell to manage registry keys under `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`.
  - Implemented state snapshot (`ProxyEnable`, `ProxyServer`, `ProxyOverride`) before enabling proxy and restore callback to revert settings on disconnect/exit.
  - Added script builders for set/restore flows, including safe PowerShell string escaping.
  - Wired `sysproxy.Enable` dispatch so Windows runtime uses the Windows backend first.
  - Marked `L3` as completed in `plans/TASKS.md`.
- **Files touched**:
  - `internal/sysproxy/windows.go`
  - `internal/sysproxy/windows_test.go`
  - `internal/sysproxy/gnome.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - FiVi now supports automatic Windows system proxy enable/restore in proxy mode.
  - Linux GNOME/KDE behavior remains available and unchanged on non-Windows systems.
- **Validation performed**:
  - `go test ./internal/sysproxy` (targeted via test tool) → pass
  - `go test ./...` → pass (`180` tests)
  - `go build ./...` → pass
- **Next task or blocker**:
  - `L2` is intentionally skipped per current direction; next practical milestone item is `L4` (Linux desktop environment auto-detection dispatch).

## Context Update — 2026-03-02 (Milestone L4: Linux DE auto-detection)

- **Task ID and date**: `L4-2026-03-02`
- **Summary of changes**:
  - Added explicit Linux desktop environment detection module with GNOME/KDE/unknown classification.
  - Updated `sysproxy.Enable` to dispatch by Linux desktop environment:
    - KDE → KDE backend
    - GNOME → GNOME backend
    - Unknown DE → warning + best-effort fallback (GNOME if `gsettings` exists, else KDE backend)
  - Refactored GNOME implementation into a dedicated `enableGNOME` helper.
  - Removed duplicated KDE session detection helper from KDE backend and centralized detection logic.
  - Added detection-focused tests and updated KDE-related test accordingly.
  - Marked `L4` as completed in `plans/TASKS.md`.
- **Files touched**:
  - `internal/sysproxy/detect.go`
  - `internal/sysproxy/detect_test.go`
  - `internal/sysproxy/gnome.go`
  - `internal/sysproxy/kde.go`
  - `internal/sysproxy/kde_test.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Linux system proxy setup now auto-selects GNOME/KDE implementation without manual intervention.
  - Unsupported Linux desktops degrade gracefully with warning and backend fallback attempts.
- **Validation performed**:
  - `go test ./internal/sysproxy` (targeted via test tool) → pass
  - `go test ./...` → pass (`186` tests)
  - `go build ./...` → pass
- **Next task or blocker**:
  - `L2` remains skipped by direction; next actionable milestone item is `L5` (LAN sharing mode).
