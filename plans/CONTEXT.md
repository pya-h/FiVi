# FiVi Session Context (Distilled)

## Sync Stamp
- **Synced on**: 2026-03-02
- **Scope synced**: includes all completed work through milestone `L5` plus post-L debug fix for share-mode system-proxy host selection.
- **Repository state expectation**: `staging` branch, `L2` intentionally skipped, continuation starts at `M1`.

## 1) Project Snapshot
- **Project**: `FiVi` (Go CLI) for anti-censorship proxy discovery, validation, ranking, connect/monitor/failover.
- **Branch**: `staging` (default: `main`)
- **Module**: `fivi` (Go 1.23+)
- **Primary goal**: reliable connectivity in restricted networks (Iran-focused defaults).
- **Current defaults**:
  - `--profile iran`
  - `--engine xray`
  - `--mode proxy`
  - `--startup-timeout 8s`

## 2) Current Milestone Status
- **A–K**: complete.
- **L1** (KDE sysproxy): complete.
- **L2** (macOS sysproxy): intentionally skipped for now by product direction.
- **L3** (Windows sysproxy): complete.
- **L4** (Linux DE detect dispatch): complete.
- **L5** (LAN sharing mode): complete.
- **Next planned milestone**: **M1** (binary auto-download/install management).

## 3) High-Impact Architecture Pointers
- `internal/engine/engine.go`
  - Main runtime loop: discover → rank → connect → monitor → failover.
  - System proxy setup happens in `connectAndMonitorOne()` via `enableSystemProxyFn`.
  - Share-mode LAN helpers live here (`resolveSystemProxyAddr`, LAN IP selection, share logs).
- `internal/config/config.go`
  - All CLI flag parsing/defaults.
  - Share mode (`--share`) rewrites listen host to `0.0.0.0` while preserving ports.
- `internal/sysproxy/`
  - `gnome.go`: `Enable` dispatch entrypoint + GNOME implementation.
  - `detect.go`: Linux desktop detection (`GNOME` / `KDE` / `unknown`).
  - `kde.go`: KDE `kwriteconfig5/6` implementation.
  - `windows.go`: PowerShell + registry proxy implementation.

## 4) Connection-Critical Behavior (Current)
- Proxy mode:
  - Starts SOCKS + HTTP listeners.
  - Applies system proxy (platform/DE specific).
  - Starts PAC server.
- Tunnel mode:
  - Uses adapter-specific TUN behavior.
  - No system proxy setup path.
- Auto-reconnect/failover:
  - Same-config reconnect attempts first.
  - Then next ranked candidate.
  - Then warm-standby candidates.

## 5) Recent Debug/Fix Highlights (This Session)
### A) Milestone-L connection risk fixed
- **Issue**: in `--share` mode, system-proxy host derivation could select unstable virtual/container interfaces (e.g., Docker bridge), causing client connection failures.
- **Fix**:
  - Prefer primary outbound IPv4.
  - Prefer private LAN IPs next.
  - Skip likely virtual interfaces (`docker*`, `veth*`, `br-*`, `virbr*`, `vmnet*`, `tailscale*`, `tun*`, `wg*`, etc.) when enumerating LAN IPs.
  - Fall back to loopback only if no usable LAN IP is found.
- **Files**:
  - `internal/engine/engine.go`
  - `internal/engine/share_test.go`

### B) Share-mode behavior hardening (already landed)
- `--share` binds proxy listeners to `0.0.0.0:<port>`.
- Runtime prints LAN endpoints and security warning.
- System proxy never receives wildcard host (`0.0.0.0`); it uses resolved concrete host.
- **Files**:
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `internal/engine/engine.go`

## 6) System Proxy Dispatch Rules (Current)
- On Windows runtime: use Windows backend (`powershell` registry keys).
- On Linux runtime:
  - KDE session → KDE backend.
  - GNOME session → GNOME backend.
  - Unknown DE → warning + best-effort fallback (GNOME if `gsettings` exists, else KDE backend).
