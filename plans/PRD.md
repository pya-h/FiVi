# FiVi Hardening PRD

## 1) Product Summary
FiVi is a Go CLI that discovers public V2Ray links, validates connectivity, ranks candidates, connects to the best node, and maintains the session with failover.

This PRD defines the next phase to make FiVi more robust for restricted networks (especially Iran), improve transparency during testing, and add a second engine mode.

## 2) Confirmed Decisions
- Preferred second engine: `sing-box`
- Optional third engine: `clash-meta` only if low effort and low maintenance
- Interactive UX: TUI enabled by default on interactive terminals
- Default profile: `iran`

## 3) Problem Statement
Current behavior can look stalled during early test batches. Source diversity is limited, worker progress is opaque, and only one engine (`xray`) is supported.

## 4) Goals
1. Expand source coverage using all `v2finder` sources.
2. Improve connection and probing reliability in censored networks.
3. Provide clear live worker/progress visibility via TUI.
4. Add `sing-box` engine mode with near-parity behavior to `xray` mode.
5. Make `iran` profile the default operational preset.

## 5) Non-Goals
- GUI desktop/mobile app
- Perfect parsing for every exotic link variant in first release
- Guarantee of continuous availability for free public nodes

## 6) Users and Use Cases
- Users in restricted networks needing fast, practical connectivity.
- Users who need confidence during long test cycles (progress, status, failures).
- Users wanting automatic failover after degradation.

## 7) Functional Requirements
### FR-1 Source Aggregation
- Include all known `v2finder` URL sources as defaults.
- Add source health tracking: success rate, latency, last-good timestamp.
- Apply cooldown to repeatedly failing sources.
- Support mixed feed formats: line-based, base64 blob, and JSON list payloads.

### FR-2 Candidate Pipeline
- Keep support for `vmess`, `vless`, `trojan`, `ss`.
- Use bounded worker concurrency and per-candidate timeout budget.
- Track per-stage status: `fetch`, `parse`, `spawn`, `probe`, `rank`.
- Classify candidates into `working`, `probable`, `failed`.

### FR-3 Connectivity and Ranking
- Default profile `iran` with blocked/unblocked target mix.
- Weighted scoring based on success ratio + latency + stability.
- Ranking must prefer nodes that reach critical blocked targets.

### FR-4 Session Management
- Connect with best candidate.
- Monitor health periodically with hysteresis.
- Fail over after sustained failures.
- Rediscover when candidate pool is exhausted.

### FR-5 Engine Modes
- `--engine xray` (existing behavior)
- `--engine singbox` (new)
- Optional `--engine clashmeta` if implementation effort stays small and reliability is acceptable.

### FR-6 TUI and UX
- If TTY is interactive, TUI is default.
- Provide `--no-tui` to force plain logs.
- TUI shows:
- global progress (tested/total/working/probable/failed)
- worker table (id, short URI, state, elapsed, last error)
- current active node and monitor status

### FR-7 Output Artifacts
- Persist discovery artifacts each round:
- `output/all_configs.txt`
- `output/working_configs.txt`
- `output/probable_configs.txt`
- Ensure writes are incremental and crash-safe.

## 8) Non-Functional Requirements
- Throughput: no major regression versus current worker pipeline.
- Observability: clear reasoned failures, no silent waiting.
- Reliability: cancellation-safe goroutines and process cleanup.
- Portability: Linux-first, maintain compatibility with current Go targets.

## 9) Success Metrics
- Time-to-first-working-node (P50/P90)
- Tested candidates per minute
- Session uptime before failover
- Successful failover rate
- Source yield and source reliability score

## 10) Risks
- Public sources are volatile and often stale.
- Censorship changes can quickly invalidate target assumptions.
- Multi-engine support increases maintenance burden.

## 11) Source List from v2finder (to import)
1. https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/All_Configs_Sub.txt
2. https://raw.githubusercontent.com/mfuu/v2ray/master/v2ray
3. https://raw.githubusercontent.com/Pawdroid/Free-servers/main/sub
4. https://raw.githubusercontent.com/aiboboxx/v2rayfree/main/v2
5. https://raw.githubusercontent.com/ripaojiedian/freenode/main/sub
6. https://raw.githubusercontent.com/peasoft/NoMoreWalls/master/list.txt
7. https://raw.githubusercontent.com/mahdibland/V2RayAggregator/master/sub/sub_merge.txt
8. https://raw.githubusercontent.com/Leon406/SubCrawler/main/sub/share/all3
9. https://raw.githubusercontent.com/ts-sf/fly/main/v2
10. https://raw.githubusercontent.com/freefq/free/master/v2
11. https://raw.githubusercontent.com/ssrsub/ssr/master/v2ray
12. https://raw.githubusercontent.com/Alvin9999/pac2/master/v2ray/1/config.txt
13. https://raw.githubusercontent.com/Alvin9999/pac2/master/v2ray/2/config.txt
14. https://raw.githubusercontent.com/Alvin9999/pac2/master/v2ray/3/config.txt
15. https://sub.pmsub.me/base64
16. https://raw.githubusercontent.com/tbbatbb/Proxy/master/dist/v2ray.config.txt
17. https://raw.githubusercontent.com/changfengoss/pub/main/data/2024_01_17/cvjOPc.txt
18. https://raw.githubusercontent.com/ermaozi/get_subscribe/main/subscribe/v2ray.txt
19. https://raw.githubusercontent.com/w1770946466/Auto_proxy/main/Long_term_subscription1.txt
20. https://raw.githubusercontent.com/w1770946466/Auto_proxy/main/Long_term_subscription2.txt
21. https://raw.githubusercontent.com/w1770946466/Auto_proxy/main/Long_term_subscription3.txt
22. https://raw.githubusercontent.com/pojokan-id/v2ray-free/main/v2ray
23. https://raw.githubusercontent.com/yebekhe/v2rayfree/main/v2
24. https://raw.githubusercontent.com/freev2fly/freev2fly.github.io/gh-pages/_data/freeservers.json
25. https://raw.githubusercontent.com/wzz1000/ClashR_for_Windows_Alpha/main/v2ray.config
26. https://raw.githubusercontent.com/ALX-003/V2RAY-XRAYS-2024/main/V2RAY-XVLESS-XVmess/V2RAY-VMESS-VLESS-TROJAN-2024.txt
27. https://raw.githubusercontent.com/FoxiYao/freesub/main/v2ray
28. https://raw.githubusercontent.com/roshan9/v2ray/main/v2ray
29. https://raw.githubusercontent.com/anam2812/v2ray-servers/main/v2ray-servers.txt
30. https://raw.githubusercontent.com/aliaraz/v2ray-config/main/config.txt

## 12) Rollout Plan
- Phase 1: source and pipeline reliability
- Phase 2: TUI and observability
- Phase 3: sing-box adapter
- Phase 4: optional clash-meta adapter
