# FiVi

A Go CLI that discovers free V2Ray/proxy share links from public sources, validates real-world connectivity (including censored targets behind restrictive firewalls), ranks nodes by speed and reliability, connects through the best one, and auto-fails over when it degrades.

---

## How It Works

```
Sources (30+ URLs)
        │
        ▼
  Fetch & Decode ──────── Cache (.cache/sources/, 6h TTL)
        │                      ▲ fallback on fetch failure
        ▼
  Parse & Deduplicate
        │
        ▼
  Parallel Validation ──── Workers (CPU×3, min 10)
  (spawn engine → probe     Each candidate gets up to
   targets via SOCKS)       3 attempts within 24s budget
        │
        ▼
  Rank by success + latency
        │
        ▼
  Connect best node ──────── Proxy mode: SOCKS5 + HTTP
        │                    Tunnel mode: TUN (all traffic)
        ▼
  Health Monitor ─────────── Failover on repeated failure
        │                    Speed sampling every 3rd check
        ▼
  All fail → restart discovery
```

1. **Fetches** subscription lists from 30+ public URLs in parallel, with source-level caching and cooldown tracking.
2. **Decodes** subscription content — plain text, base64, or JSON arrays — extracts share links, and deduplicates them.
3. **Parses** `vmess://`, `vless://`, `trojan://`, and `ss://` URIs into engine-native configs.
4. **Tests** candidates concurrently by spawning isolated engine instances (xray or sing-box), probing real targets through SOCKS5.
5. **Ranks** working nodes by probe success count, required-target reachability, and average latency.
6. **Connects** the best node, exposing local SOCKS5/HTTP proxy or full system TUN tunnel.
7. **Monitors** the active connection with periodic health checks and speed sampling; fails over to the next ranked node on degradation.
8. **Restarts** full discovery if all ranked nodes are exhausted.

---

## Requirements

