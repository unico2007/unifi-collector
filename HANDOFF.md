# Project Handoff — UniFi Collector (Go)

> This document is a complete, self-contained brief for an AI coding agent
> (e.g. Codex) or a new developer. Read it top to bottom before making changes.
> The codebase is small, idiomatic Go, fully building, `gofmt`-clean, and unit
> tested. There is one known design issue to fix — see **§9 TODO**.

---

## 1. What this is

An **enterprise-grade, vendor-extensible monitoring collector**. It connects to a
UniFi controller, collects device/client/health stats and events, exposes them
as **Prometheus** metrics (`/metrics`), and ships events to **Loki**. Grafana
visualizes both.

**The key idea: this is a framework, not a UniFi-only tool.** UniFi is just the
first *vendor adapter*. QNAP, Hikvision, FortiGate, Cisco, MikroTik, SNMP and
Syslog can be added later **without changing the core** — vendor-specific code is
confined to `internal/unifi`.

## 2. Tech stack

- Go 1.25
- `net/http` + `net/http/cookiejar` (UniFi client)
- `go.uber.org/zap` (structured logging)
- `github.com/spf13/viper` (YAML config + env overrides)
- `github.com/prometheus/client_golang` (metrics)
- Loki HTTP push API (no SDK, plain JSON)
- `golang.org/x/sync/errgroup` (lifecycle)
- Docker + docker-compose (collector, Loki, Prometheus, Grafana)

## 3. Design principles (follow these when editing)

1. **No global state.** Everything is constructed and injected (constructor DI).
   No package-level singletons, no `prometheus.DefaultRegisterer`.
2. **Vendor-neutral core.** `internal/models`, `internal/collector`,
   `internal/scheduler`, `internal/metrics`, `internal/loki`, `internal/server`
   must never import a vendor package or mention "unifi" in logic.
3. **Interfaces defined by the consumer.** Source/Sink interfaces live in
   `internal/collector`; implementations (`unifi`, `metrics`, `loki`) import
   `collector`, never the reverse. This prevents import cycles.
4. **Interface Segregation.** A collector depends on the narrowest interface it
   needs (e.g. `DeviceSource` + `DeviceMetricSink`), not a fat interface.
5. **Each package owns its own Config struct.** `unifi.Config`, `loki.Config`,
   `server.Config` are separate from `internal/config`. The composition root
   (`cmd/collector/main.go`) maps the app config onto them. Keeps packages
   decoupled from the app's config shape.
6. **Context everywhere.** All blocking work takes `context.Context` and honors
   cancellation. Graceful shutdown is driven by a single root context.
7. **Compile-time interface assertions.** e.g. `var _ collector.Source =
   (*unifi.Client)(nil)` — breaks the build if a contract is violated.
8. **Every package has table/black-box unit tests.** Keep it that way.

## 4. Directory layout & per-file responsibility

