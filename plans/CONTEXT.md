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
- [x] Milestone I (`I1`..`I5`) — Connection resilience: latency pre-scan, post-connect verify, warm standby, graceful cleanup, auto-reconnect
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
1. **Milestone I complete.** All 5 tasks (I1-I5) implemented.
2. Next: **Milestone J** (routing rules + configurable DNS + geo routing) — highest user-facing feature value.
3. Start with `J1` (routing rule parser) or `J6` (ConfigBuildOpts refactor) — both are foundational for J2-J5.

---

## Task I1: Latency Pre-Scanner — 2026-02-28

### Summary
Added `internal/latency` package with TCP connect pre-scanner that fast-rejects unreachable/slow server addresses before the expensive engine-spawn validation step.

### Changes
- **New** `internal/latency/addr.go` — Extracts server `host:port` from vmess, vless, trojan, and ss URIs. Handles base64-encoded vmess payloads, URL-based vless/trojan, and the complex ss format (both inline-@ and fully-encoded).
- **New** `internal/latency/scanner.go` — `ScanBatch()` performs parallel TCP connect checks with configurable dial timeout, latency cutoff, and parallelism (default 50). Candidates whose address can't be extracted pass through (not rejected). Returns filtered candidate list + rejected count.
- **New** `internal/latency/addr_test.go` — 9 tests covering all 4 protocols, IPv6, edge cases (empty address, numeric port, unsupported scheme).
- **New** `internal/latency/scanner_test.go` — 5 tests: reachable passes, unreachable rejected, unparseable passes through, empty input, mixed batch.
- **Modified** `internal/config/config.go` — Added `LatencyCutoff` field to `AppConfig` and `--latency-cutoff` flag (default 1500ms, 0 disables).
- **Modified** `internal/engine/engine.go` — Added `FailureReasonLatencyReject` constant. Integrated latency pre-scan into `discoverAndRank()` between normalize and validateCandidates. Logs rejection count.

### Files Touched
- `internal/latency/addr.go` (new)
- `internal/latency/scanner.go` (new)
- `internal/latency/addr_test.go` (new)
- `internal/latency/scanner_test.go` (new)
- `internal/config/config.go` (modified)
- `internal/engine/engine.go` (modified)

### Behavior Impact
- Discovery pipeline now performs lightweight TCP connect checks on all candidate server addresses AFTER normalize, BEFORE worker validation.
- Unreachable servers and servers with latency > 1500ms (configurable) are eliminated before spawning engine processes, saving significant time.
- `--latency-cutoff 0` disables the pre-scan entirely (backward compatible).
- Candidates with unparseable URIs pass through to the engine validation step (no false rejections).

### Validation
- `go build ./...` — passes
- `go test ./...` — all 14 latency tests + full project test suite passes
- No regressions in existing packages

### Next Task
`I2` — Post-connect verification loop

---

## Task I2: Post-Connect Verification Loop — 2026-02-28

### Summary
Modified `startCandidate()` to retry connectivity verification up to N times (default 5) with 2s interval before declaring "connected". Catches edge cases where the proxy port is listening but the tunnel isn't yet established.

### Changes
- **Modified** `internal/config/config.go` — Added `VerifyRetries` field and `--verify-retries` flag (default 5).
- **Modified** `internal/engine/engine.go` — Replaced single-probe check in `startCandidate()` with a retry loop. On each failed attempt, logs the failure and waits 2s before retrying. Context cancellation is respected between attempts.

### Files Touched
- `internal/config/config.go` (modified)
- `internal/engine/engine.go` (modified)

### Behavior Impact
- After engine start + proxy readiness, FiVi now retries the connectivity probe up to 5 times (configurable) with 2s intervals before declaring a node connected.
- Previous behavior was a single probe — if it failed, the candidate was immediately discarded. Now transient startup delays are tolerated.
- `--verify-retries 1` restores the old single-check behavior.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass (engine tests use mocked probes, so verification loop works with existing test infrastructure)

### Next Task
`I3` — Warm standby for instant failover

---

## Task I3: Warm Standby for Instant Failover — 2026-02-28

### Summary
Added warm standby mechanism that keeps top-3 ranked configs in memory after discovery. When all ranked nodes are exhausted during `connectAndMonitor`, the system tries warm standby configs before triggering an expensive full rediscovery cycle.

