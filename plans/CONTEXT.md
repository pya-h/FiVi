# FiVi Session Context

## Project Snapshot
- Project: `FiVi` (Go CLI)
- Purpose: discover, validate, rank, connect, and maintain public V2Ray links with failover
- Current backend: `xray`

## Confirmed Product Direction
1. Preferred second engine: `sing-box`
2. Optional `clash-meta`: only if low effort and low maintenance
3. TUI should be default on interactive terminals
4. Default operational profile should be `iran`

## User-Reported Runtime Issue
- Original failure: xray binary not found in `PATH`
- Current status: user found local `xray` binary and app starts
- New pain point: app appears stuck during initial testing batches

## Verified xray Binary Info (this session)
- File: `./xray`
- Type: Linux ELF 64-bit, statically linked, executable
- Version command output: `Xray 25.10.15`
- Conclusion: binary is valid for FiVi on this environment

## Relevant Changes Already Landed Before This Planning Pass
- Added env override for xray binary: `FIVI_XRAY_BIN`
- Improved missing-binary startup error message
- Improved decode/parser robustness and added tests
- Added tests in:
- `internal/fetch/fetch_test.go`
- `internal/parser/candidates_test.go`
- `internal/xray/config_test.go`

## External Reference Reviewed
Repository reviewed: `https://github.com/pydea-rs/v2finder`
- Extracted and consolidated 30 source URLs (listed in `plans/PRD.md`)
- Observed value:
- broader source list
- categorized output files
- simple user-visible progress
- Observed limitation:
- simplistic connectivity checks compared to FiVi engine/probe model

## Planning Artifacts Created
- `plans/PRD.md`
- `plans/TDD.md`
- `plans/TASKS.md`
- `plans/CONTEXT.md` (this file)

## Current Completion Status
- [x] Milestone A (`A1`..`A5`)
- [x] Milestone B (`B1`..`B5`)
- [x] Milestone C (`C1`..`C4`)
- [x] Milestone D (`D1`..`D4`)
- [x] Milestone E (`E1`..`E3`)
- [x] Milestone F (`F1`..`F4`)

## Next Recommended Execution Start
1. All planned milestones are complete (`A1`..`F4`).
2. Optional next work: prune stale default sources with persistent 404s and add replacement feeds.
3. Optional next work: add periodic CI smoke profiles for both `xray` and `singbox`.

## Context Update Protocol (mandatory)
After each completed task, append a section to this file:
- Task ID and date
- Summary of changes
- Files touched
- Behavior impact
- Validation performed (`go test`, `go build`, manual)
- Next task or blocker

## Constraints and Assumptions
- Network behavior is unstable and censorship-sensitive
- Public free node quality changes rapidly
- UX transparency is critical to avoid perceived hangs
- Backward compatibility of core CLI behavior is required where practical