- Non-Linux/non-Windows: falls through to GNOME path, which no-ops if `gsettings` missing.

## 7) Validation State
- Milestone-L focused tests pass.
- Full test suite passes: `go test ./...`.
- Build passes: `go build ./...`.

## 7.1) Latest Reliability Patch (Post-L)
- **Area**: `--share` + system proxy host resolution.
- **Problem fixed**: potential selection of unstable virtual/container interface IPs for system proxy address.
- **Current behavior**:
  - Prefer primary outbound IPv4.
  - Prefer private LAN IPv4.
  - Exclude likely virtual interfaces during LAN IP enumeration.
  - Fallback to loopback if no safe LAN IP is available.
- **Files**:
  - `internal/engine/engine.go`
  - `internal/engine/share_test.go`

## 8) Key Flags/UX That Matter for Ops
- `--mode proxy|tunnel`
- `--engine xray|singbox`
- `--share` (LAN exposure)
- `--socks-addr` / `--http-addr`
- `--auto-reconnect`
- `--verify-retries`
- `--latency-cutoff`

## 9) Known Product Decisions / Constraints
- macOS system-proxy (`L2`) deliberately postponed.
- Current direction is Linux + Windows practical reliability first.
- Keep UX CLI/TUI only (no GUI scope expansion).

## 10) Next Session Bootstrap Checklist
1. Confirm baseline still green:
   - `go test ./...`
   - `go build ./...`
2. Start **M1** (`./fivi install` + binary download/verify flow).
  - Expected outputs: `internal/binmgr/download.go`, `internal/binmgr/download_test.go`, and subcommand wiring in `main.go`.
  - Required checks: OS/arch detection, release lookup, archive extraction, checksum verification, target install path.
3. Keep changes scoped to milestone requirements (no extra feature drift).
4. After each completed task, append short context update here with:
   - changed files
   - behavior impact
   - validation results
   - next task

## 11) Milestone M1 Update (2026-03-02)
- **What changed**:
  - Implemented binary auto-download/install flow via new `./fivi install` subcommand.
  - Installer now fetches latest GitHub release metadata for `xray-core` and `sing-box`, selects OS/arch-matching archive assets, downloads checksum assets, verifies SHA256, extracts the engine binary, and installs to `~/.fivi/bins/<engine>/<arch>/`.
  - Binary resolver now probes `~/.fivi/bins/<engine>/<arch>/` before PATH fallback so installed binaries are auto-picked by normal runtime flag parsing.
- **Files touched**:
  - `main.go`
  - `internal/binmgr/download.go`
  - `internal/binmgr/download_test.go`
  - `internal/binresolver/resolver.go`
  - `internal/binresolver/resolver_test.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Users can bootstrap both engines using `./fivi install` without manual placement into repository `bins/`.
  - Standard startup path resolution now supports persisted installed binaries under user home (`~/.fivi/bins/...`).
- **Validation results**:
  - Targeted tests pass:
    - `go test ./internal/binmgr ./internal/binresolver`
  - Full suite pass:
    - `go test ./...`
- **Next task**:
  - `M2` Add binary version check/update flow (`./fivi update` + outdated binary warning path).

## 12) Milestone M2 Update (2026-03-02)
- **What changed**:
  - Implemented binary version check/update flow and added `./fivi update` subcommand.
  - Added startup `--check-update` flag to compare installed engine binaries with latest release versions and emit warning logs when outdated or missing.
  - Added update logic that resolves installed binaries, extracts installed version via `<bin> version`, compares semantic version parts, and installs latest versions when update is required.
- **Files touched**:
  - `internal/binmgr/update.go`
  - `internal/binmgr/update_test.go`
  - `main.go`
  - `internal/config/config.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - `./fivi update` now updates `xray` and `singbox` binaries to latest available release versions when older versions are installed.
  - `./fivi --check-update` now warns at startup if engine binaries are outdated/missing, without failing normal connection flow.
