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

---

## Task J1: Routing Rule Parser and Data Model — 2026-02-28

### Summary
Added `internal/routing` package with a complete routing rule parser supporting domain (exact + wildcard), IP, and CIDR range rules with negation. Includes CSV and file loading, profile-based defaults (Iran profile has common Iranian domains and IP ranges as direct-bypass rules), and helper functions for categorizing rules.

### Changes
- **New** `internal/routing/rules.go` — Rule data model (`RuleType`, `Rule` struct), `ParseRule()`, `ParseRules()`, `ParseCSV()`, `LoadFromFile()`, `Split()`, filter helpers (`DomainRules`, `IPRules`, `RangeRules`), `IranDefaults` preset, `ProfileDefaults()`.
- **New** `internal/routing/rules_test.go` — 26 tests covering all rule types, negation, wildcards, IPv6, CIDR normalization, error cases, CSV/file loading, profile defaults, split and filter functions.
- **Modified** `internal/config/config.go` — Added `RoutingRules []routing.Rule` to `AppConfig`. Added `--routing-rules` (inline CSV) and `--routing-rules-file` (file path) flags. Auto-loads profile defaults when no explicit rules provided.

### Files Touched
- `internal/routing/rules.go` (new)
- `internal/routing/rules_test.go` (new)
- `internal/config/config.go` (modified)

### Behavior Impact
- Users can specify routing rules via `--routing-rules "domain:*.ir,ip:1.2.3.4"` or `--routing-rules-file /path/to/rules.txt`.
- Iran profile automatically loads default direct-bypass rules for common Iranian domains (*.ir, digikala, snapp, etc.) and IP ranges (MTN, MCI, etc.).
- Rules are parsed and validated at startup — invalid rules cause immediate exit with error.
- File-based rules take priority over inline CSV; both override profile defaults.
- Rules are stored in `AppConfig.RoutingRules` for consumption by engine config builders (J2/J3).

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass (26 new routing tests + full suite)

### Next Task
`J2` — Integrate routing rules into xray config builder

---

## Task J2: Integrate Routing Rules into Xray Config Builder — 2026-02-28

### Summary
Extended `BuildRuntimeConfigFromURI` across the entire adapter chain to accept `[]routing.Rule`. The xray config builder now converts rules into xray-native routing format with correct priority ordering: private IP → negated (proxy) → direct → catch-all.

### Changes
- **Modified** `internal/xray/config.go` — Added `rules []routing.Rule` parameter to `BuildRuntimeConfigFromURI`. Added `buildXrayRouteRules()`, `collectDomains()`, `collectIPs()` helpers. Exact domains use `full:` prefix, wildcard domains use `domain:` prefix. Negated rules get `outboundTag: "proxy"`, normal rules get `outboundTag: "direct"`.
- **Modified** `internal/engines/adapter.go` — Updated `Adapter` interface: `BuildRuntimeConfigFromURI` now accepts `rules []routing.Rule`.
- **Modified** `internal/engines/xray.go` — Updated `XrayAdapter` to pass rules through to `xray.BuildRuntimeConfigFromURI`.
- **Modified** `internal/engines/singbox.go` — Updated `SingboxAdapter` signature to accept `_ []routing.Rule` (ignored until J3).
- **Modified** `internal/engine/engine.go` — Updated callers in `testOne()` and `startCandidate()` to pass `cfg.RoutingRules`.
- **Modified** `internal/engine/benchmark_test.go` — Updated mock adapter signature.
- **Modified** `internal/engine/failover_integration_test.go` — Updated mock adapter signature.
- **Modified** `internal/engines/xray_test.go` — Added `nil` rules argument to existing tests.
- **Modified** `internal/engines/singbox_test.go` — Added `nil` rules argument to all existing tests.
- **Modified** `internal/xray/config_test.go` — Added `TestBuildRuntimeConfigWithRoutingRules` (verifies rule ordering, domain prefixes, negation) and `TestBuildRuntimeConfigNoRulesDefaultBehavior` (nil rules = 2 rules).

### Files Touched
- `internal/xray/config.go` (modified)
- `internal/xray/config_test.go` (modified)
- `internal/engines/adapter.go` (modified)
- `internal/engines/xray.go` (modified)
- `internal/engines/singbox.go` (modified)
- `internal/engines/xray_test.go` (modified)
- `internal/engines/singbox_test.go` (modified)
- `internal/engine/engine.go` (modified)
- `internal/engine/benchmark_test.go` (modified)
- `internal/engine/failover_integration_test.go` (modified)

