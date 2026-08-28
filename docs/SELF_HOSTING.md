# Self-hosting Ink

This guide covers a single-instance Ink deployment. Ink currently ships source code and development infrastructure rather than a production container image, so operators build the web application and Go API from the repository.

## Requirements

- Node.js 22
- pnpm 10
- Go 1.25
- PostgreSQL 16
- `uv` when installing or testing Python plugins
- A Memobird Open Platform access key for physical printing
- HTTPS and a reverse proxy for any deployment exposed beyond localhost

## Local development

Install the root Git hooks and frontend dependencies:

```bash
pnpm install
pnpm --dir web install
```

Start the API stack and web development server in separate terminals:

```bash
make dev-api
make dev-web
```

The API bootstrap performs these steps:

1. creates `server/.env` from `server/.env.example` when missing;
2. starts the PostgreSQL 16 development container;
3. applies all migrations;
4. creates the local `admin` account when missing;
5. stores its generated password in `server/.dev-admin-password`;
6. starts the API on port `8080`.

Vite serves the web application on `http://localhost:5173` and proxies `/api` to the API.

Do not commit `.env`, `.dev-admin-password`, access keys, device identifiers, or database backups.

## Production configuration

Copy the environment template and replace every placeholder secret:

```bash
cp server/.env.example server/.env
```

The minimum production settings are:

```dotenv
APP_NAME=ink
PORT=8080
DATABASE_URL=postgres://ink:change-me@postgres:5432/ink?sslmode=require
JWT_SECRET=generate-a-long-random-secret
AI_CONFIG_ENCRYPTION_KEY=generate-a-32-byte-random-secret
MEMOBIRD_ACCESS_KEY=your-open-platform-access-key
PLUGIN_ROOT=/var/lib/ink/plugins
```

Important settings:

| Variable                   | Purpose                                                                               |
| -------------------------- | ------------------------------------------------------------------------------------- |
| `DATABASE_URL`             | PostgreSQL connection string. Use TLS outside a private local network.                |
| `JWT_SECRET`               | Signs access tokens. Rotating it invalidates existing access tokens.                  |
| `AI_CONFIG_ENCRYPTION_KEY` | Encrypts stored AI keys and plugin binding secrets. Back it up securely.              |
| `MEMOBIRD_ACCESS_KEY`      | Enables Memobird device binding and physical printing.                                |
| `MEMOBIRD_BASE_URL`        | Optional provider endpoint override; leave empty for the SDK default.                 |
| `PLUGIN_ROOT`              | Persistent plugin installation directory. It must survive API restarts.               |
| `PLUGIN_GIT_ALLOWED_HOSTS` | Comma-separated allowlist for Git plugin installation. Keep it narrow.                |
| `PLUGIN_ENV_ALLOWLIST`     | Explicit server environment variables passed to plugin subprocesses. Empty is safest. |
| `TRUSTED_PROXY_CIDRS`      | CIDRs of reverse proxies allowed to supply the configured client-IP header.           |
| `TRUSTED_PROXY_HEADER`     | Either `forwarded` or `x-forwarded-for`; configure it together with trusted CIDRs.    |

Review every variable in [`server/.env.example`](../server/.env.example) before deployment.

## Build and run

Build the frontend and API:

```bash
mkdir -p bin
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go build -C server -o ../bin/ink-api ./cmd/api
go build -C server -o ../bin/ink-migrate ./cmd/migrate
go build -C server -o ../bin/ink-seed ./cmd/seed
```

Apply migrations before starting a new API release:

```bash
cd server
../bin/ink-migrate up
../bin/ink-api
```

Serve `web/dist/` as static files and proxy `/api/` to `127.0.0.1:8080`. The API does not serve frontend assets itself.

Configure the reverse proxy to compress JavaScript, CSS, and font responses.
Use long-lived immutable caching for hashed files under `web/dist/assets/`,
while keeping `index.html`, `site.webmanifest`, and `sw.js` revalidatable. The
service worker caches only the shell and same-origin static assets; it never
caches API responses.

## Reverse proxy

Terminate TLS at the reverse proxy. Forward `/api/` without stripping the `/api` prefix, and serve the SPA with an `index.html` fallback for client-side routes.

Ink ignores forwarded client-IP headers by default. If the proxy controls and sanitizes one header family, configure both settings:

```dotenv
TRUSTED_PROXY_CIDRS=127.0.0.1/32,10.0.0.0/8
TRUSTED_PROXY_HEADER=x-forwarded-for
```

Never trust a CIDR that includes arbitrary clients. Login throttling is process-local; multi-instance deployments need a gateway-level or shared global rate limit.

## First login and printer binding

For development, read the generated credentials from `server/.dev-admin-password`. Production operators should create and manage credentials through a controlled bootstrap process rather than preserving the development password file.

The current first-admin bootstrap command is:

```bash
cd server
../bin/ink-seed dev
```

It creates the `admin` account only when it does not already exist and writes the generated password to `server/.dev-admin-password`. Sign in, change the password immediately, move credentials into your normal secret-management workflow, and remove the generated password file from the deployed host.

After login:

1. open **Devices**;
2. add a printer name and the Memobird device identifier;
3. confirm the binding reports `connected`;
4. create a manual print;
5. use **Print preview** before creating or submitting the job.

Preview generation does not contact Memobird. It returns a PNG rendered by the same 384px-wide image pipeline used for physical prints.

## Persistent data and backups

Back up both:

- PostgreSQL, which stores accounts, bindings, schedules, collected items, deliveries, and print jobs;
- `PLUGIN_ROOT`, which stores installed plugin code and dependencies.

Also retain the encryption key required to decrypt stored AI and plugin secrets. A database backup without its corresponding encryption key is incomplete.

The repository does not yet provide automated production backup or restore tooling. Define and test your own PostgreSQL backup, restore, retention, and rollback procedures before relying on Ink for important workflows.

## Health and operations

- `GET /healthz` reports process liveness.
- Structured API logs are written to stdout.
- Database migrations are explicit and should run once before an application rollout.
- A single API process runs plugin fetching, schedule polling, and inbox cleanup workers.

There is currently no separate readiness endpoint or built-in metrics endpoint.

## Upgrade checklist

1. Back up PostgreSQL and `PLUGIN_ROOT`.
2. Read the release notes and migration changes.
3. Build the new frontend and API.
4. Run migrations once.
5. Restart the API and publish the new frontend assets.
6. Verify `/healthz`, login, printer listing, plugin listing, and PNG preview generation.

## Verification

Before deploying a commit:

```bash
make check-web
make check-api
make check-local-ci
```

The smoke test uses an isolated PostgreSQL container and does not contact the physical printer provider.
