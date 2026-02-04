# Edgeprobe

Edgeprobe is a tiny, always-on network monitor built for residential internet connections. It’s a simple utility you run at the customer edge to capture outage evidence so you can present it to your ISP when you need proof. It pings targets and runs DNS queries, detects outage windows, and writes JSONL logs. When an outage starts it triggers a traceroute and records any path changes.

This README is a hands-on manual: how to run it, how to read results, and what the fields mean.

## Quick Start

1. Build the binary:

```bash
make build
```

For Raspberry Pi (32-bit):

```bash
make build-pi
```

Override ARM version if needed (e.g., Pi Zero uses `GOARM=6`):

```bash
make build-pi GOARM=6
```

For Raspberry Pi (64-bit):

```bash
make build-pi64
```

2. Create a config file (example below) and save it as `./config.toml`.

3. Run (needs root for raw ICMP):

```bash
sudo ./bin/edgeprobe -config ./config.toml
```

Logs are written as JSONL to the directory you set in `logging.dir`.

## Configuration

Default config path is `/etc/edgeprobe/config.toml`.

Example:

```toml
[logging]
dir = "/var/log/edgeprobe"
max_mb = 100
max_files = 10

[ping]
interval_ms = 1000
timeout_ms = 1000
window_secs = 60

[dns]
interval_ms = 30000
timeout_ms = 2000
queries = ["example.com", "cloudflare.com"]
resolvers = ["1.1.1.1:53", "8.8.8.8:53"]

[traceroute]
cooldown_secs = 300
max_hops = 30
timeout_ms = 2000

[[targets]]
name = "cloudflare"
host = "1.1.1.1"

[[targets]]
name = "google"
host = "8.8.8.8"
```

### Required permissions

- ICMP requires root or `CAP_NET_RAW`.
- Easiest: run with `sudo`.

## Install as a service (systemd)

Recommended for Raspberry Pi so it restarts on reboot.

### Raspberry Pi Zero (armv6)

Native build on the Pi:

```bash
sudo apt-get update
sudo apt-get install -y golang-go
make build-pi GOARM=6
sudo make install
sudo make enable
```

Cross-compile on another machine and copy to the Pi:

```bash
make build-pi GOARM=6
scp ./bin/edgeprobe pi@<pi-host>:/tmp/edgeprobe
scp ./config.example.toml pi@<pi-host>:/tmp/config.toml
scp ./scripts/edgeprobe.service pi@<pi-host>:/tmp/edgeprobe.service
ssh pi@<pi-host> "sudo install -m 755 /tmp/edgeprobe /usr/local/bin/edgeprobe"
```

Then on the Pi, install config + service and enable:

```bash
ssh pi@<pi-host> "sudo install -d /etc/edgeprobe /var/log/edgeprobe"
ssh pi@<pi-host> "sudo install -m 644 /tmp/config.toml /etc/edgeprobe/config.toml"
ssh pi@<pi-host> "sudo install -m 644 /tmp/edgeprobe.service /etc/systemd/system/edgeprobe.service"
ssh pi@<pi-host> "sudo systemctl daemon-reload && sudo systemctl enable --now edgeprobe"
```

### One-command install

```bash
sudo ./scripts/install_edgeprobe.sh
```

### Makefile install

```bash
sudo make install
sudo make enable
```

The install uses:

- Binary: `/usr/local/bin/edgeprobe`
- Config: `/etc/edgeprobe/config.toml`
- Logs: `/var/log/edgeprobe/edgeprobe.jsonl`

After updating config, restart:

```bash
sudo systemctl restart edgeprobe
```

Useful commands:

```bash
sudo make status
sudo make logs
```

To uninstall and remove config/logs:

```bash
sudo make uninstall-purge
```

## How it works (short version)

- Ping and DNS probes run continuously.
- A rolling window of ping results determines outage state.
- Only outage events are logged (no steady-state logs).
- On outage start, a traceroute is run (with per-target cooldown).

## Outage rules

Built-in thresholds (not configurable in MVP):

- Loss rate >= 5% within the window
- OR p95 RTT >= 200ms within the window
- OR 3 consecutive ping failures

Outage ends when all conditions are clear for one full window.

## Log output (JSONL)

Each log line is a JSON object with an RFC3339Nano UTC timestamp (`ts`).

File name: `edgeprobe.jsonl` (rotated by size).

### Record types

#### `degradation_start`

Fields:

- `ts`, `type`, `target`, `outage_id`
- `reason` (comma-separated: `loss_pct`, `rtt_p95_ms`, `consecutive_failures`)
- `loss_pct`, `rtt_p95_ms`, `consecutive_failures`

#### `degradation_end`

Same fields as `degradation_start`, `reason` is usually `cleared`.

#### `outage_summary`

Fields:

- `ts`, `type`, `target`, `outage_id`
- `start_ts`, `end_ts`, `duration_ms`
- `loss_pct_max`, `rtt_p95_max_ms`, `rtt_avg_max_ms`, `consecutive_failures_max`
- `ping_sent`, `ping_recv`, `dns_errors`, `traceroute_count`

#### `traceroute_result`

Fields:

- `ts`, `type`, `target`, `outage_id`
- `hops`: array of `{ttl, ip, rtt_ms}`
- `path_hash`, `err`

Missing hops have `ip = ""` and `rtt_ms = null`.

#### `path_change`

Fields:

- `ts`, `type`, `target`, `outage_id`
- `prev_path_hash`, `new_path_hash`
- `prev_hops`, `new_hops`

### What the logs mean (examples)

Count outages:

```bash
rg '"type":"outage_summary"' /var/log/edgeprobe/edgeprobe.jsonl | wc -l
```

Show outage durations:

```bash
rg '"type":"outage_summary"' /var/log/edgeprobe/edgeprobe.jsonl
```

Filter for one target:

```bash
rg '"target":"1.1.1.1"' /var/log/edgeprobe/edgeprobe.jsonl
```

## Grafana + Loki (Raspberry Pi)

This repo includes a script that generates a ready-to-run Grafana + Loki setup:

```bash
./scripts/setup_grafana_loki.sh
```

It creates a `grafana-loki/` folder with Docker Compose, Loki, Promtail, and a starter dashboard.

To run it:

```bash
cd grafana-loki
sudo docker compose up -d
```

Then open Grafana at `http://localhost:3000` (default admin/admin).

## Troubleshooting

- `icmp listen requires root`:
  - Run with `sudo` or grant `CAP_NET_RAW` to the binary.
- No logs appearing:
  - Check `logging.dir` and permissions.
  - Ensure at least one target is configured.
- Traceroute errors:
  - Ensure `traceroute` is installed and in PATH.

## Notes for beginners

- The only files you need to edit are the config and (optionally) the Grafana setup.
- Logs are append-only JSONL, so you can use standard tools like `rg`, `jq`, and `wc`.

## Glossary

- Outage: a period when one or more outage rules are triggered.
- Degradation: the start or end marker for an outage window.
- Outage window: the rolling time window used to compute loss and latency stats.
- Loss rate: percentage of pings lost in the current window.
- RTT: round-trip time for a ping in milliseconds.
- p95 RTT: the 95th percentile RTT in the current window.
- Consecutive failures: number of failed pings in a row.
- Outage ID: unique identifier for one outage (`target` + timestamp + counter).
- JSONL: one JSON object per line (append-only log format).
- Path hash: hash of traceroute hop IPs (used to detect path changes).

## License

MIT
