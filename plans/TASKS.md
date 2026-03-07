# FiVi Implementation Tasks

## Execution Rule
At the end of each completed task, update `plans/CONTEXT.md` with:
- what changed
- files touched
- behavior impact
- validation results
- next blocker or next task

## Milestone A: Source Expansion and Reliability
1. [x] `A1` Add `internal/sources/registry.go` with all 30 `v2finder` URLs.
- Output: centralized source defaults and metadata struct.
- Done when: existing flags still work and defaults load from registry.

2. [x] `A2` Add source health tracking (`success`, `failures`, `yield`, `last_ok`).
- Output: in-memory health map and per-round updates.
- Done when: round summary prints source health.

3. [x] `A3` Add source cooldown/backoff policy for repeated failures.
- Output: cooldown window and skip logic.
- Done when: failing sources are skipped temporarily.

4. [x] `A4` Add JSON feed extraction support to decode pipeline.
- Output: parse arrays/objects for embedded links.
- Done when: sample JSON feed test passes.

5. [x] `A5` Persist source stats in output report.
- Output: `output/source_report.json`.
- Done when: report generated after each round.

## Milestone B: Pipeline Robustness
6. [x] `B1` Introduce per-candidate timeout budget.
- Output: candidate context timeout wrapped around parse/spawn/probe.
- Done when: no candidate can block indefinitely.

7. [x] `B2` Replace fixed sleeps with engine readiness checks.
- Output: readiness probing for local socks/http ports.
- Done when: startup waits are adaptive and bounded.

8. [x] `B3` Add structured failure taxonomy and counters.
- Output: typed error reasons and final summary breakdown.
- Done when: each failed candidate has explicit reason class.

9. [x] `B4` Add `probable` bucket for candidates that partially pass.
- Output: classification rules and output persistence.
- Done when: `probable_configs.txt` is generated.

10. [x] `B5` Add stress-safe worker queue cancellation paths.
- Output: leak-free worker shutdown on ctx cancel.
- Done when: tests show no deadlock and clean shutdown.

## Milestone C: TUI and Visibility
11. [x] `C1` Add `internal/ui` with TTY detection and `--no-tui` flag.
- Output: mode switch (TUI/log).
- Done when: non-interactive shells stay in log mode.

12. [x] `C2` Add worker event bus and stage events.
- Output: `WorkerEvent` stream.
- Done when: all major stages emit start/end/fail events.

13. [x] `C3` Implement Bubble Tea dashboard.
- Output: live table + progress counters + current node status.
- Done when: worker states visibly update during testing.

14. [x] `C4` Add concise plain-log progress snapshots for `--no-tui`.
- Output: periodic one-line summaries.
- Done when: users can see progress without TUI.

## Milestone D: Engine Abstraction
15. [x] `D1` Add `EngineAdapter` interface and refactor xray integration.
- Output: xray runs via adapter.
- Done when: existing behavior preserved with `--engine xray`.

16. [x] `D2` Add `sing-box` adapter.
- Output: config builder + process wrapper.
- Done when: `--engine singbox` can discover/connect/monitor.

17. [x] `D3` Add engine selection and binary flags.
- Output: `--engine`, `--xray-bin`, `--singbox-bin`.
- Done when: invalid combinations return actionable errors.

18. [x] `D4` Evaluate optional `clash-meta` adapter feasibility.
- Output: short decision note in `plans/CONTEXT.md`.
- Done when: either adapter is implemented or explicitly deferred.

## Milestone E: Restricted-Network Profile
19. [x] `E1` Add profile system and set `iran` as default.
- Output: profile config package and defaults.
- Done when: app starts with `iran` profile unless overridden.

20. [x] `E2` Tune iran probe targets/timeouts/retries/backoff.
- Output: profile-specific probe strategy.
- Done when: target logic is profile-driven, not hardcoded only.

21. [x] `E3` Add profile docs in README.
- Output: usage examples and rationale.
- Done when: users can switch profile with one flag.

## Milestone F: Quality and Release
22. [x] `F1` Add unit tests for source decode, cooldown, and scoring.
- Output: deterministic tests.
- Done when: new core logic has coverage.

