# FiVi Session Context

## Project Snapshot
- Project: `FiVi` (Go CLI)
- Purpose: discover, validate, rank, connect, and maintain public V2Ray links with failover
- Engines: `xray` (default), `sing-box`
- Modes: `proxy` (SOCKS5/HTTP), `tunnel` (TUN system-wide)
- Branch: `staging`

## Codebase Structure

```
fivi/
├── main.go                         # Entry point, subcommand routing (fetch), flag parsing, mode dispatch
├── go.mod / go.sum                 # Module: fivi, Go 1.23+
├── plans/
│   ├── PRD.md                      # Product requirements (Phases 1-9)
│   ├── TDD.md                      # Technical design
│   ├── TASKS.md                    # Task list (Milestones A-M)
│   └── CONTEXT.md                  # This file
├── bins/                           # Engine binaries (xray/, singbox/ by arch)
├── output/                         # Runtime artifacts (all_configs.txt, working/probable, source_report.json)
├── internal/
│   ├── config/config.go            # Flag parsing, AppConfig struct, binary resolution
│   ├── engine/
│   │   ├── engine.go               # Core loop: discover → rank → connect → monitor → failover
│   │   ├── events.go               # WorkerEvent, StatusUpdate, Phase types, emitStatus
│   │   ├── fetch_only.go           # ./fivi fetch subcommand implementation
│   │   ├── *_test.go               # scoring, failover integration, benchmark, readiness, etc.
│   ├── engines/
│   │   ├── adapter.go              # Adapter/Instance interfaces, Parsed metadata
│   │   ├── xray.go                 # XrayAdapter (TUN-aware with route management)
│   │   ├── singbox.go              # SingboxAdapter (DNS, sniffing, TUN/proxy, NAT)
│   │   └── *_test.go
│   ├── xray/
│   │   ├── config.go               # Xray JSON config builder (DNS, sniffing, IPIfNonMatch, TUN, sockopt marks)
│   │   ├── process.go              # Start/stop xray subprocess
│   │   ├── route_linux.go          # Linux TUN route setup (ip rule/route)
│   │   ├── route_other.go          # Stub for non-Linux
│   │   └── config_test.go
│   ├── fetch/
│   │   ├── fetch.go                # FetchAll from URLs, decode (text, base64, JSON)
│   │   ├── cache.go                # File-based cache for source resilience
│   │   └── *_test.go
│   ├── parser/
│   │   ├── candidates.go           # Normalize: dedup, scheme filter, shuffle, truncate
│   │   └── candidates_test.go
│   ├── probe/probe.go              # SOCKS proxy testing (CheckThroughSOCKS, MeasureDownloadSpeed)
│   ├── sources/
│   │   ├── registry.go             # 30 default source URLs
│   │   ├── health.go               # HealthTracker (cooldown, backoff, yield)
│   │   └── *_test.go
│   ├── profile/
│   │   ├── profile.go              # iran/global profiles (targets, timeouts, required tokens)
│   │   └── profile_test.go
│   ├── ui/
│   │   ├── mode.go                 # TTY detection, TUI vs log mode
│   │   ├── dashboard.go            # Bubble Tea TUI (phase-aware: discovery + monitoring views)
│   │   ├── progress.go             # Log mode reporter (phase + health metrics)
│   │   └── *_test.go
│   ├── sysproxy/
│   │   ├── gnome.go                # GNOME gsettings proxy (Enable → restore func)
│   │   └── gnome_test.go
│   ├── binresolver/resolver.go     # Auto-detect engine binaries from bins/ directory
│   └── logx/logx.go               # Structured logging with output redirection
```

## Completion Status

### Done
- [x] Milestone A (`A1`..`A5`) — Source expansion, health tracking, cooldown, JSON feeds, reports
- [x] Milestone B (`B1`..`B5`) — Per-candidate timeout, readiness checks, failure taxonomy, probable bucket, cancellation safety
- [x] Milestone C (`C1`..`C4`) — TUI mode detection, worker events, Bubble Tea dashboard, log progress
- [x] Milestone D (`D1`..`D4`) — Engine adapter interface, xray/singbox adapters, engine flags, clash-meta deferred
- [x] Milestone E (`E1`..`E3`) — Profile system (iran default), tuned targets/timeouts, docs
- [x] Milestone F (`F1`..`F4`) — Scoring tests, failover integration tests, benchmarks, release docs
- [x] Milestone G (`G1`..`G7`) — V2rayN-quality configs (DNS, sniffing, private IP routing), status events, TUI redesign
- [x] Milestone H (`H1`..`H4`) — Xray TUN mode, max-candidates removal + shuffle, ./fivi fetch, GNOME system proxy