### Behavior Impact
- Xray routing config now includes user-defined routing rules between private IP and catch-all rules.
- Rule priority order: private IPs (direct) → negated rules (force proxy) → normal rules (direct) → catch-all (proxy).
- Backward compatible: `nil` or empty rules produce the same 2-rule config as before.
- Iran profile's default rules (*.ir domains + Iranian IP ranges) are automatically applied when using xray engine.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass (2 new xray routing tests + full suite)

### Next Task
`J3` — Integrate routing rules into singbox config builder

---

## Task J3: Integrate Routing Rules into Singbox Config Builder — 2026-02-28

### Summary
Wired routing rules into the singbox config builder. Added `buildSingboxRouteRules()` and `singboxRulesForOutbound()` to convert `[]routing.Rule` into singbox-native route rule format using `domain`, `domain_suffix`, and `ip_cidr` fields.

### Changes
- **Modified** `internal/engines/singbox.go` — Changed `_ []routing.Rule` to `rules []routing.Rule` in `BuildRuntimeConfigFromURI`. Replaced inline route rules with `buildSingboxRouteRules()` call. Added `buildSingboxRouteRules()` (constructs full rule array) and `singboxRulesForOutbound()` (groups rules by type for a given outbound). Exact IPs get `/32` suffix for `ip_cidr` format.
- **Modified** `internal/engines/singbox_test.go` — Added `TestSingboxRoutingRulesIntegration` (verifies 7-rule order with mixed rule types, domain/domain_suffix/ip_cidr fields) and `TestSingboxNoRoutingRulesDefaultBehavior` (nil rules = 3 rules).

### Files Touched
- `internal/engines/singbox.go` (modified)
- `internal/engines/singbox_test.go` (modified)

### Behavior Impact
- Singbox configs now include user routing rules between DNS/private-IP rules and catch-all.
- Rule priority: dns-out → private IP direct → negated (proxy) → normal (direct) → catch-all proxy.
- Wildcard domains → `domain_suffix`, exact domains → `domain`, IPs → `ip_cidr` with /32, CIDRs → `ip_cidr` as-is.
- Backward compatible: nil/empty rules produce the same 3-rule config as before.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass (2 new singbox routing tests + full suite)

### Next Task
`J4` — Add configurable DNS with user overrides

---

## Task J4: Configurable DNS with User Overrides — 2026-02-28

### Summary
Added `internal/dns` package with `Config` struct and DNS loop prevention. Added `--dns` and `--doh` flags. Updated both xray and singbox config builders to use configurable DNS instead of hardcoded values. Singbox builder adds a bootstrap DNS server when the remote DNS uses a hostname.