- **Validation results**:
  - Targeted tests pass:
    - `go test ./internal/binmgr ./internal/config`
  - Full suite and build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `M3` Add session persistence for fast reconnect.

## 13) Review + M3 Update (2026-03-02)
- **Review/debug on latest tasks (M1 + M2)**:
  - Found and fixed an M2 edge case where `UpdateStatus.Err` could remain set after a successful reinstall path, causing false failure reporting from `./fivi update`.
  - Added regression coverage to ensure successful install clears prior resolve/version-probe errors.
- **M3 implementation**:
  - Added session persistence module to save/load last working candidate in `~/.fivi/session.json`.
  - Engine startup now attempts a fresh persisted session first (max age: 1 hour) before normal discovery.
  - On successful connection establishment, engine now persists `raw_uri`, `score`, and `saved_at`.
- **Files touched**:
  - `internal/binmgr/update.go`
  - `internal/binmgr/update_test.go`
  - `internal/session/session.go`
  - `internal/session/session_test.go`
  - `internal/engine/engine.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - `./fivi update` no longer reports false engine errors when recovery install succeeds.
  - Restarted runs now try the previously working config first (when session is fresh), reducing time-to-connect.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/session ./internal/engine ./internal/binmgr`
  - Full suite and build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `M4` Add daemon mode (`daemon`, `stop`, `status`) with PID/log handling.

## 14) Milestone M4 Update (2026-03-02)
- **What changed**:
  - Added daemon lifecycle commands: `./fivi daemon`, `./fivi stop`, `./fivi status`.
  - Implemented detached daemon bootstrap (`daemon --run-child` internal mode), PID file write/remove, status checks, SIGTERM stop flow, and daemon log file setup with size-based rotation (`fivi.log` -> `fivi.log.1` at ~5MB).
  - Daemon runtime forces log mode (`--no-tui`) and reuses normal engine run path for auto-reconnect/failover behavior.
- **Files touched**:
  - `main.go`
  - `internal/daemon/daemon.go`
  - `internal/daemon/detach_unix.go`
  - `internal/daemon/detach_windows.go`
  - `internal/daemon/daemon_test.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Users can start FiVi in background mode and manage lifecycle using PID-based commands.
  - Daemon logs persist to `~/.fivi/fivi.log` with lightweight rotation.
  - Stale PID file entries are cleaned up by `status` when process is not running.
- **Validation results**:
  - Focused daemon tests pass:
    - `go test ./internal/daemon`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - Milestone M is complete; next work should be defined in a new milestone set or backlog extension.

## 15) Full-Review Pass + Hardening (2026-03-07)
- **What changed**:
  - Ran a full implementation review from milestones A→M and fixed remaining daemon lifecycle edge cases.
  - Daemon safety hardening:
    - Added process identity checks to avoid operating on non-FiVi reused PIDs (`ProcessLooksLikeFiViDaemon`).
    - Fixed `stop` behavior for stale/dead PID files (now removes stale PID and exits cleanly instead of signaling dead PID).
    - Added Linux cmdline-based process identity detection (`/proc/<pid>/cmdline`) with non-Linux fallback.
  - UI/log visibility improvements:
    - Added explicit warm-failover phase log line in `--no-tui` reporter.
    - Added warm-failover label in discovery TUI connection status panel.
    - Added regression-style UI test coverage for warm-failover phase logging.
  - Documentation consistency fixes:
    - Corrected README defaults to match current code (`--mode` default `proxy`, `--max-candidates` default `0`, `--startup-timeout` default `8s`).
  - Planning updates:
    - Added post-M backlog section (N1–N8) for future VPN/engine/mode/UI/reliability enhancements.
- **Files touched**:
  - `main.go`
  - `internal/daemon/daemon.go`
  - `internal/daemon/process_linux.go`
  - `internal/daemon/process_other.go`
  - `internal/ui/progress.go`
  - `internal/ui/progress_test.go`
  - `internal/ui/dashboard.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Safer daemon control: `daemon/stop/status` now avoid accidental interaction with unrelated processes.
  - Better runtime visibility during standby transitions (warm failover) in both TUI and log mode.
  - User docs now align with actual runtime defaults.
