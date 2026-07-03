# UniFi Collector

An enterprise-grade, **vendor-extensible** monitoring collector written in Go.
It connects to a UniFi controller, collects device/client/health statistics and
events, exposes them as Prometheus metrics, and ships events to Loki — ready to
visualize in Grafana.

The architecture is modular from day one: UniFi is just the first *vendor
adapter*. QNAP, Hikvision, FortiGate, Cisco, MikroTik, SNMP or Syslog can be
added without touching the core.

## Features

- UniFi login with session-cookie management and **automatic re-authentication**
- Auto-detects classic controllers vs UniFi OS (UDM/Cloud Key)
- Thread-safe HTTP client with retries and timeouts
- Periodic collection via a per-collector scheduler (independent intervals)
- Prometheus metrics on `/metrics` (stale-series safe)
- Loki push exporter with batching, label strategy and graceful flush
- Liveness (`/healthz`) and readiness (`/readyz`) probes
- Structured logging (Zap), YAML config with env overrides (Viper)
- Graceful shutdown, no global state, dependency injection throughout
- Docker + docker-compose stack (collector, Loki, Prometheus, Grafana)

## Architecture

```
              UniFi Controller
                     │ REST API / Events
                     ▼
  ┌───────────────────────────────────────────────┐
  │ internal/unifi  (vendor adapter)               │
  │   implements collector.Source (neutral models) │
  └───────────────────────────────────────────────┘
                     │
       ┌─────────────┴──────────────┐
       ▼                            ▼
  Collectors  ──scheduler──▶  metrics (Prometheus)  ──▶ Prometheus ─┐
  (device/client/                                                    ├─▶ Grafana
   event/health)  ────────▶  loki (push)            ──▶ Loki ────────┘
```

The **core** (`config`, `logger`, `models`, `collector`, `scheduler`, `metrics`,
`loki`, `server`) is vendor-neutral. Vendor-specific code lives **only** in
`internal/unifi`.

### Layout

```
cmd/collector/       # composition root (main)
internal/
  config/            # Viper YAML + env, validation
  logger/            # Zap logger factory
  models/            # vendor-neutral domain types
  collector/         # Collector, Source & Sink interfaces + Registry + collectors
  unifi/             # UniFi vendor adapter (HTTP client + mappers)
  metrics/           # Prometheus exporter (sinks + /metrics handler)
  loki/              # Loki push exporter (LogSink)
  scheduler/         # per-collector ticker scheduler
  server/            # HTTP server: /metrics /healthz /readyz
configs/             # config.yaml, prometheus.yml, loki, grafana provisioning
```

## Configuration

See [`configs/config.yaml`](configs/config.yaml). Any value can be overridden by
an env var of the form `COLLECTOR_<SECTION>_<KEY>`, e.g.:

```bash
export COLLECTOR_UNIFI_PASSWORD='secret'
export COLLECTOR_LOKI_URL='http://loki:3100/loki/api/v1/push'
```

Key sections: `server`, `logging`, `unifi`, `loki`, `collectors` (per-collector
`enabled` + `interval`).

## Running

### Locally

```bash
make run                      # uses configs/config.yaml
# or
go run ./cmd/collector --config configs/config.yaml
```

Then: `curl localhost:8080/metrics`, `/healthz`, `/readyz`.

### Full stack with Docker

```bash
# edit configs/config.yaml: set unifi.url and unifi.username
UNIFI_PASSWORD='your-password' docker compose up -d --build
```

- Grafana:    http://localhost:3000  (admin / admin)
- Prometheus: http://localhost:9090
- Collector:  http://localhost:8080/metrics

Grafana comes pre-provisioned with Prometheus and Loki datasources.

## Metrics

| Metric | Description |
|--------|-------------|
| `unifi_devices_total{vendor,site,type}` | device count |
| `unifi_clients_total{vendor,site}` | connected client count |
| `unifi_device_up` | 1 if online |
| `unifi_device_cpu_percent` / `_memory_percent` | resource usage |
| `unifi_device_uptime_seconds` | uptime |
| `unifi_device_rx_bytes` / `_tx_bytes` | traffic (cumulative) |
| `unifi_device_info{...,version,state}` | metadata (value 1) |
| `unifi_client_rssi` | signal |
| `unifi_client_tx_rate` / `_rx_rate` | rate in bits/s |
| `unifi_client_connected_seconds` | connected time |
| `unifi_health_status` / `_devices` / `_clients` | per-subsystem health |
| `collector_scrape_errors_total{collector}` | collection errors |
| `collector_scrape_duration_seconds{collector}` | collection latency |
| `collector_scrape_success{collector}` | last cycle success |

## Logs (Loki)

Events are shipped as JSON log lines. **Stream labels** (low cardinality):
`job, vendor, site, level, event, model`. High-cardinality fields
(`mac, device, hostname, client_mac`) live inside the JSON line, queryable with
`| json`:

```logql
{job="collector", site="default", event="ap_offline"}
{job="collector"} | json | mac="aa:bb:cc:dd:ee:ff"
{job="collector", level="warning"} | json | hostname=~"phone.*"
```

## Adding a new vendor

The framework is designed so a new vendor requires **no changes to the core**:

1. Create `internal/<vendor>/` and implement the capability interfaces you
   support from `internal/collector` — `DeviceSource`, `ClientSource`,
   `EventSource`, `HealthSource` — returning `internal/models` types.
2. In `cmd/collector/main.go`, construct the adapter and register its collectors
   in `registerCollectors` (a few lines).

That's it — `models`, `collector`, `scheduler`, `metrics`, `loki` and `server`
stay untouched (Open/Closed principle).

## Development

```bash
make test     # go test -race
make cover    # coverage
make lint     # golangci-lint
make build    # static binary in ./bin
make docker   # build image
```

## License

MIT (add a LICENSE file as appropriate).
