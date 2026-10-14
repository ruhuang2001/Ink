# Opt-in operational monitoring

Ordinary self-hosting should use Ink's lightweight default with no services from
this package. JSON request logs, request IDs, and health checks remain available;
leave metrics and OTLP endpoints empty to disable metrics, database backlog
sampling, and trace export. You do not need a Grafana password for that setup.

Opt into this package when you need historical graphs and operational alerts.
The base package runs four services. The `logging` and `tracing` profiles are
additional independent choices, adding two services each (eight with both). See
[Self-hosting](SELF_HOSTING.md) for ordinary deployment and
[Observability](OBSERVABILITY.md) for the signal contracts.

This independently managed stack uses Prometheus **v3.5.0**, Grafana **12.1.1**,
Alertmanager **v0.28.1**, and blackbox exporter **v0.27.0**. Version tags are pinned;
review upstream security advisories and upgrade deliberately (tags are not immutable
digest locks). It does not start Ink, PostgreSQL, or application workers.

## Host API and private metrics

Default targets in `monitoring/prometheus/prometheus.yml`:

| Signal | Target seen from containers |
| --- | --- |
| Metrics | `http://host.docker.internal:9091/metrics` |
| Liveness | `http://host.docker.internal:8080/healthz` |
| Readiness | `http://host.docker.internal:8080/readyz` |

Run your **host-based** Go API with `PORT=8080` and enable its optional separate
listener with `METRICS_ADDR=<private-reachable-IP>:9091`. An unset metrics address
disables the registry, metrics collection, and the 30-second database backlog
sampler; enabling this stack alone cannot enable API metrics. Edit the
three target addresses if you use different ports or HTTPS. Blackbox requires an
actual HTTP 200 and does not follow redirects; probes have a five-second deadline.

Docker Desktop and native Linux containers receive an explicit
`host.docker.internal:host-gateway` mapping. On native Linux this normally reaches
the host bridge gateway, **not host loopback**: bind metrics to the host's Docker
bridge IP (inspect the network gateway) and allow that port only from the monitoring
container subnet. On Docker Desktop the gateway lives in the VM; host forwarding
depends on Desktop networking settings. Verify reachability from the containers,
not just with host curl. If Desktop does not forward a loopback listener, use a
reachable private host interface or a restricted host proxy and update the target.
Do not blindly bind metrics to `0.0.0.0:9091`: if that is necessary, firewall the
port from LAN/public clients. Metrics have no authentication and must not be
published by an Internet-facing reverse proxy. Keep `/healthz` and `/readyz`
reachable without application login, but they are not business-operation checks.

## Start and inspect

Only if you choose to run this package, use the commands below from the repository
root. Choose a strong password using your secret manager or shell environment
(never commit it):

```sh
export INK_GRAFANA_ADMIN_PASSWORD='replace-with-a-strong-local-password'
docker compose -f docker-compose.observability.yml config --quiet
docker compose -f docker-compose.observability.yml up -d
docker compose -f docker-compose.observability.yml ps
```

This is a **standalone** Compose project named `ink-observability`; do not combine
it with `docker-compose.yml`. No public monitoring ports are bound:

- Grafana: <http://127.0.0.1:13000>, user `admin` (or `INK_GRAFANA_ADMIN_USER`).
- Prometheus targets/rules/alerts: <http://127.0.0.1:19090>.
- Alertmanager alerts/silences: <http://127.0.0.1:19093>.
- Blackbox: container network only, no host publication.

Grafana provisions **Ink / Ink · Operations**, UID `ink-operations`, backed by the
provisioned `ink-prometheus` datasource. It shows API traffic, HTTP p95, panics,
worker progress and interval-normalized idle age, DB pool pressure, operation
results/duration, all five backlog queues, snapshot freshness and sample errors.
No-data panels mean unknown, not success. HTTP timing includes long-lived streams.
Grafana's first-boot admin password persists in its volume; changing the environment
does not reset an existing account. Prometheus and Alertmanager UIs have no auth;
loopback is the security boundary (use an authenticated SSH tunnel for remote access).

Confirm `ink-api`, `ink-probe`, `blackbox`, and `alertmanager` scrape targets are up.
An up blackbox scrape only proves exporter reachability: separately confirm both
`probe_success{job="ink-probe"}` values are 1. Query operation/backlog metrics once
the corresponding API instrumentation is deployed; before that, empty business
panels and a sampler-missing alert are expected.