- **Go** 1.22+ (for building from source)
- **Engine binary** — at least one of:
  - [Xray-core](https://github.com/XTLS/Xray-core) — supports proxy mode only
  - [sing-box](https://github.com/SagerNet/sing-box) — supports both proxy and tunnel (TUN) modes
- **Root/sudo** — required only for tunnel mode (TUN device creation)

### Engine Binary Resolution

FiVi resolves engine binaries in this order:

1. CLI flag (`--xray-bin` / `--singbox-bin`)
2. Environment variable (`FIVI_XRAY_BIN` / `FIVI_SINGBOX_BIN`)
3. Auto-detect from `bins/{engine}/{GOARCH}/` relative to CWD or executable directory
4. Bare binary name via system `PATH`

Architecture is auto-detected via `runtime.GOARCH` (supports `amd64`, `arm64`, others passed through).

**Directory layout for auto-detection:**
```
bins/
├── xray/
│   ├── amd64/
│   │   └── xray
│   └── arm64/
│       └── xray
└── singbox/
    ├── amd64/
    │   └── sing-box
    └── arm64/
        └── sing-box
```

---

## Build

```bash
go mod tidy
go build -o fivi .
```

Cross-compile examples:

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o fivi-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o fivi-linux-arm64 .
```

---

## Quick Start

```bash
# Default: iran profile, xray engine, tunnel mode, interactive TUI
./fivi

# Global profile with sing-box in proxy mode, no TUI
./fivi --profile global --engine singbox --mode proxy --no-tui

# Custom binary paths
FIVI_XRAY_BIN=/usr/local/bin/xray ./fivi

# Or via flags
./fivi --engine singbox --singbox-bin /opt/sing-box/sing-box
```

Once connected, FiVi exposes:
- **SOCKS5** at `127.0.0.1:10808` (default)
- **HTTP** at `127.0.0.1:10809` (default)
- **TUN** device routing all system traffic (tunnel mode with sing-box only)

---

## Profiles

Profiles are preset configurations tuned for specific network conditions.

### `iran` (default)

Designed for restricted networks where popular sites are blocked at the national firewall level.

| Setting | Value |
|---|---|
| Probe targets | `https://x.com/`, `https://www.youtube.com/`, `https://www.cloudflare.com/cdn-cgi/trace` |
| Required targets | Must reach both `x.com` **and** `youtube.com` |
| Probe timeout | 8s |
| Min success | 2 of 3 targets |
| Monitor interval | 45s |
| Retry backoff | 14s |

```bash
./fivi --profile iran
```

### `global`

Lighter defaults for unrestricted networks — useful for finding fast free proxy nodes without blocked-site validation.

| Setting | Value |
|---|---|
| Probe targets | `https://www.google.com/generate_204`, `https://www.cloudflare.com/cdn-cgi/trace` |
| Required targets | None |
| Probe timeout | 5s |
| Min success | 1 of 2 targets |
| Monitor interval | 30s |
| Retry backoff | 8s |

```bash
./fivi --profile global
```

---

## Engines

### Xray

Default engine. Uses [Xray-core](https://github.com/XTLS/Xray-core) to spawn proxy instances.

- **Modes**: Proxy only (no TUN support)
- **Protocols**: vmess, vless, trojan, shadowsocks
- **Transports**: TCP, WebSocket, gRPC, HTTPUpgrade
- **Security**: TLS, REALITY

```bash
./fivi --engine xray --xray-bin /path/to/xray
```

### Sing-box

Uses [sing-box](https://github.com/SagerNet/sing-box) as the backend.

- **Modes**: Proxy **and** Tunnel (TUN)
- **Protocols**: vmess, vless, trojan, shadowsocks
- **Transports**: TCP, WebSocket, gRPC, HTTPUpgrade
- **Security**: TLS, REALITY
- **TUN config** (tunnel mode): `172.19.0.1/30` + `fdfe:dcba:9876::1/126`, auto-route, strict-route, mixed stack

```bash
# Proxy mode
./fivi --engine singbox --mode proxy

# Tunnel mode — routes ALL system traffic through the proxy (requires root)
sudo ./fivi --engine singbox --mode tunnel
```

---

## Modes

### Proxy Mode (`--mode proxy`)

Exposes local SOCKS5 and HTTP proxy listeners. Applications must be individually configured to use these proxies.

```bash
./fivi --mode proxy --socks-addr 127.0.0.1:10808 --http-addr 127.0.0.1:10809
```

**Using the proxy after connection:**
```bash
# curl through SOCKS5
curl -x socks5h://127.0.0.1:10808 https://check.torproject.org/

# curl through HTTP proxy
curl -x http://127.0.0.1:10809 https://ifconfig.me

# Set environment variables for CLI tools
export http_proxy=http://127.0.0.1:10809
export https_proxy=http://127.0.0.1:10809
```

### Tunnel Mode (`--mode tunnel`)

Creates a TUN device that captures all system traffic — no per-app configuration needed. **Requires sing-box engine and root privileges.**

```bash
sudo ./fivi --engine singbox --mode tunnel
```

> **Note:** Tunnel mode with `--engine xray` is not supported. Xray does not implement TUN. FiVi will fail at startup if you combine `--engine xray --mode tunnel`.

---

## UI Modes

### TUI Mode (default)

Interactive terminal dashboard powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Displays real-time:
- Worker activity by ID (candidate, stage, status, attempt, reason, elapsed time)
- Progress counters: processed / working / probable / failed
- Active stage breakdown (parse / spawn / probe)
- Currently connected node
- Last event timestamp

**Controls:** `q` or `Ctrl+C` to quit.

### Log Mode (`--no-tui`)

Plain-text progress snapshots printed every 2 seconds. Useful for non-interactive terminals, piping, or headless servers.

```bash
./fivi --no-tui
```

Output format:
```
2025-01-15 14:30:22 [INFO] processed=142 working=8 probable=3 failed=131 running=10 stages=probe=6,spawn=3,parse=1
```

FiVi auto-detects non-interactive terminals and falls back to log mode.

---

## Supported Protocols

### VMess (`vmess://...`)

Base64-encoded JSON payload.

```
vmess://eyJ2IjoiMiIsInBzIjoibm9kZS1uYW1lIiwiYWRkIjoiMS4yLjMuNCIsInBvcnQiOiI0NDMiLC4uLn0=
```

JSON fields: `v`, `ps` (name), `add` (address), `port`, `id` (UUID), `aid` (alterID), `scy` (security, default: `auto`), `net` (network, default: `tcp`), `type`, `tls`, `sni`, `alpn`, `fp` (fingerprint), `host`, `path`.

### VLESS (`vless://...`)

URL format with query parameters.

```
vless://uuid@host:port?type=ws&security=tls&sni=example.com&host=example.com&path=/ws#NodeName
```

Query parameters: `type` (default: `tcp`), `security`, `encryption` (default: `none`), `flow`, `sni`, `host`, `path`, `alpn`, `fp`, `pbk` (REALITY public key), `sid` (REALITY short ID), `spx` (REALITY spider X).

### Trojan (`trojan://...`)

URL format with query parameters.

```
trojan://password@host:port?type=ws&security=tls&sni=example.com&path=/trojan#NodeName
```

Query parameters: `type` (default: `tcp`), `security` (default: `tls`), `sni`, `host`, `path`, `alpn`, `fp`, `pbk`, `sid`, `spx`.

### Shadowsocks (`ss://...`)

Base64-encoded `method:password` payload.

```
ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@host:port#NodeName
```

---

## Source Management

### Default Sources

FiVi ships with 30+ built-in public subscription URLs (GitHub raw content from popular free-node repositories). Override them entirely:

```bash
./fivi --sources "https://example.com/sub1.txt,https://example.com/sub2.txt"
```

### Source Caching

Fetched source content is cached to `.cache/sources/` with a 6-hour TTL.

- Cache keys are SHA256 hashes of the source URL
- On fetch failure, FiVi falls back to stale cache — even if TTL expired
- Cache is stored as JSON: `{source, links[], fetched_at}`

### Source Health Tracking

Each source is tracked independently:

- **Cooldown** activates after 3 consecutive failures
- **Cooldown duration**: `2min × min(consecutive_failures, 5)` — max 10 minutes
- **Reset**: Cooldown and failure counter reset on any successful fetch
- **Bypass**: If all sources are in cooldown simultaneously, FiVi bypasses cooldowns and retries all

---

## Discovery & Ranking

### Candidate Testing Pipeline

Each candidate goes through up to 3 attempts within a per-candidate timeout budget (default 24s):

1. **Parse** — decode URI, extract protocol fields, build engine config
2. **Spawn** — start an isolated engine instance with reserved local ports
3. **Probe** — HTTP GET through SOCKS5 to each target URL, measure latency
4. **Outcome**:
   - **Working** — meets minimum success threshold + all required targets reached
   - **Probable** — partial success (some probes passed, or missing required targets)
   - **Failed** — below minimum success threshold

### Ranking Algorithm

Candidates are sorted by (in priority order):

1. **Required-target reachability** — candidates reaching required targets (e.g., `x.com` + `youtube.com`) rank first
2. **Probe success count** — more passed targets is better
3. **Average latency** — lower is better

Score formula: `score = (successes × 1000) - latencyMs`

### Output Files

After each discovery round, FiVi writes:

| File | Content |
|---|---|
| `output/all_configs.txt` | All fetched candidate URIs |
| `output/working_configs.txt` | Fully qualified candidates |
| `output/probable_configs.txt` | Partial-success candidates |
| `output/source_report.json` | Source health statistics (successes, failures, yield, cooldown status) |

---

## Connection Monitoring

Once connected, FiVi continuously monitors the active node:

- **Health checks** run every `--monitor-interval` (default: 45s iran, 30s global)
- **Speed sampling** occurs every 3rd health check — downloads up to `--speed-bytes` from `--speed-target` through SOCKS and measures throughput in Mbps
- **Failover** triggers after `--monitor-fail-limit` consecutive failures (default: 2)
- **Failover order**: next ranked candidate → next → ... → restart full discovery

**Status line format:**
```
[OK] node-name | uptime: 5m32s | ping: 245ms | speed: 2.4 Mbps | probes: 3/3 | socks: 127.0.0.1:10808 | http: 127.0.0.1:10809 | pid: 12345
```

---

## All CLI Flags

### Engine & Profile

```bash
# Select engine backend
--engine xray          # Default. Uses Xray-core
--engine singbox       # Uses sing-box

# Select operational profile
--profile iran         # Default. Tuned for restricted networks
--profile global       # Lighter defaults for unrestricted networks

# Connection mode
--mode proxy           # SOCKS5 + HTTP listeners only
--mode tunnel          # TUN device (all traffic, singbox only, requires root)
```

### Binary Paths

```bash
# Override engine binary locations
--xray-bin /path/to/xray
--singbox-bin /path/to/sing-box

# Or via environment variables
FIVI_XRAY_BIN=/path/to/xray
FIVI_SINGBOX_BIN=/path/to/sing-box
```

### Network Addresses

```bash
# Local proxy listen addresses
--socks-addr 127.0.0.1:10808    # Default SOCKS5 address
--http-addr  127.0.0.1:10809    # Default HTTP proxy address

# Example: listen on all interfaces
--socks-addr 0.0.0.0:1080 --http-addr 0.0.0.0:8080
```

### Performance Tuning

```bash
# Worker concurrency (default: CPU cores × 3, minimum 10)
--workers 24

# Maximum unique candidates to test per round (default: 350)
--max-candidates 500

# Number of top-ranked results to display (default: 5)
--top 10
```

### Timeouts

```bash
# Total budget per candidate across all retry attempts (default: 24s)
--candidate-timeout 30s

# Max wait for engine listeners to become ready (default: 4s)
--startup-timeout 5s

# Timeout per source URL fetch (default: 25s)
--fetch-timeout 30s

# Timeout per individual probe request (default: 8s iran, 5s global)
--probe-timeout 10s
```

### Probe & Monitoring

```bash
# Minimum probe successes to qualify as "working" (default: 2 iran, 1 global)
--min-success 2

# Health check interval (default: 45s iran, 30s global)
--monitor-interval 60s

# Consecutive health check failures before failover (default: 2)
--monitor-fail-limit 3

# Custom probe targets
--targets "https://x.com/,https://www.youtube.com/,https://www.cloudflare.com/cdn-cgi/trace"
```

### Speed Sampling

```bash
# URL used for speed measurement through the proxy (default: https://www.youtube.com/)
--speed-target "https://www.youtube.com/"

# Max bytes to download for speed sampling (default: 524288 = 512KB)
--speed-bytes 1048576
```

### Sources & Filtering

```bash
# Override default subscription sources (comma-separated)
--sources "https://example.com/sub1.txt,https://example.com/sub2.txt"

# Filter which protocol schemes to accept (default: vmess,vless,trojan,ss)
--schemes "vless,trojan"
```

### UI

```bash
# Disable interactive TUI, use plain log output
--no-tui
```

### Discovery Retry

```bash
# Backoff duration before restarting discovery after all candidates fail
# (default: 14s iran, 8s global)
--retry-backoff 20s
```

---

## Examples

### Basic: Find and connect to the fastest free node (Iran)

```bash
./fivi
```
Uses default iran profile, xray engine, tests 350 candidates with 10+ workers, connects the best node on `127.0.0.1:10808` (SOCKS5) and `127.0.0.1:10809` (HTTP).

### Route all system traffic through a tunnel

```bash
sudo ./fivi --engine singbox --mode tunnel
```
Creates a TUN device that routes all system traffic — no per-app proxy setup needed.

### Fast scan with many workers, global profile

```bash
./fivi --profile global --workers 30 --max-candidates 500 --top 10 --no-tui
```

### Use custom sources and only accept VLESS/Trojan

```bash
./fivi --sources "https://my-server.com/nodes.txt" --schemes "vless,trojan"
```

### Strict timeouts for slow networks

```bash
./fivi --probe-timeout 12s --candidate-timeout 35s --startup-timeout 6s --fetch-timeout 40s
```

### Listen on custom ports and all interfaces

```bash
./fivi --socks-addr 0.0.0.0:1080 --http-addr 0.0.0.0:8080
```

### Headless server with explicit binary paths

```bash
FIVI_XRAY_BIN=/opt/xray/xray ./fivi --no-tui --profile global --monitor-interval 30s
```

---

## Project Structure

```
FiVi/
├── main.go                        # Entry point, mode detection, run loop
├── internal/
│   ├── config/config.go           # CLI flag parsing, AppConfig struct
│   ├── engine/
│   │   ├── engine.go              # Discovery, ranking, connection, monitoring loop
│   │   └── events.go              # Worker event types and failure reasons
│   ├── engines/
│   │   ├── adapter.go             # Engine adapter interface
│   │   ├── xray.go                # Xray adapter (config gen, process mgmt)
│   │   └── singbox.go             # Sing-box adapter (config gen, TUN, process mgmt)
│   ├── xray/
│   │   ├── config.go              # URI parsing (vmess/vless/trojan/ss → config)
│   │   └── process.go             # Xray process lifecycle management
│   ├── fetch/
│   │   ├── fetch.go               # HTTP fetch, decoding (plain/base64/JSON)
│   │   └── cache.go               # Source caching with TTL and stale fallback
│   ├── parser/candidates.go       # Candidate normalization & deduplication
│   ├── probe/probe.go             # Target probing via SOCKS5
│   ├── sources/
│   │   ├── registry.go            # Default source URL registry
│   │   └── health.go              # Source health tracking & cooldown
│   ├── ui/
│   │   ├── mode.go                # TUI vs Log mode auto-detection
│   │   ├── dashboard.go           # Bubble Tea interactive dashboard
│   │   └── progress.go            # Log mode progress reporter
│   ├── profile/profile.go         # Profile presets (iran, global)
│   ├── binresolver/resolver.go    # Binary auto-detection by architecture
│   └── logx/logx.go              # Synchronized logging
├── bins/                          # Engine binaries (per arch)
├── output/                        # Generated output files
├── .cache/sources/                # Source fetch cache
├── go.mod
└── go.sum
```

---

## Parameter Reference Table

| Flag | Type | Default | Description |
|---|---|---|---|
| `--profile` | `string` | `iran` | Operational profile (`iran` or `global`) |
| `--engine` | `string` | `xray` | Engine backend (`xray` or `singbox`) |
| `--mode` | `string` | `tunnel` | Connection mode (`proxy` or `tunnel`) |
| `--xray-bin` | `string` | auto-resolved | Path to xray binary |
| `--singbox-bin` | `string` | auto-resolved | Path to sing-box binary |
| `--socks-addr` | `string` | `127.0.0.1:10808` | SOCKS5 listen address |
| `--http-addr` | `string` | `127.0.0.1:10809` | HTTP proxy listen address |
| `--no-tui` | `bool` | `false` | Disable interactive TUI, use plain log output |
| `--workers` | `int` | `CPU×3` (min 10) | Parallel candidate testing workers |
| `--max-candidates` | `int` | `350` | Max unique candidates to test per round |
| `--top` | `int` | `5` | Number of top-ranked results to display |
| `--candidate-timeout` | `duration` | `24s` | Total timeout budget per candidate (all attempts) |
| `--startup-timeout` | `duration` | `4s` | Max wait for engine listeners to become ready |
| `--fetch-timeout` | `duration` | `25s` | Timeout per source URL fetch |
| `--probe-timeout` | `duration` | `8s` (iran) / `5s` (global) | Timeout per individual probe request |
| `--min-success` | `int` | `2` (iran) / `1` (global) | Minimum successful probes to qualify a node |
| `--monitor-interval` | `duration` | `45s` (iran) / `30s` (global) | Health check interval for active connection |
| `--monitor-fail-limit` | `int` | `2` | Consecutive monitor failures before failover |
| `--sources` | `CSV` | 30+ built-in URLs | Comma-separated subscription source URLs |
| `--targets` | `CSV` | Profile-dependent | Comma-separated probe target URLs |
| `--speed-target` | `string` | `https://www.youtube.com/` | URL for speed measurement through proxy |
| `--speed-bytes` | `int64` | `524288` (512KB) | Max bytes to download for speed sampling |
| `--retry-backoff` | `duration` | `14s` (iran) / `8s` (global) | Backoff before restarting discovery |
| `--schemes` | `CSV` | `vmess,vless,trojan,ss` | Allowed URI schemes to accept |

**Environment variables:**

| Variable | Equivalent Flag | Description |
|---|---|---|
| `FIVI_XRAY_BIN` | `--xray-bin` | Override xray binary path |
| `FIVI_SINGBOX_BIN` | `--singbox-bin` | Override sing-box binary path |

---

## Notes

- Free public nodes are volatile and can expire at any time. Pass your own source list with `--sources` for reliability.
- The `iran` profile enforces that nodes must reach both `x.com` and `youtube.com` — nodes that bypass only some censored sites are ranked lower or excluded.
- Tunnel mode (`--mode tunnel`) requires root privileges and the `singbox` engine. Xray does not support TUN.
- Source caching (`.cache/sources/`) persists across runs. Delete the directory to force fresh fetches.
- Output files in `output/` are overwritten each discovery round.
