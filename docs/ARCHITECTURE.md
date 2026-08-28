# Ink Architecture

Ink is a single Go API with background workers, a Vue single-page application, PostgreSQL persistence, trusted plugin subprocesses, and a Memobird provider integration.

## System overview

```text
Browser
  │ HTTPS / JSON
  ▼
Vue SPA ───────────────► Go HTTP API
                           │
            ┌──────────────┼─────────────────┐
            ▼              ▼                 ▼
       PostgreSQL    Plugin subprocesses   Memobird API
            ▲          Node.js / Python       ▲
            │                                  │ PNG
            └──── fetch, schedule, and delivery workers
```

The frontend and API are deployed separately. The frontend calls versioned `/api/v1/...` endpoints; the API owns authentication, persistence, plugin execution, scheduling, rendering, and provider calls.

The production frontend is built as an initial application chunk plus route
chunks. Unicode-aware font subsets and route chunks are fetched on demand so a
first visit does not download every page or the full CJK font file.

## Repository layout

- `web/src`: Vue views, Pinia state, API clients, routes, shared types, and i18n resources.
- `server/cmd/api`: application composition and process lifecycle.
- `server/internal`: domain services and platform adapters.
- `server/internal/platform/store/postgres`: PostgreSQL repositories.
- `server/migrations`: ordered SQL migrations.
- `server/testdata/plugins`: local plugin fixtures used by real subprocess tests.
- `docs/PLUGIN_SPEC.md`: public plugin contract.

## Runtime components

### HTTP API

The API exposes authentication, workspace, printer, plugin, schedule, feedback, and preview endpoints. JSON bodies use strict decoding and endpoint size limits. The server applies read, write, idle, and header timeouts and shuts workers down before closing the database pool.

### PostgreSQL

PostgreSQL stores users, refresh sessions, workspace state, printer bindings, print jobs, plugin installations and bindings, collected items, print schedules, and delivery records.

Important idempotency boundaries:

- plugin content is unique by `(plugin_binding_id, external_id)`;
- schedule delivery is unique by `(print_schedule_id, plugin_item_id)`;
- delivery reservations use leases so concurrent runners do not create duplicate local jobs.

### Background workers

The API process starts three periodic workers:

- plugin fetch worker: claims enabled bindings whose `next_fetch_at` is due;
- schedule worker: runs enabled print schedules and reserves deliveries;
- inbox janitor: removes retained plugin items according to server configuration.

This is currently a single-process operational model. Horizontal scaling requires careful coordination of rate limiting, migrations, and worker ownership.

### Plugin runtime

Plugins are installed under `PLUGIN_ROOT` and executed as local subprocesses. Ink supports Node.js and Python package layouts with locked dependencies.

Each invocation receives:

- JSON through stdin;
- a minimal environment plus explicit `PLUGIN_ENV_ALLOWLIST` entries;
- an invocation-scoped temporary home and cache;
- execution and output limits.

These controls reduce accidental exposure but do not provide a security sandbox. Plugin code and build hooks remain trusted code.

### Print pipeline

Plugins emit semantic content blocks. Ink validates and flattens those blocks into printable text, then renders the title and content to a 384px-wide PNG using the bundled Chinese font. Both preview and physical printing use this server-side renderer.

```text
plugin blocks
  → validation
  → stored plugin item
  → schedule delivery
  → printable text
  → 384px PNG
  → Memobird API
```

Manual preview stops after PNG generation and never creates a job or contacts Memobird.

## Plugin and schedule separation

Collection and delivery are independent:

1. A plugin binding fetches content at the interval declared by its manifest.
2. Fetched items are stored and deduplicated.
3. A print schedule selects undelivered items for one binding.
4. Each schedule tracks its own delivery records, so two schedules may print the same collected item independently.

This prevents network collection timing from being coupled to the desired print time.

## Authentication and security boundaries

- Access tokens are JWTs tied to persisted refresh sessions.
- Refresh tokens rotate and are hashed in PostgreSQL.
- The web client currently stores session tokens in Web Storage; XSS can therefore compromise a browser session.
- Login throttling is bounded but process-local.
- AI credentials and plugin binding secrets are encrypted using `AI_CONFIG_ENCRYPTION_KEY`.
- Plugin installation is administrator-only, but administrator authorization does not make plugin code safe.
- Browser API calls use one typed client with request timeouts, cancellation,
  request IDs, and single-flight access-token refresh coordination.

See [Security Policy](../SECURITY.md) and [Plugin Security Model](PLUGIN_SPEC.md#security-model).

## Failure and retry model

- Failed plugin fetches record an error and schedule a later attempt.
- Delivery claims are leased and recoverable after lease expiry.
- Failed schedule delivery records are retryable up to the configured attempt boundary.
- A provider request accepted immediately before a local crash cannot be made perfectly idempotent unless the provider offers its own idempotency or reconciliation API.

## Current limitations

- No enforceable plugin sandbox or public untrusted plugin marketplace.
- No built-in production backup/restore workflow.
- No readiness or metrics endpoint.
- Login rate limiting is not global across multiple API instances.
- Refresh credentials remain accessible to browser JavaScript.
- The release workflow does not currently publish a complete production deployment image.