### Changes
- **New** `internal/dns/config.go` — `Config` struct (Remote, Direct), `Default()`, `NeedsBootstrap()`, `BootstrapAddr()`, type detection helpers (`RemoteIsDoH`, `RemoteIsDoT`, `RemoteIsPlain`), `extractHost()`.
- **New** `internal/dns/config_test.go` — 10 tests covering defaults, bootstrap detection, type detection, host extraction.
- **Modified** `internal/config/config.go` — Added `DNS dns.Config` to `AppConfig`. Added `--dns` and `--doh` flags. Builds `dns.Config` from flags with defaults.
- **Modified** `internal/xray/config.go` — Added `dnsCfg dns.Config` parameter. Added `buildXrayDNS()` and `extractDNSHost()`. Supports plain, DoT (tcp://host:853), and DoH DNS.
- **Modified** `internal/engines/singbox.go` — Added `dnsCfg dns.Config` parameter. Added `buildSingboxDNS()` with bootstrap DNS for hostname-based remote resolvers.
- **Modified** `internal/engines/adapter.go` — Updated `Adapter` interface with `dns.Config` parameter.
- **Modified** `internal/engines/xray.go` — Updated `XrayAdapter` to pass `dnsCfg` through.
- **Modified** all test files — Updated signatures with `dns.Config{}` zero value.

### Files Touched
- `internal/dns/config.go` (new)
- `internal/dns/config_test.go` (new)
- `internal/config/config.go` (modified)
- `internal/xray/config.go` (modified)
- `internal/xray/config_test.go` (modified)
- `internal/engines/adapter.go` (modified)
- `internal/engines/xray.go` (modified)
- `internal/engines/singbox.go` (modified)
- `internal/engines/xray_test.go` (modified)
- `internal/engines/singbox_test.go` (modified)
- `internal/engine/engine.go` (modified)
- `internal/engine/benchmark_test.go` (modified)
- `internal/engine/failover_integration_test.go` (modified)

### Behavior Impact
- `--doh tls://1.1.1.1` overrides the proxy-routed DNS resolver.
- `--dns 4.2.2.4` overrides the direct DNS resolver.
- DNS loop prevention: when remote DNS uses a hostname (e.g. `https://dns.google/dns-query`), singbox config gets a bootstrap DNS server to resolve that hostname via direct.
- Defaults unchanged: remote `tls://8.8.8.8` (proxy), direct `223.5.5.5`.
- Zero-value `dns.Config{}` falls back to hardcoded defaults in builders, preserving backward compatibility.

### Validation
- `go build ./...` — passes
- `go test ./...` — all tests pass (10 new dns tests + full suite)

### Next Task
`J5` — Add geo-based routing for singbox

---

## Task J5: Geo-Based Routing for Singbox — 2026-02-28

### Summary
Added `--geo-bypass` flag for comprehensive country-based traffic routing. For singbox, downloads `.srs` binary rule set files (geoip + geosite) from Chocolate4U/Iran-sing-box-rules, caches them in `~/.fivi/rulesets/` with 7-day refresh, and injects `rule_set` definitions and route rules into the singbox config. For xray, the existing explicit routing rules system handles geo bypass. Used a `GeoAware` interface pattern to avoid modifying the main `Adapter` interface (will be cleaned up in J6).

### Changes
- **New** `internal/routing/geo.go` — `GeoRuleSets` struct, `EnsureGeoRuleSets(country, cacheDir)` with download/cache logic, `DefaultGeoCacheDir()`, `SupportedGeoCountries()`, 7-day freshness check, atomic file writes via `.tmp` rename.
- **New** `internal/routing/geo_test.go` — 7 tests: supported countries, unsupported country error, download with httptest mock, caching (no re-download), stale refresh (8-day-old files re-downloaded), freshness check, HTTP error handling.
- **Modified** `internal/engines/adapter.go` — Added `GeoAware` interface with `SetGeoRuleSets(geo *routing.GeoRuleSets)` method.
- **Modified** `internal/engines/singbox.go` — Added `Geo *routing.GeoRuleSets` field to `SingboxAdapter`, implemented `SetGeoRuleSets()`. Updated `buildSingboxRouteRules()` to accept `geo` param and insert geo rule set references between negated and normal rules. Added `buildSingboxGeoRuleSetDefs()` to create local binary rule set definitions.
- **Modified** `internal/config/config.go` — Added `GeoBypass string` to `AppConfig`, added `--geo-bypass` flag.
- **Modified** `internal/engine/engine.go` — Added geo resolution at startup in `RunWithAdapter`: checks `GeoAware` interface, calls `EnsureGeoRuleSets`, sets on adapter. For non-GeoAware adapters (xray), logs informational message.
- **Modified** `internal/engines/singbox_test.go` — Added 4 new tests: `TestSingboxGeoBypassAddsRuleSets` (verifies rule set defs + route rules), `TestSingboxGeoBypassWithRoutingRules` (geo + user rules ordering), `TestSingboxNoGeoBypassNoRuleSetDefs` (no rule_set when geo disabled), and validates rule priority ordering.

### Files Touched
- `internal/routing/geo.go` (new)
- `internal/routing/geo_test.go` (new)
- `internal/engines/adapter.go` (modified)
- `internal/engines/singbox.go` (modified)
- `internal/engines/singbox_test.go` (modified)
- `internal/config/config.go` (modified)
- `internal/engine/engine.go` (modified)

### Behavior Impact
- `--geo-bypass ir` downloads Iran rule sets and routes all Iranian IPs/domains directly via singbox.
- Rule sets are cached in `~/.fivi/rulesets/` for 7 days, then auto-refreshed.
- Rule priority order with geo: dns-out → private IP → negated (proxy) → geo rule sets (direct) → user rules (direct) → catch-all (proxy).
- Singbox config gets `route.rule_set` definitions (local binary format) and `rule_set` route rules.
- For xray engine, `--geo-bypass` is accepted but geo bypass uses existing explicit routing rules (CIDR lists from profile defaults).
- No interface change to `Adapter` — uses optional `GeoAware` interface pattern.
- Backward compatible: without `--geo-bypass`, behavior is identical to before.

### Validation
- `go build ./...` — passes
- `go test ./...` — all 15 test packages pass (7 new geo tests + 4 new singbox geo tests)

### Next Task
`J6` — Extend `ConfigBuildOpts` to pass routing/DNS/geo to adapters

---

## Task J6: Refactor to ConfigBuildOpts Struct — 2026-02-28

### Summary
Refactored the `Adapter.BuildRuntimeConfigFromURI` method (6 parameters) into `Adapter.BuildConfig(opts ConfigBuildOpts)` using a single options struct. Eliminated the `GeoAware` interface by including `Geo *routing.GeoRuleSets` directly in the opts. Added `GeoRuleSets` field to `AppConfig` so resolved geo data flows through without threading new parameters through the entire call chain.

### Changes
- **Modified** `internal/engines/adapter.go` — Added `ConfigBuildOpts` struct (RawURI, SocksAddr, HTTPAddr, TunnelMode, Rules, DNS, Geo). Renamed interface method from `BuildRuntimeConfigFromURI` to `BuildConfig(opts ConfigBuildOpts)`. Removed `GeoAware` interface (no longer needed).
- **Modified** `internal/engines/xray.go` — Updated `XrayAdapter.BuildConfig` to unpack opts and call `xray.BuildRuntimeConfigFromURI`. Removed unused `dns` and `routing` imports.
- **Modified** `internal/engines/singbox.go` — Removed `Geo` field and `SetGeoRuleSets` from `SingboxAdapter`. Updated `BuildConfig` to read all parameters from opts, including `opts.Geo` for geo rule sets.
- **Modified** `internal/config/config.go` — Added `GeoRuleSets *routing.GeoRuleSets` to `AppConfig` (resolved at engine startup).
- **Modified** `internal/engine/engine.go` — Stores resolved geo rule sets in `cfg.GeoRuleSets` instead of using `GeoAware` interface. Updated `testOne()` and `startCandidate()` to build `engines.ConfigBuildOpts` structs.
- **Modified** `internal/engines/singbox_test.go` — Converted all `BuildRuntimeConfigFromURI(...)` calls to `BuildConfig(ConfigBuildOpts{...})`. Geo tests now pass `Geo` via opts instead of `SetGeoRuleSets`. Removed `dns` import.
- **Modified** `internal/engines/xray_test.go` — Converted to `BuildConfig(ConfigBuildOpts{...})`. Removed `dns` import.
- **Modified** `internal/engine/benchmark_test.go` — Updated mock adapter to implement `BuildConfig`. Removed `dns` and `routing` imports.
- **Modified** `internal/engine/failover_integration_test.go` — Updated mock adapter to implement `BuildConfig`. Removed `dns` and `routing` imports.

### Files Touched
- `internal/engines/adapter.go` (modified)
- `internal/engines/xray.go` (modified)
- `internal/engines/singbox.go` (modified)
- `internal/engines/singbox_test.go` (modified)
- `internal/engines/xray_test.go` (modified)
- `internal/config/config.go` (modified)
- `internal/engine/engine.go` (modified)
- `internal/engine/benchmark_test.go` (modified)
- `internal/engine/failover_integration_test.go` (modified)

### Behavior Impact
- Cleaner adapter interface: `BuildConfig(opts)` instead of 6 positional parameters.
- `GeoAware` interface eliminated — geo config flows through `ConfigBuildOpts.Geo` and `AppConfig.GeoRuleSets`.
- Fully backward compatible: zero-value `ConfigBuildOpts{}` produces the same configs as before.
- Future options (e.g. ShareMode) can be added to `ConfigBuildOpts` without changing the interface.

### Validation
- `go build ./...` — passes
- `go test ./...` — all 15 test packages pass

### Next Milestone
Milestone J complete (J1–J6). Next: **Milestone K**

## Context Update Protocol (mandatory)
After each completed task, append a section to this file:
- Task ID and date
- Summary of changes
- Files touched
- Behavior impact
- Validation performed (`go test`, `go build`, manual)
- Next task or blocker