### Pending (New — from oblivion-desktop comparison)
- [ ] Milestone I (`I1`..`I5`) — Connection resilience: latency pre-scan, post-connect verify, warm standby, graceful cleanup, auto-reconnect
- [ ] Milestone J (`J1`..`J6`) — Routing rules, DNS config, geo routing, ConfigBuildOpts refactor
- [ ] Milestone K (`K1`..`K3`) — Speed test, network stats, PAC script
- [ ] Milestone L (`L1`..`L5`) — Cross-platform proxy (KDE, macOS, Windows), LAN sharing
- [ ] Milestone M (`M1`..`M4`) — Binary auto-management, session persistence, daemon mode

## Key Technical Decisions

### Engine Configs
- **Xray**: DNS section (8.8.8.8:53), sniffing on inbounds, IPIfNonMatch domain strategy, explicit private CIDR ranges (no geoip.dat), sockopt mark 255 for TUN loop prevention
- **Singbox**: Dual DNS (remote tls://8.8.8.8 via proxy, local 223.5.5.5 via direct), DNS loop prevention (outbound:any → local-dns), sniffing, dns-out outbound, ip_is_private for private IPs, NAT for TUN
- **TUN routing**: Linux `ip rule add not fwmark 255 table 100` + `ip route add default dev xray0 table 100`

### Architecture Patterns
- **Adapter interface**: `engines.Adapter` with `Name()`, `EnsureBinary()`, `BuildRuntimeConfigFromURI()`, `Start()` → `Instance`
- **Status events**: Two channels — `WorkerEvent` (per-candidate progress) + `StatusUpdate` (engine-level phases)
- **Non-blocking emission**: `emitStatus()` and `emitStageEvent()` use select-default to never block engine
- **Source resilience**: File-based cache for offline fallback when live fetch fails in restricted networks
- **Candidate diversity**: Random shuffle instead of sort to prevent protocol bias under max-candidates cap

### Upcoming Architecture Changes (Milestone J)
- `BuildRuntimeConfigFromURI` will be refactored to accept `ConfigBuildOpts` struct
- This struct will carry routing rules, DNS config, geo bypass settings, and share mode
- Backward compatible: zero-value opts = current behavior

## Comparison: FiVi vs Oblivion-Desktop

### What Inspired New Features
| Oblivion Feature | FiVi Equivalent (Planned) | Milestone |
|---|---|---|
| Endpoint scanner (RTT-based WARP endpoint discovery) | Latency pre-scan (TCP connect to reject slow configs) | I1 |
| Connection verification (15-retry loop) | Post-connect verification loop (5 retries) | I2 |
| Routing rules (domain, ip, range, app, negated) | Routing rules (domain, ip, cidr, negated domain) | J1-J3 |
| Configurable DNS (plain + DoH) | Configurable DNS (--dns, --doh) | J4 |
| Geo-based routing (geoip/geosite rule sets) | Geo bypass (--geo-bypass) | J5 |
| Cloudflare speed test (download, upload, latency, jitter) | Full speed test subcommand | K1 |
| Network statistics (zag-netStats binary) | Native Go netstats from /proc/net/dev | K2 |
| PAC script for SOCKS proxy | PAC script server | K3 |
| Cross-platform proxy (Windows registry, macOS networksetup, KDE) | Cross-platform sysproxy expansion | L1-L4 |
| LAN sharing (bind 0.0.0.0) | --share flag | L5 |
| Binary version management | ./fivi install + update commands | M1-M2 |
| Auto-connect/restart counter | Session persistence + daemon mode | M3-M4 |
| Proxy-reset safety net on crash | Graceful cleanup + signal handling | I4 |
| Warm failover (oblivion retries same WARP) | Warm standby (top-3 pre-validated configs) | I3 |

### FiVi's Unique Strengths (Not in Oblivion)
- **Multi-source config discovery**: 30+ public sources vs single WARP
- **Multi-protocol**: vmess, vless, trojan, ss
- **Parallel candidate validation**: Worker pool with event stream
- **Source health tracking**: Cooldown, backoff, yield metrics
- **Weighted scoring**: Success ratio + latency + blocked-target reachability

## Verified Environment
- Go 1.23+
- Xray binary: `Xray 25.10.15` (Linux ELF 64-bit, static)
- Singbox binary: `sing-box 1.12.22` (verified via smoke run)
- Platform: Linux (Debian-based, GNOME)
- Both `go test ./...` and `go build ./...` pass

## Git Status
- Branch: `staging`
- Latest commit: `b78f9c8` — feat: TUN mode for xray, add fetch command, remove limit for candidates number and etc
- All milestones A-H are committed
- Working tree is clean

## Next Recommended Execution
1. Start with **Milestone I** (connection resilience) — highest impact for "hard-ass VPN" goal.
2. `I1` (latency pre-scan) is the best first task: standalone, testable, immediately improves speed.
3. `I4` (graceful cleanup) should be done early — it's a safety feature that prevents broken states.
4. After I, proceed to **Milestone J** (routing rules + DNS) — highest user-facing feature value.

## Context Update Protocol (mandatory)
After each completed task, append a section to this file:
- Task ID and date
- Summary of changes
- Files touched
- Behavior impact
- Validation performed (`go test`, `go build`, manual)
- Next task or blocker
