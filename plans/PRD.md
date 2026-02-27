# FiVi Hardening PRD

## 1) Product Summary
FiVi is a Go CLI that discovers public V2Ray links, validates connectivity, ranks candidates, connects to the best node, and maintains the session with failover.

This PRD defines all phases to make FiVi a "hard-ass VPN" — always finding a way to connect the user in restricted networks (especially Iran), improving transparency during testing, supporting multiple engines and modes, and adding resilience features inspired by oblivion-desktop's architecture.

## 2) Confirmed Decisions
- Preferred second engine: `sing-box`
- Optional third engine: `clash-meta` only if low effort and low maintenance
- Interactive UX: TUI enabled by default on interactive terminals
- Default profile: `iran`
- System proxy: GNOME (gsettings) for Linux (expandable to KDE, macOS, Windows)
- Xray TUN mode: implemented with native gVisor TUN
- Max candidates: 0 (no limit) by default, random shuffle for protocol diversity

## 3) Problem Statement
Current behavior can look stalled during early test batches. Source diversity is limited, worker progress is opaque, and only one engine (`xray`) was originally supported. After baseline milestones A-G, FiVi is functional but lacks several resilience and quality-of-life features compared to mature VPN clients like oblivion-desktop.

## 4) Comparative Analysis: Oblivion-Desktop vs FiVi

### What Oblivion-Desktop Does Well (that FiVi should learn from)
1. **Endpoint Scanner**: Scans Cloudflare WARP endpoints by RTT, finds optimal ones before connecting. FiVi equivalent: scan/benchmark configs faster with parallel latency pre-checks.
2. **Connection Verification Loop**: 15-retry connection verification with 3s intervals after engine start. FiVi just does one probe check.
3. **Routing Rules System**: User-configurable rules (geoip, domain, ip, range, app) for split tunneling. FiVi has none.
4. **DNS Configuration**: Dual DNS (plain + DoH), DNS loop prevention, configurable per-mode. FiVi has hardcoded DNS.
5. **Speed Test**: Full Cloudflare speed test (download, upload, latency, jitter). FiVi only has basic download sampling.
6. **Network Statistics**: Real-time sent/recv speed, total usage monitoring. FiVi has none.
7. **PAC Script**: Auto-generates PAC scripts for browsers that don't respect system SOCKS proxy. FiVi has none.
8. **Cross-Platform Proxy**: Windows (registry), macOS (networksetup), Linux (GNOME gsettings, KDE kwriteconfig). FiVi only has GNOME.
9. **Geo-Based Routing**: Country-specific bypass rules (ir, cn, ru, etc.) using sing-box rule sets. FiVi has profile-based targets but no geo routing.
10. **Process Safety Net**: Proxy-reset on crash (Windows), restart counter cap, antivirus exclusion hints. FiVi has none.
11. **Binary Auto-Management**: Version check, copy, update binaries automatically. FiVi requires manual binary placement.
12. **Multiple Connection Methods**: WARP, Gool (WARP-in-WARP), Psiphon, MASQUE. FiVi only uses public V2Ray configs.
13. **LAN Sharing**: Bind to 0.0.0.0 to share VPN with other devices. FiVi binds to 127.0.0.1 only.
14. **Auto-Connect / Auto-Reconnect**: Start connected on launch, auto-retry on Iran IP detection. FiVi requires manual start.

### What FiVi Does Better
1. **Config Discovery**: Automated multi-source config aggregation from 30+ public sources. Oblivion relies on single Cloudflare WARP.
2. **Multi-Protocol**: Supports vmess, vless, trojan, shadowsocks. Oblivion is WARP-only.
3. **Candidate Ranking**: Weighted scoring with success ratio, latency, blocked-target tests. Oblivion just connects to WARP.
4. **Source Health Tracking**: Cooldown, backoff, yield tracking. Oblivion has no source concept.
5. **Engine Abstraction**: Clean adapter interface for xray/singbox. Oblivion hardcodes warp-plus.
6. **Worker Pipeline**: Concurrent candidate validation with event stream. Oblivion has no parallel testing.