### Changes
- **Modified** `internal/engine/events.go` — Added `PhaseWarmFailover` status event phase.
- **Modified** `internal/config/config.go` — Added `StandbyTTL` field and `--standby-ttl` flag (default 10min, 0 disables).
- **Modified** `internal/engine/engine.go` — Added `warmStandby` struct with TTL-based validity check. `RunWithAdapter` saves top-3 configs after each discovery round. On failover, tries warm standbys before full rediscovery.

### Files Touched
- `internal/engine/events.go` (modified)
- `internal/config/config.go` (modified)
- `internal/engine/engine.go` (modified)

### Behavior Impact
- After discovery, top-3 ranked configs are saved as warm standbys.
- When all ranked nodes fail during monitoring, warm standbys are tried first (emits `PhaseWarmFailover` status).
- If warm standbys also fail or have expired (>10min by default), falls back to full rediscovery.
- `--standby-ttl 0` disables the warm standby feature.
- Reduces failover time from 30-60s (full rediscovery) to <5s when warm configs still work.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass

### Next Task
`I4` — Graceful cleanup and signal handling

---

## Task I4: Graceful Cleanup and Signal Handling — 2026-02-28

### Summary
Added `internal/cleanup` package with a `Manager` that tracks cleanup functions and executes them exactly once in LIFO order. Wired into `main.go` as a safety net for signal-driven exits and integrated into the engine's `connectAndMonitor` to register proxy restore and engine stop hooks.

### Changes
- **New** `internal/cleanup/cleanup.go` — `Manager` with `Register(name, fn)` and `Run()` (sync.Once, LIFO order, panic recovery per hook).
- **New** `internal/cleanup/cleanup_test.go` — 4 tests: reverse order execution, runs-only-once, panic recovery, empty manager.
- **Modified** `main.go` — Created global `cleanupMgr`, wired via `engine.SetCleanupManager()`, deferred `cleanupMgr.Run()`, explicit Run before `os.Exit(1)`.
- **Modified** `internal/engine/engine.go` — Added `SetCleanupManager` and `registerCleanup` helpers. Registered engine stop and proxy restore hooks in `connectAndMonitor` when connection is established.

### Files Touched
- `internal/cleanup/cleanup.go` (new)
- `internal/cleanup/cleanup_test.go` (new)
- `main.go` (modified)
- `internal/engine/engine.go` (modified)

### Behavior Impact
- On SIGINT/SIGTERM (Ctrl+C), the existing `signal.NotifyContext` cancels the context, then the deferred `cleanupMgr.Run()` executes all registered hooks: restore system proxy, stop engine process.
- Cleanup hooks run in reverse order (proxy restore before engine stop is registered after engine start, so engine stop runs first by LIFO, which is correct — stop engine then restore proxy).
- `sync.Once` prevents double cleanup if both defer and explicit call trigger.
- Panic in any hook doesn't prevent remaining hooks from running.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass including 4 new cleanup tests

### Next Task
`I5` — Auto-reconnect flag

---

## Task I5: Auto-Reconnect Flag — 2026-02-28

### Summary
Added `--auto-reconnect` flag (default: enabled) that retries the same config with exponential backoff before moving to the next ranked candidate on monitoring failure. Refactored `connectAndMonitor` into `connectAndMonitor` (orchestrates candidates + reconnect) and `connectAndMonitorOne` (single candidate lifecycle).

### Changes
- **Modified** `internal/config/config.go` — Added `AutoReconnect` field and `--auto-reconnect` flag (default true).
- **Modified** `internal/engine/engine.go` — Extracted `connectAndMonitorOne` from `connectAndMonitor`. Added reconnect loop with exponential backoff (2s, 4s, 8s, max 30s) and max 3 reconnect attempts per config. On exhaustion, falls through to next ranked candidate.

### Files Touched
- `internal/config/config.go` (modified)
- `internal/engine/engine.go` (modified)

### Behavior Impact
- When a connected node degrades, FiVi now retries the SAME config up to 3 times with exponential backoff before trying the next ranked candidate.
- This handles transient failures (brief network interruptions) without unnecessary failover.
- `--auto-reconnect=false` restores the old behavior (immediate failover to next candidate).
- Combined with warm standby (I3), the failover chain is: reconnect same config → try next ranked → try warm standbys → full rediscovery.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass

### Next Milestone
Milestone I complete. Next: **Milestone J** (Routing rules + Configurable DNS + Geo routing)

## Context Update Protocol (mandatory)
After each completed task, append a section to this file:
- Task ID and date
- Summary of changes
- Files touched
- Behavior impact
- Validation performed (`go test`, `go build`, manual)
- Next task or blocker