23. [x] `F2` Add integration tests for failover state machine.
- Output: mocked engine/probe tests.
- Done when: failover path is reproducibly validated.

24. [x] `F3` Add benchmark for candidate throughput.
- Output: benchmark command and baseline numbers.
- Done when: baseline performance report is documented.

25. [x] `F4` Final docs and release notes.
- Output: updated README + migration notes.
- Done when: all new flags and behavior are documented.

## Milestone G: V2rayN-Quality Configs + Status Events + UI Redesign

26. [x] `G1` Xray config: add DNS, sniffing, IPIfNonMatch, private-IP routing.
- Output: V2rayN-equivalent xray runtime config with DNS section, sniffing on inbounds, IPIfNonMatch domain strategy, private CIDR direct routing.
- Done when: `TestBuildRuntimeConfigHasDNSSniffingAndRouting` passes.

27. [x] `G2` Singbox config: add DNS (remote/local with loop prevention), sniffing, dns-out, ip_is_private, NAT.
- Output: V2rayN-equivalent singbox runtime config with dual DNS servers, sniffing, dns-out outbound, private IP routing, endpoint_independent_nat for TUN.
- Done when: `TestSingboxProxyModeHasDNSAndSniffing` and `TestSingboxTunnelModeHasNATAndSniffing` pass.

28. [x] `G3` Add Phase/StatusUpdate types and emitStatus helper.
- Output: `Phase` type (discovering/connecting/monitoring/failover), `StatusUpdate` struct, non-blocking `emitStatus()`.
- Done when: `TestEmitStatusNilChannelNoOp`, `TestEmitStatusDoesNotBlockWhenFull`, `TestEmitStatusSetsTimestamp` pass.

29. [x] `G4` Thread status channel through engine and main.
- Output: `status chan<- StatusUpdate` parameter on RunWithEvents, RunWithAdapter, discoverAndRank, connectAndMonitor, monitorConnection; emitStatus calls at all phase transitions; status channel created and wired in main.go.
- Done when: `go build ./...` compiles cleanly with updated signatures.

30. [x] `G5` Redesign TUI dashboard with phase-aware views.
- Output: discovery view (progress bar, color-coded stats, sorted worker table with icons) and monitoring view (node info, ping/speed/probes, proxy addresses, health history).
- Done when: `TestDashboardPhaseTransition`, `TestDashboardMonitoringView`, `TestRenderProgressBar`, `TestFormatElapsed` pass.

31. [x] `G6` Update log mode progress reporter with status channel and phase logging.
- Output: phase transition logs, health check metric logs, graceful dual-channel close handling.
- Done when: `TestRunLogProgressReporterEmitsSnapshots` passes with updated signature.

32. [x] `G7` Update all UI tests for new signatures.
- Output: dashboard_test.go and progress_test.go updated for new function signatures.
- Done when: `go test ./internal/ui` passes.

## Milestone H: TUN Mode + Fetch Command + System Proxy + Config Limits

33. [x] `H1` Xray TUN mode: add native gVisor TUN inbound, sockopt mark, route management.
- Output: TUN inbound in xray config, `route_linux.go`/`route_other.go`, TUN-aware xray adapter.
- Done when: `--engine xray --mode tunnel` starts TUN without error on Linux.

34. [x] `H2` Remove max-candidates cap, replace sort with shuffle for protocol diversity.
- Output: default `--max-candidates 0`, `rand.Shuffle` in Normalize, updated tests.
- Done when: all protocols have equal chance, no artificial cap by default.

35. [x] `H3` Add `./fivi fetch` subcommand for cache-only fetching.
- Output: `fetch_only.go`, subcommand detection in `main.go`.
- Done when: `./fivi fetch` fetches all sources, saves cache, prints summary, and exits.

36. [x] `H4` Auto-configure GNOME system proxy in proxy mode.
- Output: `sysproxy/gnome.go`, integrated into `connectAndMonitor`, restore on disconnect.
- Done when: `gsettings` proxy is set on connect and restored on disconnect/exit.

