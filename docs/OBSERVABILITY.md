# Observability

The recommended default for ordinary self-hosting is lightweight: JSON request
logs, request IDs, `/healthz`, and `/readyz`, with no monitoring services required.
Metrics collection and tracing are optional. Leave `METRICS_ADDR`,
`OTEL_EXPORTER_OTLP_ENDPOINT`, and `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` empty to
keep them disabled. The [monitoring package](MONITORING.md) and its additional
`logging`/`tracing` profiles are separate opt-in choices.

## Implemented foundation

The API always writes JSON request-completion logs to stdout. It creates a private
Prometheus registry only when `METRICS_ADDR` is configured. With an empty address,
HTTP, worker, business, and database pool metrics are disabled and the database
backlog sampler does not run. A Collector, Grafana, and a metrics server are not
required to process requests.

| Endpoint | Listener | Contract |
| --- | --- | --- |
| `GET /healthz` | API | Process liveness, always 200 while accepting requests. No database or provider calls. |
| `GET /readyz` | API | 200 when PostgreSQL responds and all enabled workers have been started and have not exited; otherwise 503. Database ping has a 1-second timeout. Shutdown immediately makes the probe fail. |
| `GET /metrics` | Separate `METRICS_ADDR` listener | Prometheus exposition; never registered on the API listener. |

`METRICS_ADDR` is empty by default (metrics and sampler disabled). Set it to
`127.0.0.1:9090` for a Prometheus instance on the same host. Container deployments
may bind on their private monitoring network, but must not publish that port or
route it through a public reverse proxy. The endpoint has no application
authentication. An explicitly configured listener failing to bind causes the
process to shut down rather than silently run without the requested endpoint.

Example scrape configuration for Prometheus running on the API host:

```yaml
scrape_configs:
  - job_name: ink-api
    scrape_interval: 15s
    static_configs:
      - targets: ['127.0.0.1:9090']
```

The readiness worker set includes plugin fetching, print scheduling, inbox
cleanup, and print status synchronization when enabled. A transient job failure
does not remove the API from service. Readiness checks worker lifecycle, **not
forward progress**: a blocked worker with an open done channel still passes.
When metrics are enabled, worker scan metrics expose progress for monitoring; automatic stall thresholds
are provided as configurable Prometheus rules, not readiness failures. Readiness does not call AI,
Memobird, or plugin subprocesses and does not return internal error details.

## HTTP metrics and logs

When `METRICS_ADDR` is configured, metrics include:

- `ink_http_requests_total{method,route,status}`
- `ink_http_request_duration_seconds{method,route,status}` (histogram)
- `ink_http_response_size_bytes{method,route,status}` (histogram)
- `ink_http_in_flight_requests`
- `ink_http_panics_total`
- Standard Go runtime and process collectors, where supported by the platform.

Route labels come from `http.ServeMux` templates, e.g.
`/api/v1/print-jobs/{jobID}`. Unmatched requests use `unmatched`; unknown HTTP
methods use `OTHER`. URLs, query strings, user IDs, request IDs, and exception
strings are never metric labels. Histograms can be aggregated across replicas.
Exclude `/healthz`, `/readyz`, and `unmatched` from business API SLO queries;
monitor readiness and scrape failures separately.

Every API response receives a server-generated `X-Request-ID`, including 404,
405, and probes. The same ID appears in JSON error envelopes and completion
logs. Incoming request IDs are not trusted. Completion logs contain only
`request_id`, normalized `method`, `route`, `status`, `duration_ms`,
`response_size_bytes`, `panicked`, and `aborted`, plus standard slog fields.
No request/response body, header, query string, panic value, or client address
is logged by this middleware. Existing business logs have separate policies.
When optional OTLP tracing is enabled, completion logs also include real
`trace_id` and `span_id` fields. They are not metric labels.

A panic before final headers produces a generic JSON 500. A panic after headers
or a flush preserves the status already sent and aborts the response; it logs
`aborted=true` and increments the panic counter when metrics are enabled. An intentional
`http.ErrAbortHandler` abort does not count as an application panic. Status 0
means the request aborted before final headers. Size measures successfully
written response body bytes, not headers or on-wire compressed size.

Example API p95:

```promql
histogram_quantile(0.95,
  sum by (le, route) (
    rate(ink_http_request_duration_seconds_bucket{route=~"/api/v1/.*"}[5m])
  )
)
```

## Worker progress and database pool metrics

With metrics enabled, active workers export the following metrics with a static `worker` label:
`plugin_fetch`, `schedule`, `inbox_janitor`, or `print_status_sync`. Disabled
print status synchronization exports no worker series.

- `ink_worker_scans_total{worker,result}` — `success`, `error`, or `canceled`.
- `ink_worker_scan_duration_seconds{worker}` — completed scan histogram.
- `ink_worker_scan_in_progress{worker}` — 1 while a scan is executing, else 0.
- `ink_worker_last_scan_started_timestamp_seconds{worker}`
- `ink_worker_last_scan_completed_timestamp_seconds{worker}`
- `ink_worker_last_successful_scan_timestamp_seconds{worker}`
- `ink_worker_poll_interval_seconds{worker}`

Timestamps start at zero. Successful empty scans update success and completion
timestamps too. `success` means the batch returned no error, **not that every
task succeeded**; per-item failures may be handled inside the service. A pure
context cancellation is `canceled`; deadlines and errors joined with
cancellation are `error`. Error text, task IDs and user IDs are never labels.
Metrics do not change worker retries, scheduling, timeouts, or readiness.

