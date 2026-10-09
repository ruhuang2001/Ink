# Ink API

The Go service owns authentication, PostgreSQL persistence, printer integration, trusted plugin execution, content collection, scheduling, PNG rendering, and background cleanup.

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
- `make smoke-api`: real API smoke flow for auth, workspace persistence, plugin upload/binding/fetch, and schedule delivery (requires Docker and `uv`; uses a disposable PostgreSQL container and temporary credentials)
- `make reset-db`: remove the local PostgreSQL volume

`make reset-db` deletes local development database data. It is not a production migration or rollback command.

Default smoke runs use an isolated temporary PostgreSQL container and remove it, the temporary credentials, and plugin artifacts on exit. CI can supply an empty disposable database explicitly with `INK_SMOKE_BOOTSTRAP_DB=0 DATABASE_URL=... make smoke-api`; its migrated schema and seeded admin remain in that external database, which the caller must dispose of. This override does not read the database URL from `.env` and should never target a development or production database.

`INK_DEV_ADMIN_CREDENTIALS_PATH` optionally changes where `ink-seed dev` writes initial credentials; relative paths are resolved from the command's working directory. The default remains `server/.dev-admin-password`.

## Endpoint groups

- auth: login, refresh, current user, logout, password change
- workspace: read and write persisted workspace state
- admin: member creation and shared AI configuration
- printers and print jobs: device bindings, queue actions, and job lifecycle
- plugins and schedules: installation, binding, validation, and scheduling
- feedback: printable feedback submission for the admin workflow
- preview: server-side 384px PNG rendering without creating a print job

> **Plugin operator warning:** Plugins, dependency installers/build hooks, and entrypoints execute as trusted server-side code with the API process's privileges. Admin-only installation is not a sandbox; install only plugins and dependencies you trust. See [`docs/PLUGIN_SPEC.md`](../docs/PLUGIN_SPEC.md#security-model).