- **Validation results**:
  - Full suite and build pass:
    - `go test ./...`
    - `go build ./...`

## 16) Request Patchset Update (2026-03-07)
- **What changed**:
  - Tunnel-mode validation alignment:
    - Candidate testing now builds configs with `TunnelMode=cfg.IsTunnelMode()` instead of forcing proxy-mode build semantics.
  - Direct single-config ingestion:
    - Added `--config` (CSV) and `--config-file` (one URI per line) and merged direct configs with fetched source links before normalize/dedupe.
  - Probable classification and fallback:
    - Added latency-aware probable promotion for candidates that fail probe but have known good connect latency.
    - Added plan-B flow: if no working candidates are found, FiVi now attempts probable candidates before full rediscovery.
  - Updated docs/tests/planning for these behaviors.
- **Files touched**:
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `internal/latency/scanner.go`
  - `internal/engine/engine.go`
  - `internal/engine/benchmark_test.go`
  - `internal/engine/tunnel_validation_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Tunnel users no longer rely on proxy-only candidate qualification, reducing false negatives for tunnel-capable configs.
  - Users can combine subscriptions and raw single configs in one run.
  - Runs with zero working candidates now attempt probable configs first, improving connection odds before a full retry cycle.
- **Validation results**:
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`

## 17) Probable Threshold Tuning (2026-03-07)
- **What changed**:
  - Added `--probable-latency-threshold` flag to control latency-based promotion of probe-failed candidates into probable bucket.
  - Default threshold is `450ms`; setting to `0` disables latency-based promotion.
  - Refactored promotion decision into helper for explicit behavior and testability.
- **Files touched**:
  - `internal/config/config.go`
  - `internal/engine/engine.go`
  - `internal/engine/tunnel_validation_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Operators can tune how strict probable promotion should be for low-latency but probe-failed configs.
  - Helps reduce false optimism in noisy networks while preserving fallback flexibility.

## 18) N1 Daemon Hardening + Health Visibility (2026-03-07)
- **What changed**:
  - Added daemon lifecycle commands:
    - `./fivi restart [fivi flags...]`
    - `./fivi logs --tail N`
  - Added PID metadata support with startup readiness marker:
    - `~/.fivi/fivi.pid.json` with `state=started|ready` and timestamps.
  - Added daemon heartbeat persistence:
    - `~/.fivi/daemon.health.json` periodically updated with phase, active node/protocol, ping/probe/failures, and last successful probe timestamp.
  - Reworked daemon child runtime path to run log reporter + heartbeat/metadata fanout from status updates.
- **Files touched**:
  - `main.go`
  - `internal/daemon/daemon.go`
  - `internal/daemon/daemon_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Daemon ops are now easier (`restart`, log tailing) and safer to inspect without interactive attachment.
  - `status` now exposes daemon state (`started` vs `ready`) when metadata is available.
  - External monitoring can read `daemon.health.json` for lightweight health checks.

## 19) N2 Adaptive Probe Target Strategy (2026-03-07)
- **What changed**:
  - Added rolling per-target quality scoring from validation outcomes (working vs non-working pass rates).
  - Implemented adaptive target reordering between discovery rounds using separability score.
  - Required targets remain pinned to the front of the target list to preserve strict restricted-network guarantees.