## Task Update: A1 (2026-02-24)
- Summary of changes:
- Added centralized source registry with 30 default sources and metadata category.
- Wired `config.DefaultSources()` to load from the registry instead of hardcoded inline list.
- Added regression tests to lock source defaults to registry output.
- Files touched:
- `internal/sources/registry.go`
- `internal/sources/registry_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- Behavior impact:
- Default `--sources` now includes the full curated 30-source list from planning scope.
- Existing flag behavior remains compatible because `--sources` still uses CSV override semantics.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `A2` (source health tracking with per-round summary output).

## Task Update: A2 (2026-02-24)
- Summary of changes:
- Added in-memory source health tracker with per-source counters: successes, failures, consecutive failures, total yield, last yield, last_ok, and last_seen.
- Wired discovery flow to observe source outcomes every round.
- Added round summary logging for source health totals and top unhealthy sources.
- Files touched:
- `internal/sources/health.go`
- `internal/sources/health_test.go`
- `internal/engine/engine.go`
- Behavior impact:
- Every discovery round now accumulates source reliability and yield history.
- Logs include health summary instead of only per-source success/failure lines.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `A3` cooldown policy and skip logic for repeatedly failing sources.

## Task Update: A3 (2026-02-24)
- Summary of changes:
- Added cooldown policy in health tracker (starts after repeated consecutive failures, increasing bounded backoff).
- Added source selection logic to skip sources currently in cooldown.
- Added safe fallback when all sources are cooling down (bypass cooldown for one round to avoid deadlock).
- Files touched:
- `internal/sources/health.go`
- `internal/sources/health_test.go`
- `internal/engine/engine.go`
- Behavior impact:
- Repeatedly failing sources are temporarily skipped, reducing wasted fetch time.
- Discovery can still proceed when all sources degrade due to bypass fallback.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `A4` JSON feed extraction support in decode pipeline with tests.

## Task Update: A4 (2026-02-24)
- Summary of changes:
- Added JSON feed extraction support in subscription decode pipeline.
- Implemented recursive JSON traversal to collect embedded V2Ray links from nested arrays/objects.
- Added robust text extraction helpers and central link-scheme detection helper.
- Files touched:
- `internal/fetch/fetch.go`
- `internal/fetch/fetch_test.go`
- Behavior impact:
- Feeds like JSON arrays/objects (including nested fields) can now contribute candidate links.
- Improves compatibility with source payloads such as raw JSON server lists.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `A5` persist source stats report (`output/source_report.json`).

## Task Update: A5 (2026-02-24)
- Summary of changes:
- Added per-round source stats report generation in JSON format.
- Implemented report write path: `output/source_report.json`.
- Added atomic write behavior via temp file + rename to avoid partial report files.
- Files touched:
- `internal/engine/engine.go`
- Behavior impact:
- Each discovery round now exports source reliability/yield telemetry for external inspection.
- Enables easier diagnosis of dead sources and cooldown trends.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Milestone A completed (`A1`..`A5`). Next task: `B1` per-candidate timeout budget.

## Task Update: B1 (2026-02-24)
- Summary of changes:
- Added configurable per-candidate timeout budget (`--candidate-timeout`, default `24s`).
- Candidate validation now runs under a bounded context so a single candidate cannot block discovery indefinitely.
- Files touched:
- `internal/config/config.go`
- `internal/engine/engine.go`
- Behavior impact:
- Candidate testing attempts are now constrained by a total budget across retries.
- Improves predictability and prevents long stalls on problematic candidates.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `B2` replace fixed sleeps with readiness checks.

## Task Update: B2 (2026-02-24)
- Summary of changes:
- Added startup readiness timeout config (`--startup-timeout`, default `4s`).
- Replaced fixed post-start sleeps with active readiness checks on local SOCKS/HTTP listener addresses.
- Added readiness tests for both success and timeout paths.
- Files touched:
- `internal/config/config.go`
- `internal/engine/engine.go`
- `internal/engine/readiness_test.go`
- Behavior impact:
- Candidate testing and active connection startup no longer rely on blind sleeps.
- Startup waits now end as soon as listeners are ready, or fail fast on readiness timeout.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `B3` structured failure taxonomy and counters.

## Task Update: B3 (2026-02-24)
- Summary of changes:
- Added structured candidate failure taxonomy (`candidate_timeout`, `engine_not_ready`, `probe_failed`, etc.).
- Refactored candidate test flow to return typed outcomes (`working`, `probable`, `failed` with reason).
- Added per-round validation summary with failure reason counters.
- Files touched:
- `internal/engine/engine.go`
- Behavior impact:
- Failed candidates now have explicit reason classes, and logs include aggregated failure breakdown.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `B4` probable bucket and output persistence.

## Task Update: B4 (2026-02-24)
- Summary of changes:
- Added `probable` candidate classification for partial probe success.
- Added bucket persistence for round artifacts:
- `output/working_configs.txt`
- `output/probable_configs.txt`
- Implemented atomic bucket writes and dedupe/sort of links before save.
- Files touched:
- `internal/engine/engine.go`
- Behavior impact:
- Discovery rounds now persist copy-ready working/probable candidate link sets.
- Partial-success candidates are no longer dropped silently.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `B5` stress-safe worker queue cancellation.

## Task Update: B5 (2026-02-24)
- Summary of changes:
- Rebuilt candidate validation worker pipeline for cancellation safety:
- workers select on `ctx.Done()`
- worker-to-outcome send is context-aware
- outcome aggregation runs concurrently until worker group close
- Added bounded outcome channel based on worker count to avoid large buffered retention.
- Files touched:
- `internal/engine/engine.go`
- Behavior impact:
- Improved shutdown behavior under cancellation and reduced risk of worker send blocking/deadlock.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Milestone B completed (`B1`..`B5`). Next task: `C1` TTY mode switch and `--no-tui`.

## Session Finalization (2026-02-24)
- Context file status: finalized for the current session state.
- Source of truth for completed work: task update entries `A1`..`B5` in this file and checkmarks in `plans/TASKS.md`.
- Verified baseline at close:
- `go test ./...` passed
- `go build ./...` passed
- Mandatory carry-forward rule remains active:
- Update this file at the end of each completed task in future sessions.

## Task Update: C1 (2026-02-24)
- Summary of changes:
- Added `internal/ui` package with terminal capability detection and mode selection (`tui` vs `log`).
- Added `--no-tui` flag to force plain log mode even in interactive terminals.
- Wired startup mode selection in `main` with explicit mode logging and mode-branch scaffolding for upcoming TUI integration.
- Added unit tests for mode selection logic.
- Files touched:
- `internal/ui/mode.go`
- `internal/ui/mode_test.go`
- `internal/config/config.go`
- `main.go`
- Behavior impact:
- Non-interactive terminals now explicitly resolve to log mode.
- Interactive terminals resolve to TUI mode unless `--no-tui` is set (current runner still uses engine log path; TUI rendering arrives in `C3`).
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `C2` worker event bus and stage event emission.

## Task Update: C2 (2026-02-24)
- Summary of changes:
- Added typed worker event model (`WorkerEvent`) with stage and status enums.
- Added `RunWithEvents` execution path so callers can subscribe to worker stage events.
- Wired candidate workers to emit stage events for `parse`, `spawn`, `probe`, and `done` with statuses (`running`, `ok`, `failed`, `probable`), attempt number, reason, and elapsed time.
- Added regression tests for non-blocking event emission and parse-failure event sequence.
- Files touched:
- `internal/engine/events.go`
- `internal/engine/events_test.go`
- `internal/engine/engine.go`
- Behavior impact:
- Candidate validation now produces a structured event stream that can drive both TUI and log presentations.
- Stage-level transitions and failures are now externally observable instead of only implicit in aggregate logs.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `C3` Bubble Tea dashboard.

## Task Update: C4 (2026-02-24)
- Summary of changes:
- Added plain-log progress reporter that consumes `WorkerEvent` stream and prints periodic one-line snapshots.
- Wired `main` runtime path to run engine with event stream (`RunWithEvents`) and launch log-mode reporter.
- Added tests for progress-state accounting and reporter output behavior.
- Files touched:
- `main.go`
- `internal/ui/progress.go`
- `internal/ui/progress_test.go`
- Behavior impact:
- In log mode (`--no-tui` or non-interactive terminals), users now get periodic progress snapshots (`processed/working/probable/failed/running`) during candidate testing.
- This reduces perceived hangs during long discovery/validation rounds.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- `go run . --help` verified `--no-tui` flag exposure.
- Next task or blocker:
- Remaining Milestone C task: `C3` Bubble Tea dashboard for interactive mode.

## Task Update: C3 (2026-02-24)
- Summary of changes:
- Implemented Bubble Tea interactive dashboard wired to `WorkerEvent` stream.
- Added live progress counters, active stage summary, worker table, and current-node display.
- Refactored mode runner so interactive terminals launch dashboard while engine runs concurrently; log mode path remains with periodic plain snapshots.
- Added dashboard unit tests and module wiring for Bubble Tea/Lipgloss dependencies.
- Files touched:
- `internal/ui/dashboard.go`
- `internal/ui/dashboard_test.go`
- `main.go`
- `go.mod`
- `go.sum`
- Behavior impact:
- Interactive mode now visibly updates worker states in real time during candidate testing.
- `--no-tui` continues to force log mode; non-interactive shells remain in log mode.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Milestone C completed (`C1`..`C4`). Next task: `D1` engine adapter refactor.

## Task Update: D1 (2026-02-24)
- Summary of changes:
- Added new `internal/engines` abstraction with `Adapter`, `Instance`, and parsed metadata types.
- Implemented `XrayAdapter` that wraps existing `internal/xray` build/start/binary checks.
- Refactored discovery/connection flow to run through adapter boundary (`RunWithAdapter`, adapter-driven candidate start/probe path) while preserving existing xray behavior.
- Updated engine event tests for adapter-aware worker execution and added adapter unit tests.
- Files touched:
- `internal/engines/adapter.go`
- `internal/engines/xray.go`
- `internal/engines/xray_test.go`
- `internal/engine/engine.go`
- `internal/engine/events_test.go`
- Behavior impact:
- Runtime execution now goes through an engine adapter interface instead of direct xray calls in core loop.
- Current default behavior remains xray-backed and backward compatible.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `D2` sing-box adapter implementation.

## Task Update: D3 (2026-02-24)
- Summary of changes:
- Added engine selection and binary configuration flags: `--engine`, `--xray-bin`, `--singbox-bin`.
- Normalized engine aliases (`sing-box` -> `singbox`) and added actionable flag validation errors for invalid engine selections / empty selected binary path.
- Refactored runtime selection path to choose adapter and binary based on configured engine.
- Added adapter selection tests and binary resolution tests in engine package.
- Files touched:
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/engine/engine.go`
- `internal/engine/adapter_select_test.go`
- `main.go`
- `plans/TASKS.md`
- Behavior impact:
- CLI can now select backend engine mode at startup.
- Invalid engine combinations fail fast with explicit, actionable error messages.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- `go run . --help` verified new flags are exposed.
- `go run . --engine nope` verified actionable validation error.
- Next task or blocker:
- `D2` implementation is in place but still needs runtime verification with an installed `sing-box` binary to satisfy task done criteria.