For a running scan, inspect its elapsed time (also exposes a blocked first scan):

```promql
(time() - ink_worker_last_scan_started_timestamp_seconds)
  and (ink_worker_scan_in_progress == 1)
```

For an idle worker, inspect time since completion against its poll interval.
Do not apply a universal one-minute heartbeat threshold: the janitor normally
waits six hours, and execution time depends on batch size and provider budgets.
These are diagnostic signals, not pre-calibrated alerts. Monitor Prometheus
`up` separately; process restarts reset counters and timestamps.

With metrics enabled, the primary PostgreSQL pool exports `ink_db_pool_` metrics:

- Gauges: `connections`, `max_connections`, `acquired_connections`,
  `idle_connections`, `constructing_connections`.
- Counters: `acquires_total`, `empty_acquires_total`, `canceled_acquires_total`,
  `acquire_duration_seconds_total`.

Scrapes read only in-memory `pgxpool.Stat()` values, without querying PostgreSQL
or acquiring a connection. Pool values are process-local and independently
sampled, not a transactionally consistent snapshot across all gauges.
Acquisition duration includes all successful acquisitions, not just waits;
`empty_acquires_total` identifies those that waited. These are pool-pressure
signals, not query latency or database availability probes.

## Individual business operations

With metrics enabled, `ink_operations_total{operation,result}` and
`ink_operation_duration_seconds{operation}` distinguish batch scanning from
actual operations: `plugin_fetch`, `schedule`, `delivery`, `print_submit`,
`print_status`, and `png_render`. Results are `success`, `error`, or `canceled`.
Plugin fetch includes ingestion and success persistence. Delivery success means
a local print job was created/reused and delivery saved, **not physical printing**.
Print submission success means provider submission and local persistence succeeded.
A successful status check may still say not printed; exhausted polling is an error.
PNG metrics cover actual preview and submission rendering, including size rejection.
Manual and automatic schedule runs with failed individual deliveries count as errors even when the
batch API returns no error. Metrics do not change existing retries or state transitions.

## Cached database-wide backlog

Only when `METRICS_ADDR` is configured, a background query samples five fixed
queues every 30 seconds with a two-second context timeout. It does not run in the
default lightweight setup, and Prometheus scrapes never run this query. `ink_backlog_items{queue}`
and `ink_backlog_oldest_due_timestamp_seconds{queue}` expose counts and oldest due
times, with zero time for an empty queue. `ink_backlog_last_success_timestamp_seconds`
and `ink_backlog_sample_errors_total` distinguish stale data from healthy emptiness.
On failure the previous complete sample is retained; before first success there
are no queue series. Timeout limits query time, not the number of matching rows;
large tables need query-plan/capacity review.

`plugin_fetch` and `schedule` mirror due-claim predicates, excluding active leases.
`print_status` includes due accepted-job checks and provider-less queued jobs at
the three-minute unknown-submission recovery boundary. `pending_confirmation`
and `submitted_print` are informational populations (their timestamps are creation
times), not actionable overdue work. Counts cover due scans/checks, not all inbox
items nor schedule-relative retry candidates. A submitted job can also be due for
a status check; these populations must not be added together. All replicas observe
the same database: aggregate queues with **max, not sum**.

## Collection, logs, traces and operational rules

[Monitoring runbook](MONITORING.md) describes the standalone opt-in Compose stack,
provisioned Grafana dashboard, Prometheus/blackbox probes, Alertmanager rules,
optional Alloy/Loki request-log collection and OTLP Collector/Tempo tracing.
The four-service monitoring package is optional; enabling both additional profiles
starts eight services in total. Ordinary deployments need no Grafana password.

Trace export is independently opt-in: set `OTEL_EXPORTER_OTLP_ENDPOINT` to an
OTLP/HTTP base endpoint, or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` to the full
traces endpoint (including `/v1/traces`). Both are empty by default, so no
exporter is created. You can use an existing Collector without enabling metrics
or starting the bundled monitoring package.

Request spans use route templates; operation child spans contain no error strings,
IDs or content. W3C trace context is accepted without baggage. The SDK uses bounded,
nonblocking batching and full ingress sampling; Collector tail policies retain
ERROR and >3-second traces plus 5% ordinary traffic. Finite queues, outages and
late/incomplete traces mean this is not a lossless guarantee. Calibrate decision
wait, capacity and worker/backlog alert budgets for real traffic before claiming
an SLO. No external notification receiver is configured by default.

`server/scripts/synthetic_preview.py` uses a dedicated account's access token to
check probes, workspace reads and actual PNG preview without creating or submitting
print jobs. It rejects redirects and never prints token/account content. Scheduling
and token renewal belong to the deployment operator; browser RUM/production INP
collection is not implemented by this backend monitoring stack.

## Verification

```sh
make check-api
make check-observability
cd server && go test -race ./internal/platform/httpapi ./internal/platform/observability ./internal/platform/config ./internal/scheduler
```

Tests cover bounded labels, sensitive-data exclusion, request correlation,
implicit status/body bytes, panic before/after headers and flush, intentional
aborts, readiness dependency failures and timeout, and registry/listener route
isolation. Worker tests exercise all four real runners with idle, blocked,
failed, timed-out, canceled, and mixed failure/cancellation scans through the
metrics HTTP handler. Pool tests scrape without a database and verify canceled
acquisition counters. Real deployment acceptance should also stop PostgreSQL and verify
that `/readyz` becomes 503 while `/healthz` stays 200, then restore PostgreSQL
and verify readiness recovers.