---

## Milestone I: Connection Resilience + Latency Pre-Scan + Graceful Cleanup

37. [x] `I1` Add latency pre-scanner to fast-reject unreachable configs.
- Add `internal/latency` package with TCP connect scanner.
- Extract server host:port from raw V2Ray URIs (parse vmess/vless/trojan/ss address fields).
- Run batch TCP connect with 2s timeout, reject configs above `--latency-cutoff` (default 1500ms).
- Integrate into discovery pipeline AFTER normalize, BEFORE worker validation.
- Emit `candidate_latency_reject` failure reason in validation summary.
- Output: `internal/latency/scanner.go`, `scanner_test.go`, integration in `engine.go`.
- Done when: unreachable servers are skipped before spawning engine, test passes.

38. [x] `I2` Add post-connect verification loop.
- After engine start and proxy readiness, retry connectivity check up to 5 times with 2s interval.
- Only declare "connected" after first successful verification through the proxy.
- Catches cases where proxy port is listening but tunnel is not yet established.
- Configurable via `--verify-retries` (default 5).
- Output: updated `startCandidate()` in `engine.go`.
- Done when: connection is verified before monitoring starts.

39. [x] `I3` Add warm standby for instant failover.
- After ranking, keep top-3 configs in memory as warm standbys.
- On failover, try warm standby configs BEFORE triggering full rediscovery.
- Warm configs expire after configurable TTL (default 10min, `--standby-ttl`).
- Add `PhaseWarmFailover` status event.
- Output: warm standby logic in `connectAndMonitor()`, new phase event.
- Done when: failover tries standby configs first, falls back to rediscovery if all fail.

40. [x] `I4` Add graceful cleanup and signal handling.
- Register SIGINT/SIGTERM handlers in `main.go`.
- Cleanup sequence: restore system proxy → teardown TUN routes → kill engine → remove temp files.
- Use `sync.Once` to prevent double cleanup.
- Add restart counter (max 3) to prevent infinite loops.
- Output: `internal/cleanup/cleanup.go`, signal wiring in `main.go`.
- Done when: Ctrl+C cleanly restores proxy and tears down TUN, test passes.

41. [x] `I5` Add `--auto-reconnect` flag for reconnection without full rediscovery.
- On monitor failure, try restarting the SAME config before trying next ranked config.
- Exponential backoff: 2s, 4s, 8s, max 30s.
- Max reconnect attempts per config: 3.
- Only fall through to next ranked config after reconnect exhaustion.
- Output: updated `monitorConnection()` and `connectAndMonitor()` in `engine.go`.
- Done when: transient failures trigger reconnect, persistent failures trigger failover.

## Milestone J: Routing Rules + Configurable DNS + Geo Routing

42. [x] `J1` Add routing rule parser and data model.
- Add `internal/routing` package.
- Parse rule format: `domain:example.com`, `domain:*.ir`, `domain:!forced.com`, `ip:1.2.3.4`, `range:10.0.0.0/8`.
- Negated rules (`!`) have higher priority.
- Load from `--routing-rules` (inline CSV) or `--routing-rules-file` (file path, one per line).
- Add profile defaults: `iran` profile includes `domain:*.ir` and common Iranian direct domains.
- Output: `internal/routing/rules.go`, `rules_test.go`.
- Done when: rules parse correctly, defaults load from profile, tests pass.

43. [x] `J2` Integrate routing rules into xray config builder.
- Convert routing rules to xray route rule format.
- Insert domain rules as `domain` and `domain suffix` arrays in route rules.
- Insert IP/CIDR rules as `ip` arrays in route rules.
- Negated domains route to proxy outbound; normal rules route to direct.
- Place user rules AFTER private IP rule, BEFORE catch-all proxy rule.
- Output: updated `internal/xray/config.go`, tests.
- Done when: generated xray config contains user routing rules.