## Task Update: D4 (2026-02-24)
- Summary of changes:
- Evaluated optional `clash-meta` adapter feasibility against current priorities and maintenance budget.
- Decision: explicitly deferred `clash-meta` adapter for now; do not block delivery on it.
- Rationale:
- Primary immediate value is xray + sing-box parity; adding clash-meta now would increase test matrix and long-term maintenance without near-term priority gain.
- Files touched:
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- Behavior impact:
- No runtime behavior change; roadmap is clarified and milestone tracking now treats clash-meta as deferred by decision.
- Validation performed:
- N/A (planning/decision update only).
- Next task or blocker:
- Remaining Milestone D blocker is `D2` runtime verification with a real `sing-box` binary.

## Task Update: E1 (2026-02-24)
- Summary of changes:
- Added profile package with named profile settings and resolver (`iran` default, `global` alternate).
- Added `--profile` flag and profile validation with actionable errors for unknown profile names.
- Wired profile-based defaults for probe targets/timeouts/min-success/retry-backoff/monitor interval while preserving explicit CLI flag override precedence.
- Updated startup log to include selected profile.
- Added profile unit tests and config coverage for normalization/env behavior.
- Files touched:
- `internal/profile/profile.go`
- `internal/profile/profile_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `main.go`
- `plans/TASKS.md`
- Behavior impact:
- App now starts with `iran` profile by default unless overridden via `--profile`.
- Users can switch to supported profiles without changing multiple individual flags.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- `go run . --help` verified `--profile` flag exposure.
- `go run . --profile unknown` verified actionable validation error.
- Next task or blocker:
- Next task: `E2` profile-specific tuning and target strategy refinement.

## Task Update: E2 (2026-02-24)
- Summary of changes:
- Extended profile settings with explicit required-target tokens and tuned `iran` defaults for timeout/retry behavior.
- Replaced hardcoded banned-site qualification checks with profile-driven required-target matching logic.
- Added engine tests for required-target matching behavior.
- Files touched:
- `internal/profile/profile.go`
- `internal/profile/profile_test.go`
- `internal/config/config.go`
- `internal/engine/engine.go`
- `internal/engine/required_targets_test.go`
- Behavior impact:
- Probe qualification strategy is now profile-driven instead of hardcoded to specific domains.
- `iran` profile now enforces blocked-target reachability through profile config data.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `E3` profile documentation in README.

## Task Update: E3 (2026-02-24)
- Summary of changes:
- Added README profile documentation with one-flag usage examples and profile rationale.
- Updated useful flag examples to include profile selection in standard run commands.
- Files touched:
- `README.md`
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- Behavior impact:
- Users can discover and switch operational profiles directly from docs.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- `go run . --help` verified `--profile` flag exposure.
- Next task or blocker:
- Milestone E completed (`E1`..`E3`). Remaining blocker before full Milestone D close: runtime verification for `D2` with real `sing-box` binary.

## Task Update: F1 (2026-02-24)
- Summary of changes:
- Added explicit scoring function (`scoreCandidate`) and deterministic scoring invariant tests.
- Existing decode and cooldown unit coverage from earlier milestones remains intact; scoring now has dedicated coverage.
- Files touched:
- `internal/engine/engine.go`
- `internal/engine/scoring_test.go`
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- Behavior impact:
- No functional behavior change; scoring logic is now unit-tested directly for monotonic expectations.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `F2` integration tests for failover state machine.

## Task Update: F2 (2026-02-24)
- Summary of changes:
- Added failover integration test with mocked adapter/probe/readiness hooks to reproducibly validate transition from failed primary candidate to next candidate.
- Introduced test-time function indirection for probe/readiness/address-reservation calls to enable deterministic state-machine tests.
- Files touched:
- `internal/engine/engine.go`
- `internal/engine/failover_integration_test.go`
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- Behavior impact:
- No runtime behavior change; failover path now has explicit integration coverage in tests.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Next task: `F3` candidate throughput benchmark.

## Task Update: F3 (2026-02-24)
- Summary of changes:
- Added candidate-throughput benchmark for validation worker pipeline using mocked adapter/probe/readiness to measure framework overhead deterministically.
- Captured baseline benchmark numbers.
- Files touched:
- `internal/engine/benchmark_test.go`
- `internal/engine/engine.go`
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- Behavior impact:
- Adds repeatable performance measurement for candidate-validation throughput.
- Validation performed:
- `go test ./internal/engine -run ^$ -bench BenchmarkValidateCandidatesThroughput -benchmem` passed.
- Baseline report:
- `BenchmarkValidateCandidatesThroughput-8`: `627470 ns/op`, `93879 B/op`, `1024 allocs/op` (120 candidates per benchmark iteration).
- Next task or blocker:
- Next task: `F4` docs and release notes finalization.

## Task Update: F4 (2026-02-24)
- Summary of changes:
- Updated README with engine/profile examples and migration notes covering new flags and output artifacts.
- Finalized release-facing documentation updates for current milestones.
- Files touched:
- `README.md`
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- Behavior impact:
- New operational modes and migration details are documented for users upgrading from earlier CLI behavior.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- Milestone F completed (`F1`..`F4`). Remaining open implementation blocker across roadmap: `D2` runtime validation with a real `sing-box` binary.

## Task Update: D2 (2026-02-25)
- Summary of changes:
- Completed runtime verification for `sing-box` adapter with real binary (`sing-box 1.12.22`).
- Performed end-to-end smoke run with `--engine singbox` and verified discovery, candidate validation, active connect, and monitor status updates.
- First smoke run showed local port conflict on default `127.0.0.1:10808`; rerun on free ports (`127.0.0.1:19080`, `127.0.0.1:19081`) validated full flow successfully.
- Files touched:
- `plans/TASKS.md`
- `plans/CONTEXT.md`
- `output/smoke_singbox.log`
- `output/smoke_singbox_ports19080.log`
- Behavior impact:
- Milestone D is now fully complete; `--engine singbox` is runtime-verified for discover/connect/monitor in this environment.
- Validation performed:
- `go test ./...` passed.
- `go build ./...` passed.
- Manual smoke run: `go run . --engine singbox --singbox-bin ./.tools/singbox/sing-box-1.12.22-linux-amd64/sing-box --no-tui --workers 8 --max-candidates 60 --top 3 --socks-addr 127.0.0.1:19080 --http-addr 127.0.0.1:19081`.
- Observed evidence:
- discovery from 30 sources with candidate testing summary (`processed=60`, `working=7`, `probable=11`).
- active connect success: `[OK] ... socks=127.0.0.1:19080 http=127.0.0.1:19081 pid=...`.
- monitor success tick: `[OK] ... uptime=46s ... success=3/3`.
- Next task or blocker:
- Milestone D completed (`D1`..`D4`); no remaining open roadmap tasks in `plans/TASKS.md`.

## Maintenance Update: Benchmark Logging Fix (2026-02-25)
- Summary of changes:
- Added `logx.SetOutput(io.Writer)` to allow temporary output redirection.
- Muted logging inside throughput benchmark to remove log I/O noise from benchmark signal.
- Files touched:
- `internal/logx/logx.go`
- `internal/engine/benchmark_test.go`
- Behavior impact:
- No runtime behavior change for normal CLI usage.
- Benchmark output is now concise and throughput measurements are less skewed by logging overhead.
- Validation performed:
- `go test ./...` passed.
- `go test ./internal/engine -run ^$ -bench BenchmarkValidateCandidatesThroughput -benchmem` passed.
- New baseline report:
- `BenchmarkValidateCandidatesThroughput-8`: `261620 ns/op`, `93795 B/op`, `1023 allocs/op`.
- Next task or blocker:
- None.

## Maintenance Update: Listen Address Preflight (2026-02-25)
- Summary of changes:
- Added startup preflight validation for active listen addresses (`--socks-addr`, `--http-addr`) before entering discovery loop.
- Added actionable error text when configured ports are already occupied.
- Added unit tests covering available and conflict cases.
- Files touched:
- `internal/engine/engine.go`
- `internal/engine/listenaddr_test.go`
- Behavior impact:
- Prevents long rediscovery loops when local proxy ports are already in use.
- Startup now fails fast with explicit guidance to change address flags or free the conflicting port.
- Validation performed:
- `go test ./internal/engine -run TestEnsureListenAddrsAvailable -v` passed.
- `go test ./...` passed.
- `go build ./...` passed.
- Next task or blocker:
- None.
