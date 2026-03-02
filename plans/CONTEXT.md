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
