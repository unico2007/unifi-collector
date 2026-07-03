# Claude Handoff — UniFi Collector

This is the current project state and the exact context needed to continue work.

## Project

Path:

```bash
/Users/murad/Desktop/unifi-collector
```

This is a Go-based UniFi monitoring collector. It logs into a UniFi controller,
collects devices/clients/health/events, exposes Prometheus metrics on
`/metrics`, pushes events to Loki, and visualizes data in Grafana.

Main stack:

- Go 1.25
- Docker Compose
- Prometheus
- Loki
- Grafana
- UniFi controller at `https://10.10.0.3`

The project is intentionally vendor-extensible. Core packages should stay
vendor-neutral; UniFi-specific logic belongs under `internal/unifi`.

## Important Existing Docs

Read these first:

```bash
HANDOFF.md
README.md
```

`HANDOFF.md` contains the original architecture and TODO list.

## What Was Completed

### 1. Auth failure stampede fixed

Implemented in:

```bash
internal/unifi/client.go
internal/unifi/client_test.go
internal/config/config.go
internal/config/config_test.go
cmd/collector/main.go
configs/config.yaml
```

Fix summary:

- Added `AuthFailureCooldown` to `unifi.Config`.
- Default cooldown is `60s`.
- After an auth failure, login errors are cached.
- During cooldown, `ensureLoggedIn` returns cached auth error without any network call.
- This prevents multiple collectors plus readiness loop from hammering UniFi login.
- Login path does not retry `429`.
- `401`, `403`, `400`, and `429` on login are treated as `ErrAuth`.

Tests added:

- Concurrent auth failure results in exactly one login attempt.
- `429` on login is not retried.

### 2. Grafana dashboard provisioning added

Added:

```bash
configs/grafana/provisioning/dashboards/dashboards.yml
configs/grafana/provisioning/dashboards/json/unifi-collector-overview.json
```

Updated:

```bash
configs/grafana/provisioning/datasources/datasources.yml
```

Dashboard URL:

```text
http://localhost:3000/d/unifi-collector-overview/unifi-collector-overview
```

Grafana path:

```text
Home -> Dashboards -> UniFi -> UniFi Collector Overview
```

Dashboard currently includes:

- Online Devices
- Total Devices
- Connected Clients
- Collector Health
- Device CPU
- Device Memory
- Network Throughput
- Client Count
- Weakest Client Signal
- Top Client RX Rates
- UniFi Health
- Collector Scrape Duration
- Collector Errors
- Recent UniFi Events from Loki

Verified earlier with real data:

- 22 online devices
- 22 total devices
- roughly 130+ connected clients
- device/client/health metrics were visible in Grafana

### 3. Event endpoint fallback started

Files changed:

```bash
internal/unifi/client.go
internal/unifi/events.go
internal/unifi/mapper_test.go
```

Why:

The user's UniFi controller returns `404` for:

```text
GET /proxy/network/api/s/default/stat/event
```

This made `collector_scrape_success{collector="events"}` become `0`, so the
dashboard's `Collector Health` panel turned red even though devices, clients,
and health worked.

Current event fix:

- `Client.PostJSON` was added.
- HTTP status errors now use a structured `statusError`.
- `Events()` first tries `GET stat/event`.
- If GET returns `404` or `405`, it falls back to `POST stat/event`.
- POST body includes:

```json
{
  "_limit": 3000,
  "within": "<milliseconds since last event watermark>"
}
```

- If both GET and POST return `404`/`405`, events are treated as optional:
  `Events()` logs one warning and returns an empty event list with no error.

Tests added:

- GET `stat/event` 404 falls back to POST.
- Unsupported event endpoint returns no error and no events.

## Test Status

These passed after the above changes:

```bash
go test ./...
make test
```

`make test` runs:

```bash
go test -race -count=1 ./...
```

On macOS there is a linker warning from Go/race build:

```text
ld: warning: ... malformed LC_DYSYMTAB ...
```

