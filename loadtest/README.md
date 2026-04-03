# SynapseOMS Load Tests

Load testing harness using [k6](https://k6.io/) to validate performance targets.

## Prerequisites

Install k6:

```bash
# macOS
brew install k6

# Windows (Chocolatey)
choco install k6

# Linux (Debian/Ubuntu)
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
  --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D68
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6

# Docker
docker pull grafana/k6
```

## Running Tests

Start the system first:

```bash
docker compose -f deploy/docker-compose.yml up -d
```

### Order Flow Load Test

Sustained order submission at 5,000 orders/sec for 5 minutes:

```bash
k6 run loadtest/k6/order_flow.js
```

### WebSocket Streaming Load Test

1,000 concurrent WebSocket clients across three streams:

```bash
k6 run loadtest/k6/ws_stream.js
```

### Docker (no local install)

```bash
docker run --rm -i --network host grafana/k6 run - < loadtest/k6/order_flow.js
docker run --rm -i --network host grafana/k6 run - < loadtest/k6/ws_stream.js
```

## Interpreting Results

k6 outputs a summary table after each run. Key metrics to check:

| Check | Metric | Pass Criteria |
|-------|--------|---------------|
| Order latency p99 | `order_submit_latency` | < 50ms |
| Fill rate | `fill_received` | > 99% |
| WS fan-out latency p99 | `ws_message_latency` | < 5ms |
| WS message delivery | `ws_message_received` | > 95% |

Example output:

```
order_submit_latency.........: avg=12ms  p(95)=28ms  p(99)=42ms
fill_received................: 99.7%
```

If thresholds fail, k6 exits with a non-zero code and prints which thresholds were breached.

## Performance Targets

| Metric | Target |
|--------|--------|
| Order submission throughput | 5,000 orders/sec sustained |
| Order submission p99 latency | < 50ms |
| Pre-trade risk check p99 | < 10ms |
| Fill-to-WebSocket p99 | < 20ms |
| VaR computation (100 instruments) | < 500ms |
| Monte Carlo VaR (10k paths, 50 instruments) | < 2s |
| Portfolio optimization (50 instruments) | < 1s |
| WebSocket broadcast (1000 clients) | < 5ms fan-out |

## Customizing

Override rate and duration via environment variables:

```bash
k6 run -e RATE=1000 -e DURATION=1m loadtest/k6/order_flow.js
```

To send results to Grafana Cloud or InfluxDB:

```bash
k6 run --out influxdb=http://localhost:8086/k6 loadtest/k6/order_flow.js
```