```
cmd/collector/main.go        # composition root: builds & injects everything,
                             # runs loki/scheduler/server under one errgroup ctx,
                             # -healthcheck subcommand, readiness login loop.

internal/
  config/config.go           # Config structs + Load(): Viper YAML + env
                             #   (COLLECTOR_<SECTION>_<KEY>) + defaults + validate.
  logger/logger.go           # New(Config) *zap.Logger. Nop() for tests. No global.
  models/models.go           # Vendor-neutral domain types: Device, Client,
                             #   Event (+EventType consts), Health. Has Labels map
                             #   for extra dims. NO vendor fields allowed here.

  collector/                 # THE CONTRACTS (vendor-neutral):
    collector.go             #   Collector interface {Name(); Collect(ctx)} + Registry
    source.go                #   DeviceSource/ClientSource/EventSource/HealthSource
                             #     capability interfaces + aggregate Source.
    sink.go                  #   DeviceMetricSink/ClientMetricSink/HealthMetricSink
                             #     + LogSink interfaces.
    device_collector.go      #   reads DeviceSource -> DeviceMetricSink
    client_collector.go      #   reads ClientSource -> ClientMetricSink
    health_collector.go      #   reads HealthSource -> HealthMetricSink
    event_collector.go       #   reads EventSource -> LogSink; tracks lastSeen
                             #     watermark so only NEW events are forwarded.

  unifi/                     # THE ONLY VENDOR-SPECIFIC PACKAGE:
    client.go                #   authenticated HTTP core: login (auto-detect
                             #     UniFi-OS vs classic), cookie jar, auto re-login
                             #     on 401/LoginRequired, CSRF, retry, timeout.
    source.go                #   Name()="unifi", collector.Source assertion, atof.
    devices.go               #   rawDevice + Devices()  (stat/device)
    clients.go               #   rawClient  + Clients()  (stat/sta)  kbps->bits/s
    events.go                #   rawEvent   + Events()   (stat/event) + classifyEvent
    health.go                #   rawHealth  + Health()   (stat/health)

  metrics/metrics.go         # Prometheus exporter. Owns a private *Registry.
                             #   Implements the 3 metric sinks. RESET-then-write
                             #   per scrape to avoid stale series. ObserveScrape()
                             #   for collector self-metrics. Handler() -> /metrics.
  loki/loki.go               # Loki push exporter. Implements LogSink. Buffered
                             #   channel + background Run(ctx) that batches by
                             #   size/time, groups into streams, ships JSON, flushes
                             #   on shutdown. Low-cardinality stream labels; high-
                             #   cardinality fields go inside the JSON log line.
  scheduler/scheduler.go     # Per-collector goroutine + ticker. Per-cycle timeout.
                             #   Reports outcome to a ScrapeObserver (metrics).
                             #   Graceful stop via ctx + WaitGroup.
  server/server.go           # HTTP: /metrics, /healthz (liveness, always 200),
                             #   /readyz (readiness, atomic bool). Graceful Shutdown.

configs/
  config.yaml                # main runtime config (edit unifi.url/username here)
  prometheus.yml             # scrapes collector:8080
  loki-config.yml            # single-binary Loki, filesystem storage
  grafana/provisioning/datasources/datasources.yml  # Prometheus + Loki datasources

Dockerfile                   # multi-stage: golang:1.25 -> distroless static nonroot
docker-compose.yml           # collector + loki + prometheus + grafana
Makefile                     # build/run/test/cover/lint/docker/up/down
.golangci.yml                # linters
.github/workflows/ci.yml     # vet + race test + coverage + lint + docker build
README.md                    # user-facing docs
```

## 5. Data flow

```
UniFi Controller
   │  HTTPS REST (stat/device, stat/sta, stat/event, stat/health)
   ▼
unifi.Client  ── maps raw JSON ─▶ internal/models (neutral)
   │  (satisfies collector.Source)
   ▼
Collectors (device/client/health/event)   ← scheduler calls Collect(ctx) on ticks
   │                     │
   ▼ metric sinks        ▼ log sink
metrics.Metrics      loki.Exporter
   │ /metrics             │ push API (batched)
   ▼                      ▼
Prometheus             Loki
   └──────► Grafana ◄──────┘
```

Scheduler wraps each `Collect` in a per-cycle timeout and calls
`metrics.ObserveScrape(name, duration, err)` (via the `ScrapeObserver` interface)
so every cycle updates `collector_scrape_*` metrics.

## 6. Key contracts (exact signatures)

```go
// internal/collector
type Collector interface {
    Name() string
    Collect(ctx context.Context) error
}
type DeviceSource interface { Devices(ctx context.Context) ([]models.Device, error) }
type ClientSource interface { Clients(ctx context.Context) ([]models.Client, error) }
type EventSource  interface { Events(ctx context.Context, since time.Time) ([]models.Event, error) }
type HealthSource interface { Health(ctx context.Context) ([]models.Health, error) }
type Source interface { Name() string; DeviceSource; ClientSource; EventSource; HealthSource }

type DeviceMetricSink interface { RecordDevices([]models.Device) }
type ClientMetricSink interface { RecordClients([]models.Client) }
type HealthMetricSink interface { RecordHealth([]models.Health) }
type LogSink          interface { WriteEvents(ctx context.Context, []models.Event) error }

// internal/scheduler
type ScrapeObserver interface {
    ObserveScrape(collectorName string, duration time.Duration, err error)
}
```

## 7. Configuration

`configs/config.yaml`; every value overridable via `COLLECTOR_<SECTION>_<KEY>`
(e.g. `COLLECTOR_UNIFI_PASSWORD`, `COLLECTOR_LOKI_URL`). Sections: `server`,
`logging`, `unifi`, `loki`, `collectors` (map keyed by collector name, each with
`enabled` + `interval`).

Validation (in `config.validate`): `unifi.url`, `unifi.username`,
`unifi.password` required; `loki.url` required if `loki.enabled`; enabled
collectors need a positive interval.

**Important operational note:** the event collector is only registered when Loki
is enabled (events have no other sink). If `loki.enabled: false`, events are
skipped with a warning.