But tests exit successfully.

## Docker/Grafana Current State

Docker Compose services:

```bash
docker compose ps
```

Expected services:

- `collector`
- `prometheus`
- `loki`
- `grafana`

Grafana login:

```text
username: admin
password: admin
```

Unless the user changed it in the browser or set `GRAFANA_PASSWORD`.

Important: `docker-compose.yml` currently sets:

```yaml
COLLECTOR_UNIFI_PASSWORD: "${UNIFI_PASSWORD:-changeme}"
```

So if you run:

```bash
docker compose up -d --build collector
```

without `UNIFI_PASSWORD`, the collector will use `changeme` and fail login.

Use:

```bash
UNIFI_PASSWORD='REAL_UNIFI_PASSWORD' docker compose up -d --build collector
```

Do not put real passwords in this handoff or commit them to files.

## Very Important Current Runtime Note

The collector was rebuilt/restarted during work. If it is now showing:

```text
unifi: authentication failed (status 403)
```

that likely means it was restarted without the real `UNIFI_PASSWORD`.

Ask the user for the real UniFi password or tell them to run:

```bash
UNIFI_PASSWORD='REAL_UNIFI_PASSWORD' docker compose up -d --build collector
```

Then verify:

```bash
docker compose logs -f collector
curl -i http://localhost:8080/readyz
curl -s 'http://localhost:9090/api/v1/query?query=collector_scrape_success' | jq '.data.result'
```

Expected:

- `/readyz` becomes `200`
- devices/clients/health collectors show success `1`
- events should also show success `1` after the optional-event fallback, unless
  there is a different non-404/non-405 error

## Useful Commands

Run tests:

```bash
go test ./...
make test
```

Build binary:

```bash
make build
```

Run full stack:

```bash
UNIFI_PASSWORD='REAL_UNIFI_PASSWORD' docker compose up -d --build
```

Restart Grafana after dashboard changes:

```bash
docker compose restart grafana
```

Check Prometheus scrape:

```bash
curl -s 'http://localhost:9090/api/v1/query?query=up{job="unifi-collector"}' | jq
```

Check collector success:

```bash
curl -s 'http://localhost:9090/api/v1/query?query=collector_scrape_success' | jq '.data.result'
```

Check UniFi device count:

```bash
curl -s 'http://localhost:9090/api/v1/query?query=unifi_devices_total' | jq '.data.result'
```

Check collector logs:

```bash
docker compose logs -f collector
```

## Known Follow-Up Items

### A. Restore runtime with real password

If collector currently has `403`, restart with real `UNIFI_PASSWORD`.

### B. Confirm event fallback on the real controller

After collector is restarted with real credentials, check:

```bash
curl -s 'http://localhost:9090/api/v1/query?query=collector_scrape_success{collector="events"}' | jq
```

If it is `1`, dashboard health should turn green.

If it stays `0`, inspect:

```bash
docker compose logs --tail=120 collector
```

### C. Improve Compose secret ergonomics

Consider changing `docker-compose.yml` so missing `UNIFI_PASSWORD` fails fast
instead of silently using `changeme`. For example:

```yaml
COLLECTOR_UNIFI_PASSWORD: "${UNIFI_PASSWORD:?set UNIFI_PASSWORD}"
```

This avoids accidental auth failures and account lockout risk.

### D. Optional: remove events from Collector Health

If events are not operationally important, dashboard `Collector Health` can be
changed to ignore the `events` collector:

```promql
min(collector_scrape_success{collector!="events"})
```

But the better fix is the event fallback already started above.

## Design Rules To Preserve

- No global state.
- Do not use `prometheus.DefaultRegisterer`.
- Core packages must not import `internal/unifi`.
- Interfaces live where they are consumed.
- Keep `internal/models` vendor-neutral.
- Preserve constructor dependency injection.
- Keep context-aware blocking calls.
- Add tests for behavior changes.