- **Files touched**:
  - `internal/engine/engine.go`
  - `internal/engine/tunnel_validation_test.go`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Probe targets are now dynamically ordered to prioritize targets that better distinguish good vs bad candidates.
  - Required profile targets are still enforced and prioritized.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/engine`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `N3` Engine expansion feasibility (`hysteria2` adapter).

## 20) Session Distillate / Next-Session Bootstrap (2026-03-07)
- **Stable baseline now**:
  - Milestones `A–K`, `L1/L3/L4/L5`, `M1–M4`, request patchset (`R1–R4`), and backlog `N1/N2` are complete and validated.
  - `L2` (macOS sysproxy) remains intentionally skipped by product direction.
- **Last completed implementation**:
  - `N2` adaptive probe target strategy is in place (rolling target quality + adaptive ordering), with required targets pinned first.
  - Commit created for this step: `feat: adaptive probe target strategy`.
- **Behavioral invariants to preserve**:
  - Tunnel-mode candidate validation must remain tunnel-aware (no proxy-forced build path).
  - Probable fallback path should run before full rediscovery when working set is empty.
  - Daemon lifecycle safety checks (PID identity, stale PID cleanup, heartbeat/metadata) must stay intact.
  - Required profile targets must stay pinned even when adaptive target ordering runs.
- **Recommended first actions next session**:
  1. Re-run baseline verification: `go test ./... && go build ./...`.
  2. Start `N3`: evaluate/implement `hysteria2` adapter feasibility (proxy-mode first), scoped to existing adapter contracts.
  3. If `N3` lands, update `plans/TASKS.md` + append a new section in this file with files changed, behavior impact, and validation.
- **Primary files to inspect first for continuation**:
  - `internal/engines/adapter.go`
  - `internal/engines/xray.go`
  - `internal/engines/singbox.go`
  - `internal/engine/engine.go`
  - `plans/TASKS.md`

## 21) N3 Engine Expansion: Hysteria2 Adapter (2026-03-07)
- **What changed**:
  - Added new `hysteria2` engine adapter under existing adapter abstraction.
  - Implemented `hysteria2`/`hy2` URI parsing and runtime config generation for proxy mode.
  - Added process wrapper lifecycle for hysteria2 (`EnsureBinary`, `Start`, `Stop`) using client config file execution.
  - Wired engine selection and binary resolution:
    - `--engine hysteria2`
    - `--hysteria2-bin`
    - `FIVI_HYSTERIA2_BIN`
  - Added engine-alias normalization support (`hy2`, `hysteria-2` -> `hysteria2`).
- **Files touched**:
  - `internal/engines/hysteria2.go`
  - `internal/engines/hysteria2_test.go`
  - `internal/engine/engine.go`
  - `internal/engine/adapter_select_test.go`
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - FiVi now supports `hysteria2` in proxy mode through the same discovery/connect/monitor pipeline.
  - Tunnel mode for hysteria2 is intentionally not enabled yet (adapter returns explicit proxy-mode-only error).
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/engines ./internal/engine ./internal/config`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `N4` Additional mode: `urltest` continuous benchmarking mode.

## 22) N4 URLTest Continuous Benchmarking Mode (2026-03-07)
- **What changed**:
  - Added new connection mode: `--mode urltest`.
  - Implemented proactive switching logic in monitoring path while preserving existing `proxy` and `tunnel` semantics.
  - URLTest mode now:
    - tracks active candidate score stability over time,
    - triggers preemptive failover on stable score drop,
    - periodically benchmarks top alternative ranked candidates,
    - triggers preemptive failover when a better candidate consistently outperforms active.
  - Added helper benchmark path that reuses existing candidate validation pipeline (`testOne`) for alternative-candidate scoring.
  - Extended mode normalization/documentation to include `urltest`.
