# FiVi Technical Design Document

## 1) Scope
Implement PRD requirements with focus on reliability, observability, and engine abstraction while preserving existing CLI usability.

## 2) Current Baseline
- Go CLI with modules:
- `fetch` for source pull and decode
- `parser` for normalization
- `xray` for runtime config/process
- `probe` for SOCKS tests
- `engine` for ranking, connect, monitor
- Already fixed recently:
- `FIVI_XRAY_BIN` env override
- clearer xray-not-found errors
- decode and parser regressions covered by unit tests

## 3) Target Architecture
### 3.1 Modules
1. `internal/sources`
- default source registry
- source health metadata
- cooldown policy

2. `internal/discovery`
- feed ingestion and decode pipeline
- JSON feed extraction support
- dedupe and canonicalization

3. `internal/runner`
- worker pool orchestration
- stage state machine
- event emission for TUI/logs

4. `internal/engines`
- engine abstraction interface
- `xray` adapter
- `singbox` adapter
- optional `clashmeta` adapter

5. `internal/ui`
- terminal capability detection
- bubbletea TUI
- plain log fallback

6. `internal/monitor`
- active session checks
- failover/hysteresis
- profile-specific probe strategy

### 3.2 Core Interfaces
```go
type EngineAdapter interface {
    Name() string
    EnsureBinary(path string) error
    BuildRuntimeConfig(rawURI, socksAddr, httpAddr string) ([]byte, Parsed, error)
    Start(ctx context.Context, bin string, cfgJSON []byte, name string) (Instance, error)
}

type WorkerEvent struct {
    WorkerID   int
    Candidate  string
    Stage      string // parse|spawn|probe|rank|done
    Status     string // running|ok|failed|skipped
    Err        string
    ElapsedMs  int64
    Timestamp  time.Time
}
```

## 4) Data Flow
1. Load profile (`iran` default) and engine mode.
2. Load source registry and health cache.
3. Fetch feeds concurrently with bounded parallelism.
4. Decode feeds (text, base64, json list/object extraction).
5. Normalize/dedupe candidates.
6. Worker pool validates candidates and emits `WorkerEvent`.
7. Rank candidates by weighted score.
8. Connect best candidate through selected engine.
9. Monitor; failover or rediscover as required.

## 5) Key Design Decisions
### 5.1 Profile System
- `iran` default profile values:
- stricter blocked-target requirements
- slightly longer timeouts
- more resilient retry/backoff

### 5.2 Worker and Timeout Model
- `global_round_deadline`
- `per_candidate_budget`
- `per_probe_timeout`
- cancellation propagation from root context

### 5.3 Scoring Formula
- `score = w1*success_ratio + w2*blocked_target_pass + w3*stability - w4*latency`
- profile-specific weights

### 5.4 TUI Behavior
- Default when `isatty(1)==true` and `--no-tui` is not set.
- fallback to log mode for non-interactive environments.

### 5.5 Output Persistence
- atomic writes using temp file + rename for summary files.
- append-safe real-time checkpoints for in-progress results.

## 6) Engine Support Plan
### 6.1 xray
- keep existing implementation behind `EngineAdapter`.

### 6.2 sing-box
- config builder for vmess/vless/trojan/ss outbounds.
- run process similar to xray with dedicated temp config and log.

### 6.3 clash-meta (optional)
- only if adapter can be implemented with low complexity.
- if not, keep roadmap item and do not block sing-box delivery.

## 7) Failure Taxonomy
- `source_fetch_error`
- `source_decode_error`
- `candidate_parse_error`
- `engine_start_error`
- `probe_timeout`
- `probe_blocked_target_fail`
- `monitor_health_fail`

## 8) Observability
- structured logs for each failure class.
- worker events channel to UI/log reporters.
- round summary including:
- source success ratio
- candidate throughput
- failover count

## 9) Testing Strategy
1. Unit tests
- source decode and normalization
- profile selection defaults
- scoring function invariants
- adapter config generation

2. Integration tests
- mocked engine + mocked probe for failover state machine
- event stream ordering and completeness

3. Smoke tests
- `go test ./...`
- `go build ./...`
- manual run in `--no-tui` and TUI modes

## 10) Migration and Compatibility
- keep existing flags working.
- add new flags in backward-compatible way:
- `--engine`
- `--profile`
- `--no-tui`

## 11) Security and Safety
- do not execute arbitrary remote scripts.
- sanitize and bound feed payload sizes.
- isolate runtime config artifacts in temp dirs with restricted perms.

## 12) Open Items
- Decide whether to include clash-meta in initial milestone after sing-box baseline lands.
