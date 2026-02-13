# fivi

A Go CLI that discovers free V2Ray share links, validates which ones truly connect (including blocked-in-Iran targets), picks the fastest stable node, then keeps monitoring and auto-fails over.

## What it does

1. Fetches subscription lists from multiple URLs in parallel.
2. Decodes subscription content (plain or base64 blob), extracts share links, and de-duplicates them.
3. Supports `vmess`, `vless`, `trojan`, `ss` links.
4. Tests candidates concurrently by launching isolated `xray` instances.
5. Probes via SOCKS against:
- `https://x.com/`
- `https://www.youtube.com/`
- `https://www.cloudflare.com/cdn-cgi/trace`
6. Keeps only nodes that can reach both `x.com` and `youtube.com`.
7. Ranks working nodes by success and latency.
8. Connects using the best node and exposes local SOCKS/HTTP proxy.
9. Health-checks on interval; on repeated failure, switches to next best.
10. If all fail, restarts discovery and testing.

## Requirements

- Go 1.22+
- `xray` binary installed and available in PATH, or pass `--xray-bin /path/to/xray`.

## Build

```bash
go mod tidy
go build -o fivi .
```

## Run

```bash
./fivi
```

Useful flags:

```bash
./fivi \
  --workers 24 \
  --max-candidates 300 \
  --monitor-interval 45s \
  --probe-timeout 7s \
  --socks-addr 127.0.0.1:10808 \
  --http-addr 127.0.0.1:10809
```

Override sources:

```bash
./fivi --sources "https://source1/sub.txt,https://source2/sub.txt"
```

## Usage after connected

Configure apps/system to use:

- SOCKS5 proxy: `127.0.0.1:10808`
- HTTP proxy: `127.0.0.1:10809`

## Notes

- Free public nodes are volatile; defaults can expire. Pass your own source list with `--sources`.
- The app prioritizes practical connectivity and failover, not perfect protocol completeness for every exotic link format.