44. [x] `J3` Integrate routing rules into singbox config builder.
- Convert routing rules to singbox route rule format.
- Domain rules → `domain` and `domain_suffix` in route rule objects.
- IP/CIDR rules → `ip_cidr` in route rule objects.
- Negated domain rules → `outbound: "proxy"` (force through proxy).
- Normal rules → `outbound: "direct"`.
- Place user rules AFTER private IP and DNS rules, BEFORE catch-all.
- Output: updated `internal/engines/singbox.go`, tests.
- Done when: generated singbox config contains user routing rules.

45. [x] `J4` Add configurable DNS with user overrides.
- Add `--dns` flag for plain DNS (direct resolver).
- Add `--doh` flag for DNS-over-HTTPS (proxy-routed resolver).
- Add `internal/dns` package with `Config` struct.
- Implement DNS loop prevention: detect if DoH uses hostname, add bootstrap resolver.
- Update both xray and singbox config builders to use DNS config.
- Defaults: remote `tls://8.8.8.8` (proxy), direct `223.5.5.5`.
- Output: `internal/dns/config.go`, updated config builders, tests.
- Done when: `--dns` and `--doh` flags override DNS in generated configs.

46. [x] `J5` Add geo-based routing for singbox.
- Add `--geo-bypass` flag (country code: ir, cn, ru, etc.).
- Download `.srs` rule set files from Chocolate4U/Iran-sing-box-rules.
- Cache in `~/.fivi/rulesets/` with 7-day refresh.
- Add geoip + geosite route rules to singbox config for direct bypass.
- For xray: use explicit CIDR lists (no geoip.dat dependency).
- Output: `internal/routing/geo.go`, updated singbox config builder, tests.
- Done when: `--geo-bypass ir` routes Iranian traffic directly through singbox.

47. [x] `J6` Extend `ConfigBuildOpts` to pass routing/DNS/geo to adapters.
- Refactor `BuildRuntimeConfigFromURI` signature to accept `ConfigBuildOpts` struct.
- Include RoutingRules, DNSConfig, GeoBypass, ShareMode in opts.
- Update all callers (engine.go testOne, startCandidate, etc.).
- Maintain backward compatibility for testing (opts with zero values = current behavior).
- Output: updated `internal/engines/adapter.go`, all adapter implementations, caller sites.
- Done when: `go build ./...` and `go test ./...` pass with new signature.

## Milestone K: Speed Test + Network Stats + PAC Script

48. [x] `K1` Add full speed test subcommand.
- Add `./fivi speedtest` subcommand.
- Measure download speed: multiple file sizes (100KB, 1MB, 10MB) from configurable target.
- Measure upload speed: POST payloads of increasing size.
- Measure latency: multiple pings through proxy, calculate p50/p90.
- Measure jitter: variance of ping measurements.
- Run through specified SOCKS proxy (`--socks-addr`) or active connection.
- Output: `internal/speedtest/speedtest.go`, `speedtest_test.go`, subcommand in `main.go`.
- Done when: `./fivi speedtest --socks-addr 127.0.0.1:10808` prints download/upload/latency/jitter.

49. [x] `K2` Add real-time network statistics.
- Add `internal/netstats` package.
- Linux: read `/proc/net/dev` for interface counters, calculate delta per second.
- Detect correct interface: TUN interface name for tunnel mode, default interface for proxy.
- Sample every 1s, expose via channel for TUI consumption.
- Add to TUI monitoring view: sent/recv speed + total usage.
- Configurable via `--stats` flag (off by default to minimize overhead).
- Output: `internal/netstats/linux.go`, `netstats_test.go`, TUI integration.
- Done when: TUI shows real-time network stats when `--stats` is enabled.

50. [x] `K3` Add PAC script server for proxy mode.
- Add `internal/pac` package.
- Generate `FindProxyForURL` JavaScript from routing rules.
- Default: everything through `SOCKS5 127.0.0.1:<port>`.
- Bypass: localhost, private ranges, routing rule domain/IP matches.
- Serve on `localhost:<http_port+1>` via Go `net/http`.
- Auto-start in proxy mode, stop on disconnect.
- Print PAC URL in connection status.
- Output: `internal/pac/pac.go`, `pac_test.go`, integration in `engine.go`.
- Done when: PAC URL is accessible and returns valid PAC script.

