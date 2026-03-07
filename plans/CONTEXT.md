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