## 8. Build / run / test

```bash
go run ./cmd/collector --config configs/config.yaml   # run (NOT `go run main.go`)
make test          # go test -race
make cover         # coverage (~62% total; business logic pkgs 72-95%)
make build         # static binary -> bin/collector
make docker        # build image
UNIFI_PASSWORD=... docker compose up -d --build        # full stack
```

Endpoints: `:8080/metrics`, `:8080/healthz`, `:8080/readyz`.

### Verified working (end-to-end)
Against a mock UniFi server the collector: auto-detected UniFi-OS, logged in,
flipped `/readyz` to ready, scraped device/client/health, produced correct
metrics (`unifi_device_cpu_percent`, `unifi_client_rssi`, `unifi_devices_total`,
`collector_scrape_success=1`, ...), and shut down cleanly on SIGTERM (exit 0).

## 9. TODO — KNOWN ISSUE TO FIX (highest priority)

**Login storm / auth-failure stampede.** When UniFi login **persistently fails**
(wrong credentials, MFA, etc.), every collector cycle (4 collectors) *plus* the
readiness loop each independently call `ensureLoggedIn` -> `doLogin`, hammering
the controller. Real UniFi-OS controllers rate-limit and then **lock the account**
(`429 AUTHENTICATION_FAILED_LIMIT_REACHED`). This was observed live: the collector
locked out a real controller within seconds.

Additionally, `429` is currently treated as a *transient* status in
`sendWithRetry`, so a failed login is retried 4× with backoff — which makes the
lockout worse.

**Required fix (in `internal/unifi/client.go`):**
1. Add an **auth-failure cooldown**: when `doLogin` fails with an auth error
   (401/403/`ErrAuth`), cache the failure + a timestamp under the existing mutex.
   In `ensureLoggedIn`, if we are within the cooldown window (e.g. 60s,
   configurable), return the cached error **without any network call**. This
   stops all collectors + the readiness loop from stampeding the controller.
2. Do **not** retry `429` on the login path (or honor `Retry-After`). Keep `429`
   retry only for read endpoints, and even there cap it. Consider distinguishing
   the login path from data paths in `sendWithRetry`.
3. (Optional) Surface auth state via a metric, e.g. `unifi_up` / an auth-failure
   counter, and consider gating collectors on readiness so they don't even try
   while not logged in.

Add unit tests: (a) N concurrent `ensureLoggedIn` calls after an auth failure
result in exactly **one** network attempt within the cooldown; (b) `429` on login
is not retried into a storm.

### Other, lower-priority improvements
- Grafana dashboard JSON + provisioning (panels for devices/clients/health/events).
- Prometheus alert rules (AP offline, high CPU, scrape errors) + Alertmanager.
- Loki push failures currently only logged; add a metric/counter for them.
- `stat/event` is fetched via GET (recent window). For high-volume sites consider
  the POST `stat/event` with `within`/`_limit` params.
- Secret management (Docker secrets / Vault) instead of env password.
- The project is not a git repo yet; run `git init` and commit.

## 10. How to add a new vendor (the whole point of the architecture)

Example: add MikroTik.
1. Create `internal/mikrotik/` with a `Client` that implements the capability
   interfaces it supports from `internal/collector` — at minimum `DeviceSource`,
   returning `internal/models` types. Give it its own `mikrotik.Config`. Add a
   compile-time assertion for whatever aggregate/interfaces it satisfies.
2. In `cmd/collector/main.go`:
   - build the adapter from config,
   - in `registerCollectors`, register the relevant collectors
     (`collector.NewDeviceCollector(mkClient, mx, log)` etc.).
3. Add a `mikrotik:` section to `internal/config/config.go` + `config.yaml`.

**Nothing in `models`, `collector`, `scheduler`, `metrics`, `loki`, `server`
changes.** That's the Open/Closed guarantee. Note the metrics namespace is
currently `"unifi"` (constant `metricNamespace` in main); for a true multi-vendor
deployment, either make it neutral (e.g. `"net"`) and rely on the `vendor` label,
or give each adapter its own namespace — decide and document this.

## 11. Conventions cheat-sheet
- Errors wrapped with `%w` and a package prefix (`"unifi: ..."`).
- Tests are black-box (`package foo_test`) except where they need internals.
- HTTP servers/clients always use `context` + timeouts; bodies always closed.
- Run `gofmt -w` and `go vet ./...` before finishing; keep `-race` green.