## 5) Goals (Phase 2 — Post Milestone G)
1. Add connection resilience: smarter retry, reconnection, warmup verification.
2. Add split-tunneling / routing rules for bypass traffic.
3. Add configurable DNS (plain + DoH) with user override.
4. Add speed test and network statistics monitoring.
5. Add cross-platform system proxy support (KDE, macOS, Windows beyond GNOME).
6. Add PAC script generation for proxy mode.
7. Add latency pre-scan to fast-reject slow configs before full probe.
8. Add binary auto-download and version management.
9. Add LAN sharing mode (bind 0.0.0.0).
10. Add graceful cleanup and safety nets for crashes.
11. Add geo-based routing rules (sing-box rule sets).
12. Add auto-connect and persistent session resume.

## 6) Non-Goals
- GUI desktop/mobile app (stay CLI + TUI)
- WARP/Cloudflare integration (different approach from oblivion)
- Perfect parsing for every exotic link variant in first release
- Guarantee of continuous availability for free public nodes

## 7) Users and Use Cases
- Users in restricted networks needing fast, practical connectivity.
- Users who need confidence during long test cycles (progress, status, failures).
- Users wanting automatic failover after degradation.
- Power users who want routing rules and split tunneling.
- Users sharing VPN over LAN with other devices.
- Users who want zero-config binary management.

## 8) Functional Requirements

### FR-1 Source Aggregation (Done — Milestones A)
- [x] Include all known `v2finder` URL sources as defaults.
- [x] Add source health tracking: success rate, latency, last-good timestamp.
- [x] Apply cooldown to repeatedly failing sources.
- [x] Support mixed feed formats: line-based, base64 blob, and JSON list payloads.

### FR-2 Candidate Pipeline (Done — Milestone B)
- [x] Keep support for `vmess`, `vless`, `trojan`, `ss`.
- [x] Use bounded worker concurrency and per-candidate timeout budget.
- [x] Track per-stage status: `fetch`, `parse`, `spawn`, `probe`, `rank`.
- [x] Classify candidates into `working`, `probable`, `failed`.

### FR-3 Connectivity and Ranking (Done — Milestone E)
- [x] Default profile `iran` with blocked/unblocked target mix.
- [x] Weighted scoring based on success ratio + latency + stability.
- [x] Ranking must prefer nodes that reach critical blocked targets.

### FR-4 Session Management (Done — Milestones B, E)
- [x] Connect with best candidate.
- [x] Monitor health periodically with hysteresis.
- [x] Fail over after sustained failures.
- [x] Rediscover when candidate pool is exhausted.

### FR-5 Engine Modes (Done — Milestone D + H-TUN)
- [x] `--engine xray` (existing behavior)
- [x] `--engine singbox` (new)
- [x] Xray TUN mode support
- [ ] Optional `--engine clashmeta` deferred.

### FR-6 TUI and UX (Done — Milestone C, G)
- [x] TUI with phase-aware views (discovery + monitoring).
- [x] Plain log fallback with `--no-tui`.
- [x] Status event system for phase transitions.

### FR-7 Output Artifacts (Done — Milestones A, B)
- [x] `output/all_configs.txt`, `output/working_configs.txt`, `output/probable_configs.txt`
- [x] `output/source_report.json`

### FR-8 Latency Pre-Scan (New)
- Before full probe cycle, do a lightweight latency-only check (TCP connect or ICMP) on candidate server addresses.
- Fast-reject configs with >2s latency to avoid wasting worker time on unreachable servers.
- Configurable threshold via `--latency-cutoff`.

### FR-9 Routing Rules / Split Tunneling (New)
- Support user-defined routing rules: domain bypass, IP bypass, CIDR range bypass.
- Rule types: `domain:*.ir`, `ip:10.0.0.0/8`, `domain:!force-proxy.com` (negated = force through proxy).
- Rules apply to sing-box and xray route configurations.
- Default rule presets per profile (e.g., `iran` profile bypasses `*.ir` domains).
- Configurable via `--routing-rules` flag or rules file.

### FR-10 Configurable DNS (New)
- Allow user to specify plain DNS and DoH servers.
- Flags: `--dns`, `--doh`.
- DNS loop prevention: proxy server address resolved via direct DNS.
- Defaults: remote `tls://8.8.8.8` (through proxy), local `223.5.5.5` (direct).
- Applied to both xray and singbox generated configs.