Stop only this project with `docker compose -f docker-compose.observability.yml down`.
Do **not** use `down -v` unless you intentionally want to erase monitoring history,
silences, and Grafana accounts. The base stack owns three separate named volumes;
enabling both optional profiles adds three more. Prometheus retains 15 days.
Back up these volumes if history matters.

## Alert policy and response

Thresholds are starting points, not a production SLO. Tune against actual workload.

| Rule | Trigger / persistence | First response |
| --- | --- | --- |
| Metrics unavailable | failed or absent API scrape / 2m | Check optional listener, gateway, firewall, API process |
| Monitor unavailable | failed/absent blackbox or Alertmanager scrape / 2m | Inspect monitoring service logs |
| Probe failed | failed scrape, missing result or non-200 health/readiness / 2m | Inspect API and dependency readiness |
| API high 5xx | `/api/.*` only, >5% over 5m, at least 20 requests / 5m | Inspect route/status and request-ID logs; health failures cannot dilute ratio |
| HTTP panic | any counter increase over 5m / immediate | Inspect sanitized panic/request logs |
| Running worker stalled | scan active and start age >300s / 2m | Check blocked network/DB work; independent of poll interval |
| Idle worker stalled | idle completion age >2 × configured poll interval +300s / 2m | Check worker progress/config; six-hour janitor gets **12h +5m**, then 2m persistence |
| DB pool pressure | acquired/max >85% / 10m | Check slow queries, leaked connections and capacity |
| Operation errors | `result="error"` >10% over 10m, >=10 attempts / 10m | Inspect operation logs/provider failures |
| Operation slow | operation histogram p95 >60s over 10m / 10m | Inspect provider latency and timeouts |
| Actionable backlog old | oldest due >900s and count >0 / 10m | Inspect `plugin_fetch`, `schedule`, or `print_status` worker/provider |
| Sampler stale | latest success older than 300s or absent / 5m | Check DB sampling errors; backlog gauges may be stale |
| Sample errors | any increase over 10m / 5m | Inspect DB connectivity/permissions and sampler freshness |

Zero worker completion timestamps indicate no scan has ever completed; an idle
worker in that state is intentionally unhealthy after the alert persistence period.
Disabled workers do not export series and are not expected to alert. Readiness is
the companion stopped-worker signal; these rules cannot infer the names of omitted
worker series. Missing per-replica metrics should be handled through configured
scrape targets; `absent` additionally catches whole removed jobs/endpoints.
Prometheus cannot report its own complete outage—use an independently operated
external watchdog/dead-man check if that failure must page someone.

Business counters are process-local and summed by operation. The result contract
is `error` versus other outcomes; skipped/canceled outcomes are not counted as
errors. Histograms need observations before quantiles exist. Backlog gauges are
**DB-wide**, not per-process work: count uses `max by(queue)`, age uses the maximum
age across replicas (conservative oldest view), and last-success uses the maximum
timestamp (a fresh replica is enough). Never sum duplicated queue counts. Zero due
timestamps mean no overdue item and are excluded; future timestamps clamp to age
zero. `pending_confirmation` and `submitted_print` are visible context but never
overdue-work alert targets. Use one stack per database; independent databases need
separate deployment labels and aggregation scope, not this shared-DB aggregation.

## Notifications are NOT configured

The default `local-only` receiver has **no integrations**. Alerts are visible in
Prometheus/Alertmanager, but **nothing is delivered** to email, chat or webhooks.
This is intentional; a local sink is not a completed notification deployment.

To enable real notifications, replace/add a receiver in
`monitoring/alertmanager.yml` using a supported integration (for example
`webhook_configs` with your real endpoint and `send_resolved: true`), then route to
its exact name. Mount credentials from a private file outside Git and use the
integration's `*_file` fields where supported; never put tokens in committed URLs.
Validate with `amtool check-config`, restart **only** Alertmanager, and exercise a
controlled firing-and-resolved alert. Verify delivery and content at the destination,
not merely the Alertmanager UI. Configure grouping, repeat cadence and escalation
for your on-call policy. No test alert is sent externally by this repository.

## Optional traces and request-log search

Add `--profile tracing`, `--profile logging`, or both to the same Compose command
only when those additional services are needed. Tracing uses Collector 0.135.0 and Tempo 2.8.2; logging uses
Alloy 1.10.2 and Loki 3.5.5. Set the host API's
`OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:14318` to enable OTLP/HTTP export.
Tracing does not depend on `METRICS_ADDR`; you can instead point the API at an
existing Collector without running this Compose package. With both OTLP endpoint
settings empty, the API creates no exporter.
The SDK records all ingress/operation spans, regardless of remote sampled flags;
Collector retains ERROR spans, traces slower than 3 seconds and 5% of other traces.
No baggage, raw URLs, credentials, request bodies or error strings are recorded.
Request logs carry real trace/span IDs for correlation. Grafana Explore provides
the `ink-tempo` and `ink-loki` datasources; Loki trace IDs link to Tempo.