- **Files touched**:
  - `internal/engine/engine.go`
  - `internal/engine/urltest_test.go`
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Active node can now be switched before hard outage in URLTest mode when degradation is stable or alternatives are consistently better.
  - Existing `proxy` and `tunnel` modes remain unchanged.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/config ./internal/engine`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `N5` UI clarity improvements for long runs.

## 23) N5 UI Clarity Improvements for Long Runs (2026-03-07)
- **What changed**:
  - Added monitoring failure-reason rollup payloads to status updates (`FailureTop`) computed over a rolling recent-check window.
  - Added discovery source-health offender payloads (`SourceTop`) derived from source health stats (success/fail/yield/cooldown).
  - Updated TUI dashboard:
    - monitoring view now shows “why failed” rollup (top failure reasons over recent checks),
    - discovery view now shows compact source-health top offenders panel.
  - Updated `--no-tui` log reporter to mirror the same diagnostics in phase logs and periodic snapshots (`failTop=[...]`, `srcTop=[...]`).
- **Files touched**:
  - `internal/engine/events.go`
  - `internal/engine/engine.go`
  - `internal/engine/tunnel_validation_test.go`
  - `internal/ui/dashboard.go`
  - `internal/ui/dashboard_test.go`
  - `internal/ui/progress.go`
  - `internal/ui/progress_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - Users can diagnose recurring degradation reasons directly in monitoring UI/log mode without scanning verbose worker events.
  - Discovery runs now expose source-level offenders compactly, improving long-run debugging of cooldown/yield/failure issues.
  - Existing connection semantics remain unchanged; this is visibility/diagnostics-only behavior.
- **Validation results**:
  - Focused package tests pass:
    - `go test ./internal/engine`
    - `go test ./internal/ui`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `N6` Connection quality guardrails.

## 24) N6 Connection Quality Guardrails (2026-03-07)
- **What changed**:
  - Added optional active-session quality guardrails:
    - `--quality-latency-floor`
    - `--quality-jitter-floor`
    - `--quality-floor-streak`
  - Extended monitor loop to evaluate quality even when probes still pass, using:
    - average probe latency,
    - probe latency jitter (standard deviation across successful targets).
  - Added soft-failover path: when quality floors are breached for consecutive checks (`quality-floor-streak`), active candidate is rotated before hard outage.
  - Added helper functions for jitter calculation, guardrail breach evaluation, and streak normalization.
- **Files touched**:
  - `internal/config/config.go`
  - `internal/engine/engine.go`
  - `internal/engine/quality_guardrail_test.go`
  - `internal/engine/tunnel_validation_test.go`
  - `README.md`
  - `plans/TASKS.md`
- **Behavior impact**:
  - FiVi can now preemptively fail over on sustained poor quality (high latency/jitter) before hard monitor failures accumulate.
  - Guardrails are opt-in (`0` disables latency/jitter thresholds), so existing deployments keep prior behavior unless enabled.
- **Validation results**:
  - Focused package tests pass:
    - `go test ./internal/engine`
    - `go test ./internal/config`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task**:
  - `N7` Route/DNS runtime diagnostics (`./fivi doctor`).

## 25) N7 Route/DNS Runtime Diagnostics (2026-03-07)
- **What changed**:
  - Added new `./fivi doctor` subcommand to run pre-connect diagnostics without starting discovery/connect loops.
  - Implemented `internal/doctor` diagnostics package with structured findings (`PASS/WARN/FAIL/SKIP`) and a human-readable report formatter.
  - Added routing diagnostics to detect parse errors and dual-input precedence ambiguity (`--routing-rules` + `--routing-rules-file`).
  - Added DNS diagnostics for remote resolver validity and bootstrap loop risks, including explicit failure when DoH/DoT hostname bootstrap resolver is not an IP.
  - Added system-proxy diagnostics for command availability by OS/desktop environment (Linux GNOME/KDE/unknown, Windows PowerShell, macOS `networksetup` awareness).
- **Files touched**:
  - `main.go`
  - `internal/doctor/doctor.go`
  - `internal/doctor/doctor_test.go`
  - `README.md`
  - `plans/TASKS.md`
  - `plans/CONTEXT.md`
