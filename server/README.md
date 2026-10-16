# Ink API

The Go service owns authentication, PostgreSQL persistence, AI configuration and replies, printer integration, PNG rendering, and background print-status synchronization. The 1.0 candidate removes plugin installation, fetching and scheduling from the API. Historical plugin tables and files are preserved without execution.

Ordinary self-hosting uses the lightweight default: JSON request logs, request
IDs, `/healthz`, and `/readyz`, with no monitoring services required. Metrics and
trace export are disabled until explicitly configured.

For deployment instructions, see [Self-hosting Ink](../docs/SELF_HOSTING.md). For component and data-flow details, see [Architecture](../docs/ARCHITECTURE.md).

## Local development

Recommended setup from the repository root:

```bash
make dev-db
make migrate-up
make seed-dev
make dev-api
```

Fast path:

```bash
make dev-api
```

That command creates `server/.env` when missing, ensures PostgreSQL is ready, applies migrations, seeds the development admin account, and starts the API server.

The generated admin credentials are written to `server/.dev-admin-password`.

## Reverse proxies and login limiting

Ink ignores `Forwarded` and `X-Forwarded-For` by default and uses the direct socket peer for audit metadata and login rate limiting. When the API is behind a known reverse proxy, configure both:

- `TRUSTED_PROXY_CIDRS`: comma-separated proxy network CIDRs.
- `TRUSTED_PROXY_HEADER`: exactly `forwarded` or `x-forwarded-for`, matching the one header family the proxy sanitizes and manages.

Never trust a CIDR containing arbitrary clients, and configure every proxy in the selected chain to overwrite or safely append its peer address. Login limits are applied independently by account and client IP. `LOGIN_RATE_LIMIT_MAX_ENTRIES` bounds single-process memory; when the bound is full, unknown identities are denied until expired entries are cleaned. Multi-instance deployments must enforce a global limit at the gateway or move limiter state to shared storage.

## Commands

- `make dev-api`: bootstrap and start the API
- `make migrate-up`: apply SQL migrations from `server/migrations/`
- `make seed-dev`: create the development admin account if it does not already exist
- `make check-api`: `gofmt`, Go tests, and backend build
- `make smoke-api`: real API smoke flow for auth, workspace persistence, devices, print history and PNG preview (requires Docker; uses a disposable PostgreSQL container and temporary credentials)
- `make reset-db`: remove the local PostgreSQL volume

`make reset-db` deletes local development database data. It is not a production migration or rollback command.

Default smoke runs use an isolated temporary PostgreSQL container and remove it and temporary credentials on exit. CI can supply an empty disposable database explicitly with `INK_SMOKE_BOOTSTRAP_DB=0 DATABASE_URL=... make smoke-api`; its migrated schema and seeded admin remain in that external database, which the caller must dispose of. This override does not read the database URL from `.env` and should never target a development or production database.

`INK_DEV_ADMIN_CREDENTIALS_PATH` optionally changes where `ink-seed dev` writes initial credentials; relative paths are resolved from the command's working directory. The default remains `server/.dev-admin-password`.

## Endpoint groups

- auth: login, refresh, current user, logout, password change
- workspace: read and write persisted workspace state
- admin: member creation and shared AI configuration
- printers and print jobs: device bindings, queue actions, and job lifecycle
- feedback: printable feedback submission for the admin workflow
- preview: server-side 384px PNG rendering without creating a print job

Structured content blocks remain supported by preview/rendering; they are part of the core content model and do not require plugin execution. The [historical plugin specification](../docs/PLUGIN_SPEC.md) is archived reference material, not an available 1.0 endpoint contract.

Set `WORKERS_ENABLED=false` for isolated recovery checks to stop print-status synchronization. Manual API operations remain enabled; also clear provider credentials and isolate outbound networking as described in [Backup and restore](../docs/BACKUP_RESTORE.md#safe-recovery-api-checks).

## Observability

Leave `METRICS_ADDR` empty to disable the Prometheus registry, HTTP/worker/
business/pool metrics, and periodic database backlog sampling. Keep both
`OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` empty to
disable trace export. Tracing can be enabled independently of metrics.

See [Observability](../docs/OBSERVABILITY.md) for the signal contracts and
[Monitoring](../docs/MONITORING.md) for the optional Compose package. Ordinary
deployments do not need that package, its logging/tracing profiles, or a Grafana
password.