Collector waits 30 seconds after first seeing a trace before deciding. Increase
`decision_wait` beyond your longest trace duration plus export skew (including SSE,
provider timeouts and multi-item dispatch); increase `num_traces`/memory together.
All spans of a trace must reach the same tail-sampling Collector when scaling out.
SDK queue size is 2048; Collector capacity is 10000 traces and a 192 MiB memory
limiter. Overload, late spans, restarts and exporter outages can lose telemetry;
do not claim every error/slow trace is guaranteed retained. Export failures emit
generic warnings without endpoint/error payloads. Tempo retains 48 hours locally.

For file-based host logs, set `INK_LOG_DIR` to an **absolute dedicated directory**
and have the process supervisor write API stdout into `*.log` there. Default is
the ignored `.amp/in/logs` directory, not automatic collection of existing logs.
Mount only that directory, never `.env`, the repository or Docker's privileged
socket. Alloy forwards only JSON records with `msg="http request completed"`;
legacy business logs are dropped because their privacy policy is not guaranteed.
IDs remain JSON fields, not indexed labels. Loki retains seven days; configure
supervisor log rotation and disk quotas independently. In Explore use
`{service="ink-api"} | json | request_id="..."` or `trace_id="..."`.
Loki (13100), Tempo (13200) and Collector OTLP (14318) bind host loopback only
and have no application authentication; do not expose them publicly. Optional
datasources are unavailable unless their profiles run. Logs/traces are diagnostic
best-effort telemetry, not durable audit records.

## Safe authenticated synthetic

Run `python3 server/scripts/synthetic_preview.py` with `INK_SYNTHETIC_API_URL` and
`INK_SYNTHETIC_ACCESS_TOKEN` supplied by your private secret manager. Use a dedicated
non-admin account, HTTPS except loopback development, and arrange token renewal.
It checks health/readiness, workspace read and a 384px PNG preview; it cannot call
create/submit/retry/device routes. Exit 0 and JSON `success=true` indicate success;
failure returns exit 1 with a generic error. Send its result to your independent
deployment scheduler/watchdog. This repository does not configure a recurring run
or production account. UI browser vitals/RUM are separate from these API contracts.

## Reproducible validation

`make check-observability` validates all profiles, rules, dashboard JSON and a real
API using disposable PostgreSQL. It checks actual PNG preview, job count stability,
metric route isolation, database pause/recovery and clean shutdown. The GitHub
observability workflow runs this gate; it never touches a deployed API or printer.

Individual commands below run from the repository root and create no persistent volumes:

```sh
INK_GRAFANA_ADMIN_PASSWORD=validation-only docker compose -f docker-compose.observability.yml config --quiet
docker run --rm --entrypoint /bin/promtool -v "$PWD/monitoring/prometheus:/etc/prometheus:ro" prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml
docker run --rm --entrypoint /bin/promtool -v "$PWD/monitoring/prometheus:/etc/prometheus:ro" prom/prometheus:v3.5.0 check rules /etc/prometheus/rules.yml
docker run --rm --entrypoint /bin/promtool -v "$PWD/monitoring/prometheus:/etc/prometheus:ro" -w /etc/prometheus prom/prometheus:v3.5.0 test rules rules.test.yml
docker run --rm --entrypoint /bin/amtool -v "$PWD/monitoring/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro" prom/alertmanager:v0.28.1 check-config /etc/alertmanager/alertmanager.yml
docker run --rm -v "$PWD/monitoring/blackbox.yml:/etc/blackbox/blackbox.yml:ro" prom/blackbox-exporter:v0.27.0 --config.file=/etc/blackbox/blackbox.yml --config.check
python3 -m json.tool monitoring/grafana/dashboards/ink.json >/dev/null
```

Rule regressions cover missing/down scrapes and probes, sustained versus sparse API
errors and excluded readiness errors, panic reset versus actual increment, pool
persistence, running versus six-hour idle janitor stalls, replica max count/age,
zero due timestamps, non-actionable queues, business error/duration thresholds,
and missing/stale versus fresh-replica sampler state. These synthetic tests do not
prove production routing, host-network reachability, provider success, or external
notification delivery. Check the live targets and rendered dashboard separately.