## Milestone L: Cross-Platform Proxy + LAN Sharing

51. [x] `L1` Expand system proxy to KDE.
- Add `internal/sysproxy/kde.go`.
- Use `kwriteconfig5` or `kwriteconfig6` (auto-detect Plasma version).
- Write to `kioslaverc [Proxy Settings]`: ProxyType, socksProxy, NoProxyFor.
- Save and restore previous settings.
- Output: `internal/sysproxy/kde.go`, `kde_test.go`.
- Done when: KDE proxy is set on connect and restored on disconnect.

52. [ ] `L2` Expand system proxy to macOS.
- Add `internal/sysproxy/darwin.go`.
- Use `networksetup -setsocksfirewallproxy <service> <ip> <port>` for all network services.
- Set bypass domains via `networksetup -setproxybypassdomains`.
- Save and restore previous SOCKS proxy state per service.
- Output: `internal/sysproxy/darwin.go`, `darwin_test.go`.
- Done when: macOS proxy is set on connect and restored on disconnect.

53. [x] `L3` Expand system proxy to Windows.
- Add `internal/sysproxy/windows.go`.
- Use PowerShell to modify `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`.
- Set `ProxyServer`, `ProxyOverride`, `ProxyEnable`.
- Save and restore previous proxy settings.
- Output: `internal/sysproxy/windows.go`, `windows_test.go`.
- Done when: Windows proxy is set on connect and restored on disconnect.

54. [x] `L4` Auto-detect desktop environment on Linux.
- Check `$XDG_CURRENT_DESKTOP`, `$DESKTOP_SESSION`, `$GNOME_DESKTOP_SESSION_ID`.
- Select GNOME, KDE, or log warning for unsupported DE.
- Update `sysproxy.Enable` to dispatch to correct implementation.
- Output: `internal/sysproxy/detect.go`, `detect_test.go`.
- Done when: system proxy works on both GNOME and KDE without user intervention.

55. [x] `L5` Add LAN sharing mode.
- Add `--share` flag to bind proxy to `0.0.0.0` instead of `127.0.0.1`.
- Print local IP addresses and port on start for easy sharing.
- When combined with system proxy, use actual local IP (not 0.0.0.0).
- Add security warning to output.
- Output: updated `config.go`, `engine.go`.
- Done when: `--share` makes proxy accessible from LAN.

## Milestone M: Binary Management + Auto-Connect + Daemon

56. [x] `M1` Add binary auto-download command.
- Add `./fivi install` subcommand.
- Detect OS and architecture.
- Query GitHub releases API for latest xray-core and sing-box versions.
- Download appropriate binary archive, extract, place in `~/.fivi/bins/<engine>/<arch>/`.
- Verify SHA256 checksum from release assets.
- Output: `internal/binmgr/download.go`, `download_test.go`, subcommand in `main.go`.
- Done when: `./fivi install` downloads and places correct binaries.

57. [x] `M2` Add binary version check and update.
- Add `./fivi update` subcommand.
- Compare installed binary version with latest GitHub release.
- Download and replace if newer version available.
- Add `--check-update` flag for main command: warn if binaries are outdated on startup.
- Output: `internal/binmgr/update.go`, subcommand in `main.go`.
- Done when: `./fivi update` updates binaries to latest version.

58. [x] `M3` Add session persistence for fast reconnect.
- Save last working config URI + score + timestamp to `~/.fivi/session.json`.
- On startup, if session file exists and is fresh (<1h), try persisted config first.
- Skip full discovery if persisted config still works.
- Fall back to normal discovery if persisted config fails.
- Output: `internal/session/session.go`, `session_test.go`, integration in `engine.go`.
- Done when: restart tries last working config before rediscovery.