- **Behavior impact**:
  - Users can now run actionable preflight checks (`./fivi doctor`) before long runs to catch common DNS/routing/system-proxy assumption issues early.
  - `doctor` returns non-zero only when at least one check is `FAIL`; warnings remain non-fatal for operational flexibility.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/doctor`
  - Build pass:
    - `go build ./...`
- **Next task**:
  - `N8` Binary trust and rollout control.

## 26) N8 Binary Trust and Rollout Control (2026-03-07)
- **What changed**:
  - Added binary rollout policy support with channel pinning:
    - `stable` (non-prerelease)
    - `latest` (can include prereleases)
  - Added optional exact release tag allowlist filtering for install/update/check-update paths.
  - Extended release fetching to support GitHub release-list payloads and policy-based release selection.
  - Added rollout state persistence at `~/.fivi/bins/rollout_state.json` with per-engine metadata:
    - current version,
    - last known good version,
    - previous version backup path,
    - consecutive startup failure counters.
  - Added automatic rollback on repeated startup failures:
    - startup failures increment per-version counter,
    - once threshold is reached, FiVi restores previous binary backup and resets failure streak.
  - Added new runtime/config/install/update controls:
    - `--binary-channel`
    - `--binary-allowlist`
    - `--binary-rollback-threshold`
  - Wired startup success/failure recording into both normal and daemon run paths.
- **Files touched**:
  - `internal/binmgr/download.go`
  - `internal/binmgr/update.go`
  - `internal/binmgr/policy.go`
  - `internal/binmgr/policy_test.go`
  - `internal/binmgr/rollout.go`
  - `internal/binmgr/rollout_test.go`
  - `internal/config/config.go`
  - `internal/config/config_test.go`
  - `main.go`
  - `README.md`
  - `plans/TASKS.md`
  - `plans/CONTEXT.md`
- **Behavior impact**:
  - Binary updates can now be constrained to trusted channels/tags.
  - Repeated startup breakage after binary changes can self-heal by rolling back to previous known binaries.
  - `check-update`, `install`, and `update` now respect rollout policy inputs for safer operations.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/binmgr`
    - `go test ./internal/config`
  - Build pass:
    - `go build ./...`
- **Next task / blocker**:
  - Backlog milestone sequence is complete through `N8`; next work is a new requested patchset or future backlog expansion.

## 27) L2 macOS System Proxy + Full Audit Pass (2026-03-07)
- **What changed**:
  - Implemented macOS system proxy backend (`networksetup`) in `internal/sysproxy/darwin.go`:
    - enumerate network services,
    - apply SOCKS proxy for each service,
    - set bypass domains,
    - save/restore per-service prior SOCKS + bypass state.
  - Added Darwin parser/command tests in `internal/sysproxy/darwin_test.go`.
  - Updated sysproxy dispatcher to route `runtime.GOOS == darwin` through new backend.
  - Ran full codebase audit and fixed a rollout-accounting bug:
    - startup success is now recorded only on clean run completion (`err == nil`),
    - interrupted/canceled runs no longer incorrectly mark binaries as successful.
  - Updated README proxy-mode docs with cross-platform automation support matrix.
- **Files touched**:
  - `internal/sysproxy/darwin.go`
  - `internal/sysproxy/darwin_test.go`
  - `internal/sysproxy/gnome.go`
  - `main.go`
  - `README.md`
  - `plans/TASKS.md`
  - `plans/CONTEXT.md`
