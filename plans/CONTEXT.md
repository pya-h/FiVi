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
- [ ] Milestone C (`C1`..`C4`)
- [ ] Milestone D (`D1`..`D4`)
- [ ] Milestone E (`E1`..`E3`)
- [ ] Milestone F (`F1`..`F4`)

## Next Recommended Execution Start
1. Implement `C3` from `plans/TASKS.md` (Bubble Tea dashboard for interactive mode).
2. Reuse the current `WorkerEvent` stream as the single source for both TUI and log visibility.
3. After Milestone C is fully complete, continue with Milestone D (`EngineAdapter` refactor and `sing-box`).

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
