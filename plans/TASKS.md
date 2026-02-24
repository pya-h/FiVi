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
