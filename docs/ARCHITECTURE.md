# Ink Architecture

Ink 1.0 is a single Go API with print-status synchronization, a Vue single-page application, PostgreSQL persistence, AI provider access, and a Memobird integration.

## System overview

```text
Browser
  │ HTTPS / JSON
  ▼
Vue SPA ───────────────► Go HTTP API
                           │
            ┌──────────────┼─────────────────┐
            ▼              ▼                 ▼
       PostgreSQL      AI provider        Memobird API
            ▲                                  ▲
            │                                  │ PNG / status
            └──── print-status synchronization
```

The frontend and API are deployed separately. The frontend calls versioned `/api/v1/...` endpoints; the API owns authentication, persistence, AI replies, rendering, and provider calls.

The production frontend is built as an initial application chunk plus route
chunks. Route chunks are fetched on demand, and the web console uses native
system Chinese fonts instead of downloading a full CJK web font.

## Repository layout

- `web/src`: Vue views, Pinia state, API clients, routes, shared types, and i18n resources.
- `server/cmd/api`: application composition and process lifecycle.
- `server/internal`: domain services and platform adapters.
- `server/internal/platform/store/postgres`: PostgreSQL repositories.
- `server/migrations`: ordered SQL migrations.
- `server/testdata/plugins`: retained legacy file fixtures for recovery checks.
- `docs/PLUGIN_SPEC.md`: historical plugin contract, unavailable in 1.0.

## Runtime components

### HTTP API

The API exposes authentication, workspace, printer, AI, feedback, and preview endpoints. JSON bodies use strict decoding and endpoint size limits. The server applies read, write, idle, and header timeouts and shuts workers down before closing the database pool.

### PostgreSQL

PostgreSQL stores users, refresh sessions, workspace state, AI configuration, printer bindings, print jobs, and retained legacy plugin installations, bindings, collected items, schedules and delivery records. Legacy rows and files are archival data; the 1.0 API has no plugin/schedule routes or plugin execution.

Workspace saves compare a database revision atomically. An outdated snapshot
returns a conflict without replacing saved content; the browser retains the
local draft and offers export or a confirmed reload. See [Workspace API](WORKSPACE_API.md).

Removing a printer marks its binding offline and disables its schedules.
Print jobs and delivery history remain readable; queued jobs accepted before
removal remain eligible for completion checks. A retained binding can be
reconnected. Legacy schedules remain inactive in 1.0.

### Background workers

The API starts only the print-status worker, which reads a bounded batch of due queued jobs and checks Memobird independently of browser requests. Both `WORKERS_ENABLED` and `PRINT_STATUS_SYNC_ENABLED` must be true. Disabled workers are excluded from readiness and shutdown checks; readiness continues to test PostgreSQL. Legacy fetching, schedule delivery and inbox cleanup never start. Optional metrics use a separate read-only backlog sampler, disabled when `METRICS_ADDR` is empty.

This is currently a single-process operational model. Horizontal scaling requires careful coordination of rate limiting, migrations, and worker ownership.

### Preserved legacy data

Plugin tables, collected items, schedules and delivery records remain in PostgreSQL and backups to protect existing installations. Unknown legacy workspace fields are retained on save. No legacy janitor modifies retained plugin rows. Historical source packages and fixtures are kept for compatibility and recovery tests only.

### Print pipeline

Manual content and AI replies are rendered as a 384px-wide PNG using the bundled Chinese font. Preview and physical printing use the same server-side renderer.

```text
written content / AI reply → input limits → 384px PNG → Memobird API
```

Manual preview stops after PNG generation and never creates a job or contacts Memobird.
It requires an authenticated session and shares content and image-height limits
with physical printing. Oversized input is rejected before image allocation;
the renderer does not truncate it. See [Print API limits](PRINT_API.md#content-limits).

Print lists return paginated summaries without the content body. A separate
detail endpoint supplies content for preview, while a lightweight status
endpoint supplies current states, account-wide counts, and the latest job ID. List,
detail, and status reads never contact Memobird. The browser refreshes account-wide status without relying on its current page being a complete job list.

The status worker persists the next check time and retries provider failures
with backoff. Its conditional update checks the observed provider job and
version so an old result cannot overwrite a cancelled or resubmitted job.
Submission returns the accepted queued state; physical completion is observed
asynchronously. Checks use recoverable database leases and conditional updates; provider calls are bounded and performed sequentially within each worker.

Account workspace snapshots exclude their old print-history copies when read
and saved. The `print_jobs` table remains the source of print content and
history; anonymous browser workspaces retain their local full-content jobs.
See [Print API](PRINT_API.md) and [performance measurements](PRINT_PERFORMANCE.md).

## Authentication and security boundaries

- Access tokens are JWTs tied to persisted refresh sessions.
- Refresh tokens rotate and are hashed in PostgreSQL.
- The web client currently stores session tokens in Web Storage; XSS can therefore compromise a browser session.
- Login throttling is bounded but process-local.
- AI credentials and plugin binding secrets are encrypted using `AI_CONFIG_ENCRYPTION_KEY`.
- Browser API calls use one typed client with request timeouts, cancellation,
  request IDs, and single-flight access-token refresh coordination.

See [Security Policy](../SECURITY.md).

## Failure and retry model

- Print submissions use atomic claims and persistent versions/leases.
- Uncertain submissions require human confirmation rather than automatic resending.
- Status checks use bounded retries/backoff and never resubmit accepted jobs.
- A provider request accepted immediately before a local crash cannot be made perfectly idempotent unless the provider offers its own idempotency or reconciliation API.

## Current limitations

- Plugins and plugin schedules are outside the 1.0 runtime scope.
- No built-in production backup/restore workflow.
- Worker progress, business backlog metrics, and OTLP tracing are available as opt-in capabilities. Tracing is not lossless and no external alert receiver is configured by default. See [Observability](OBSERVABILITY.md).
- Login rate limiting is not global across multiple API instances.
- Refresh credentials remain accessible to browser JavaScript.
- The release workflow does not currently publish a complete production deployment image.