59. [x] `M4` Add daemon mode for background operation.
- Add `./fivi daemon` subcommand.
- Detach from terminal, write PID to `~/.fivi/fivi.pid`.
- Auto-reconnect on failure with exponential backoff.
- Log to file (`~/.fivi/fivi.log`).
- `./fivi stop` sends SIGTERM to daemon PID.
- `./fivi status` reads PID file and checks if daemon is running.
- Output: daemon scaffolding in `main.go`, PID management, log file rotation.
- Done when: `./fivi daemon` runs in background, `./fivi stop` stops it.

---

## Post-M Backlog (Proposed)

60. [x] `N1` Daemon hardening and health visibility.
- Add daemon command support for `restart` and `logs --tail N`.
- Add startup health marker in PID metadata to distinguish "started" vs "ready".
- Add periodic daemon heartbeat file (`~/.fivi/daemon.health.json`) with last successful probe and active node.
- Done when: users can inspect daemon health without attaching interactive UI.

61. [x] `N2` Adaptive probe target strategy.
- Add rolling probe-target quality scoring and auto-prioritize targets that best separate good/bad nodes.
- Keep profile-required targets pinned while adapting optional targets.
- Done when: average time-to-first-working-node improves without reducing required-target strictness.

62. [ ] `N3` Engine expansion feasibility: `hysteria2` adapter.
- Evaluate and optionally implement `--engine hysteria2` under existing adapter abstraction.
- Support proxy mode first; tunnel optional if low complexity.
- Done when: discovery/connect/monitor works with hysteria2 or feasibility is explicitly documented as deferred.

63. [ ] `N4` Additional mode: `urltest` continuous benchmarking mode.
- New mode where FiVi continuously benchmarks top candidates in background and switches proactively before degradation.
- Keep current mode semantics unchanged.
- Done when: active node can be preemptively switched based on stable score drop thresholds.

64. [ ] `N5` UI clarity improvements for long runs.
- Add explicit "why failed" rollup in TUI monitoring view (top failure reasons over last N checks).
- Add compact source health panel in discovery view (success/cooldown/yield top offenders).
- Mirror key TUI insights in `--no-tui` snapshots.
- Done when: users can diagnose stalls/degradation without reading verbose logs.

65. [ ] `N6` Connection quality guardrails.
- Add optional latency/jitter floors for active session, not just initial candidate qualification.
- Trigger soft failover before hard outage when metrics cross configured thresholds.
- Done when: severe quality drops trigger earlier failover with fewer user-visible outages.

66. [ ] `N7` Route/DNS runtime diagnostics.
- Add `./fivi doctor` subcommand to validate DNS/routing/proxy assumptions pre-connect.
- Include checks for loop risks, unresolved DoH bootstrap, and system-proxy command availability.
- Done when: users get actionable preflight output before starting a long discovery run.

67. [ ] `N8` Binary trust and rollout control.
- Add pinned-version channel support (`stable`, `latest`) and optional allowlist for release tags.
- Persist last known good binary version and rollback on repeated startup failures.
- Done when: binary update failures can be auto-mitigated without manual reinstall.

## Request Patchset (2026-03-07)

68. [x] `R1` Align candidate validation with selected mode.
- Ensure candidate test builds runtime config with tunnel mode when `--mode tunnel` is selected.
- Prevent proxy-only qualification from incorrectly rejecting tunnel-capable configs.
- Done when: tunnel-mode validation path uses `TunnelMode=true` in candidate test build.

69. [x] `R2` Add single-config inputs merged with subscriptions.
- Add `--config` (CSV URIs) and `--config-file` (one URI per line) to include direct configs.
- Merge direct configs with fetched source configs before normalize/dedupe.
- Done when: `--sources` and direct config inputs work together in one candidate pool.

70. [x] `R3` Probable plan-B fallback and good-latency promotion.
- Promote probe-failed candidates with known good latency to probable bucket.
- If no working candidates are found, attempt probable candidates as plan-B before rediscovery.
- Done when: probable fallback path is active and written artifacts include promoted probable entries.

71. [x] `R4` Make latency-based probable promotion configurable.
- Add `--probable-latency-threshold` to control which low-latency probe-failed candidates are promoted to probable.
- Default threshold: `450ms`; `0` disables this promotion path.
- Done when: users can tune/disable latency-based probable promotion without code changes.