### FR-11 Speed Test (New)
- Full speed test command: `./fivi speedtest`.
- Measures download, upload, latency, jitter through the active proxy.
- Uses Cloudflare speed.cloudflare.com or configurable target.
- Can run while connected or standalone through a specified SOCKS proxy.

### FR-12 Network Statistics (New)
- Real-time traffic monitoring while connected.
- Display in TUI: sent/recv speed, total usage.
- Optional `--stats` flag to enable.
- Read from OS network interface counters (no external binary).

### FR-13 Cross-Platform System Proxy (New)
- Expand system proxy beyond GNOME gsettings.
- Add KDE support (kwriteconfig5/6).
- Add macOS support (networksetup).
- Add Windows support (registry via PowerShell).
- Auto-detect desktop environment on Linux.
- Configurable bypass list from routing rules.

### FR-14 PAC Script Server (New)
- Generate PAC (Proxy Auto-Config) script for proxy mode.
- Serve on localhost:<port+1>.
- Handles SOCKS5 proxy routing with bypass rules.
- Useful for browsers that don't respect system SOCKS proxy natively.

### FR-15 Binary Auto-Management (New)
- Auto-download xray/singbox binaries for the current OS/arch.
- Version check and update when stale.
- Store in `~/.fivi/bins/` or configurable path.
- Command: `./fivi install` to bootstrap binaries.
- Signature/checksum verification for downloads.

### FR-16 LAN Sharing (New)
- `--share` flag binds proxy to `0.0.0.0` instead of `127.0.0.1`.
- Allows other devices on LAN to use the proxy.
- Print QR code or connection info for easy sharing.

### FR-17 Connection Resilience (New)
- Post-connect verification loop: retry connectivity check up to N times before declaring connected.
- Exponential backoff on reconnection attempts.
- Warm standby: keep top-3 configs pre-validated, switch instantly on failover.
- Restart counter cap to prevent infinite restart loops.
- Graceful cleanup on SIGINT/SIGTERM (restore proxy, teardown TUN routes, kill engine process).

### FR-18 Geo-Based Routing (New)
- Support geo-IP and geo-site rule sets for sing-box.
- Country-specific bypass: `--geo-bypass ir` routes Iranian IPs/sites directly.
- Download rule sets from public repos (e.g., Chocolate4U/Iran-sing-box-rules).
- Cache rule sets locally with refresh interval.

### FR-19 Auto-Connect and Session Persistence (New)
- `./fivi daemon` mode: run as background service, auto-reconnect on failure.
- Persist last working config to disk for instant reconnect on restart.
- `--auto-reconnect` flag for automatic retry on connection drop without full rediscovery.
- Save/load session state for resume across restarts.

## 9) Non-Functional Requirements
- Throughput: no major regression versus current worker pipeline.
- Observability: clear reasoned failures, no silent waiting.
- Reliability: cancellation-safe goroutines and process cleanup.
- Portability: Linux-first, expand to macOS and Windows.
- Safety: graceful cleanup on crash/exit, no leaked proxy settings.

## 10) Success Metrics
- Time-to-first-working-node (P50/P90)
- Tested candidates per minute
- Session uptime before failover
- Successful failover rate
- Source yield and source reliability score
- Reconnection time after failover (target: <5s with warm standby)

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
- Phase 1: source and pipeline reliability (Done — Milestones A-B)
- Phase 2: TUI and observability (Done — Milestones C, G)
- Phase 3: sing-box adapter (Done — Milestone D)
- Phase 4: V2rayN configs, TUN mode, fetch command, system proxy (Done — Milestone H)
- Phase 5: Connection resilience + latency pre-scan + graceful cleanup (Milestone I)
- Phase 6: Routing rules + configurable DNS + geo routing (Milestone J)
- Phase 7: Speed test + network stats + PAC script (Milestone K)
- Phase 8: Cross-platform proxy + LAN sharing (Milestone L)
- Phase 9: Binary auto-management + auto-connect + daemon mode (Milestone M)