- **Behavior impact**:
  - Proxy mode now supports automatic macOS SOCKS system-proxy setup and restoration.
  - Binary rollout health signals are more accurate because canceled runs are not treated as successful startups.
  - Documentation now reflects actual platform proxy behavior.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/sysproxy`
  - Full-suite and static checks pass:
    - `go test ./...`
    - `go vet ./...`
    - `go build ./...`
- **Next task / blocker**:
  - Milestone backlog items are complete through `N8`; optional next step is broader feature iteration or performance profiling-driven enhancements.

## 28) Post-L2 Optimization Follow-up (2026-03-07)
- **What changed**:
  - Fixed diagnostics consistency bug in `doctor` for macOS:
    - `./fivi doctor` now reports `PASS` when `networksetup` is present on Darwin,
    - warning text now only applies when `networksetup` is missing.
  - Added Darwin-specific doctor tests to lock expected behavior.
  - Improved monitoring responsiveness by capping speed-sample timeout used in monitor ticks:
    - speed sampling now uses `min(--probe-timeout, 4s)`.
    - prevents long speed tests from delaying health-loop progression and failover decisions under high timeouts.
  - Added unit tests for speed-sample timeout derivation.
- **Files touched**:
  - `internal/doctor/doctor.go`
  - `internal/doctor/doctor_test.go`
  - `internal/engine/engine.go`
  - `internal/engine/monitor_timeout_test.go`
  - `plans/CONTEXT.md`
- **Behavior impact**:
  - `doctor` output now accurately matches implemented macOS proxy support.
  - Active-session monitoring remains more responsive even when users set large probe timeouts, reducing risk of delayed degradation handling.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/doctor`
    - `go test ./internal/engine`
  - Full suite/build pass:
    - `go test ./...`
    - `go build ./...`
- **Next task / blocker**:
  - No open milestone blocker in `TASKS.md`; next valuable step is profile-guided performance tuning (`pprof`) on discovery worker throughput.

## 29) URLTest Responsiveness Optimization (2026-03-07)
- **What changed**:
  - Improved URLTest monitoring loop responsiveness by bounding alternative-candidate benchmark timeout:
    - added `effectiveURLTestBenchmarkTimeout()`
    - benchmark duration now uses `min(--candidate-timeout, 6s)` with `6s` default cap.
  - This prevents prolonged benchmark calls from delaying monitor ticks and proactive switching decisions when candidate timeout is configured very high.
  - Minor cleanup in reconnect flow: removed redundant local state (`reconnected`) while preserving behavior/log output.
- **Files touched**:
  - `internal/engine/engine.go`
  - `internal/engine/monitor_timeout_test.go`
  - `plans/CONTEXT.md`
- **Behavior impact**:
  - Faster/steadier monitoring cadence in URLTest mode under large candidate-timeout settings.
  - Better failover/switch responsiveness during degradation windows.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/engine`
    - `go test ./internal/doctor`
  - Full suite/static/build pass:
    - `go test ./...`
    - `go vet ./...`
    - `go build ./...`
- **Next task / blocker**:
  - Backlog complete in `TASKS.md`; remaining improvements are continuous tuning and optional profiling instrumentation.

## 30) Extra Step X1/X2: Profiling Hooks + Task Traceability (2026-03-07)
- **What changed**:
  - Added opt-in runtime profiling support via local pprof server:
    - new `internal/profiler` package,
    - lifecycle-managed startup/shutdown with context-driven cleanup,
    - default safe bind `127.0.0.1:6060`.
  - Added runtime flags and wiring:
    - `--pprof`
    - `--pprof-addr`
    - startup integration in both normal and daemon execution paths.
  - Added profiler unit tests validating endpoint serving and address normalization.
  - Updated README with pprof usage and flags.
  - Added explicit extra-step entries in `plans/TASKS.md` and marked them complete (`X1`, `X2`) per request.
- **Files touched**:
  - `internal/profiler/profiler.go`
  - `internal/profiler/profiler_test.go`
  - `internal/config/config.go`
  - `main.go`
  - `README.md`
  - `plans/TASKS.md`
  - `plans/CONTEXT.md`
- **Behavior impact**:
  - Operators can profile runtime hotspots in production-like runs without code changes.
  - Planning artifacts now explicitly capture and tick extra post-task improvements for better traceability.
- **Validation results**:
  - Focused tests pass:
    - `go test ./internal/profiler`
  - Full suite/static/build pass:
    - `go test ./...`
    - `go vet ./...`
    - `go build ./...`
- **Next task / blocker**:
  - No mandatory backlog blocker; optional next step is adding profile snapshot export commands for non-interactive diagnostics.
