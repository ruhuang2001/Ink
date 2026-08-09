# Ink API

The Go service in `server/` backs authentication, workspace persistence, printers, plugins, schedules, and feedback flows for Ink.

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
- `make smoke-api`: real API smoke flow for auth, workspace persistence, plugin upload/binding/fetch, and schedule delivery (requires `uv`)
- `make reset-db`: remove the local PostgreSQL volume

## Endpoint groups

- auth: login, refresh, current user, logout, password change
- workspace: read and write persisted workspace state
- admin: member creation and shared AI configuration
- printers and print jobs: device bindings, queue actions, and job lifecycle
- plugins and schedules: installation, binding, validation, and scheduling
- feedback: printable feedback submission for the admin workflow

> **Plugin operator warning:** Plugins, dependency installers/build hooks, and entrypoints execute as trusted server-side code with the API process's privileges. Admin-only installation is not a sandbox; install only plugins and dependencies you trust. See [`docs/PLUGIN_SPEC.md`](../docs/PLUGIN_SPEC.md#security-model).
