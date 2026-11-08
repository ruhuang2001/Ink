# Repository Guidelines

Ink is a two-part repository with shared maintainer docs at the root.

## Project Structure & Module Organization

- [`web/`](web) contains the Vue 3 + TypeScript frontend. Source lives in [`web/src/`](web/src):
  `app/` for the root component and theme/locale wiring, `views/` for route screens,
  `layouts/` for shared chrome, `router/` for navigation and auth redirects, `stores/` for
  Pinia state, `services/` for API adapters, `components/` for shared UI, `i18n/` for
  messages, `types/` and `utils/` for shared shapes and helpers, and `test/` for shared
  Vitest setup. Imports use the `@/*` alias for `web/src/*`.
- [`server/`](server) contains the Go API. `cmd/` has entrypoints (`api`, `migrate`, `seed`,
  `smoke-fixture`). `internal/` holds domain packages (`ai`, `auth`, `content`, `feedback`,
  `printer`, `scheduler`, `session`, `user`, `workspace`) plus `internal/platform/` for
  infrastructure (`config`, `httpapi`, `store/postgres`, `migrate`, `seed`, `observability`,
  `token`, `password`, `secret`, `clock`, `idgen`). `migrations/` stores ordered SQL schema
  changes and `scripts/` holds local bootstrap helpers and the API smoke test.
- [`docs/`](docs) contains maintainer-facing architecture, API, operations and release docs,
  plus [`docs/design/`](docs/design) for interface previews and screenshots.
- Top-level automation and policy files live in [`.github/`](.github), alongside
  [`Makefile`](Makefile), [`docker-compose.yml`](docker-compose.yml) (PostgreSQL only),
  [`docker-compose.observability.yml`](docker-compose.observability.yml) (optional monitoring
  stack), and community health files.

## Build, Test, and Development Commands

Run common workflows from the repo root with `make`.

- `make dev-web`: start the Vite dev server (proxies `/api` to `http://localhost:8080`).
- `make dev-api`: create `server/.env` when missing, start PostgreSQL, apply migrations, seed
  the `admin` account, and start the API on port `8080`.
- `make dev-db` / `make reset-db` / `make logs-db`: manage the PostgreSQL container.
- `make migrate-up` / `make seed-dev` / `make bootstrap-api`: run individual bootstrap steps.
- `make check-web`: lint, format check, tests with coverage, build, and the performance budget.
- `make check-api`: `gofmt` check, Go tests, and the backend build.
- `make test-web` / `make test-api` / `make build-web` / `make build-api` / `make lint-api`:
  individual steps (lint-api runs `golangci-lint`).
- `make smoke-api`: end-to-end API smoke flow against a throwaway PostgreSQL container.
- `make check-local-ci`: run the web and server CI jobs locally via `act`, then the smoke test.
- `make check-observability`: validate the optional monitoring stack configuration.

Use Node.js 22, pnpm 10 (pinned as `pnpm@10.28.0`), and Go 1.26.10, as documented in
[`CONTRIBUTING.md`](CONTRIBUTING.md) and `server/go.mod`.

Git hooks are installed by `pnpm install`: pre-commit runs `lint-staged` over staged files,
and pre-push runs [`scripts/check-push.sh`](scripts/check-push.sh), which runs the web,
server and Go lint checks natively for whichever paths changed since `origin/main`. Changes
that touch `.github/workflows/` fall back to the full `make check-local-ci` run, which needs
`act` and a running Docker daemon.

## Coding Style & Naming Conventions

Frontend code uses TypeScript, Vue 3 SFCs, Tailwind CSS, `oxlint`, and `oxfmt`. Let the
formatter control spacing and import order; existing code uses double quotes and sorted
imports. Name Vue screens and layouts in PascalCase, for example `StatusView.vue` and
`AppShell.vue`. Keep stores, services, and helper modules in camelCase, for example
`workspace.ts`. Every user-facing string lives in [`web/src/i18n/messages/`](web/src/i18n/messages);
`zh-CN` is the fallback locale and `en-US` mirrors it, so add new keys to both files.

Backend code uses Go with package-oriented structure under `server/internal`. Keep packages
focused on one domain or platform concern, prefer explicit error handling, and run `gofmt`
before submission. HTTP routes are registered in
[`server/internal/platform/httpapi/server.go`](server/internal/platform/httpapi/server.go)
under versioned `/api/v1/...` paths.

## Testing Guidelines

Vitest runs in `jsdom` with shared setup in [`web/src/test/setup.ts`](web/src/test/setup.ts).
Coverage thresholds in [`web/vite.config.ts`](web/vite.config.ts) are `70%` for lines,
functions, and statements, and `60%` for branches. Components keep their tests beside them as
`*.spec.ts`. The performance budget in [`web/scripts/check-performance-budget.mjs`](web/scripts/check-performance-budget.mjs)
caps initial JavaScript at 150 KB gzip, initial CSS at 80 KB, and font assets at 64 KB.

Go tests live beside their packages as `*_test.go`. Tests that need a database read
`INK_TEST_DATABASE_URL` and skip when it is unset; the recovery integration test owns its own
disposable container instead of reading `DATABASE_URL`.

For changes touching auth, workspace persistence, local bootstrap flow, manual printing, or
paginated print reads, run [`make smoke-api`](Makefile). It exercises login, workspace
persistence and conflict protection, PNG preview, manual printing, and paginated
print/status/detail reads against a temporary database.

## Commit & Pull Request Guidelines

Follow the existing commit format: `type(scope): summary`, for example
`feat(web): add print queue filters`. Use one of `feat`, `fix`, `docs`, `refactor`, `test`,
`style`, `chore`, or `perf`, and keep the summary imperative, lowercase, and without a
trailing period.

PRs should stay focused, explain why the change is needed, link related issues when
applicable, list the commands you ran, and include screenshots for visible UI changes. If
behavior, setup, or contributor workflow changes, update the relevant docs in the same change.

## Security & Configuration Tips

Start from [`web/.env.example`](web/.env.example) for frontend env values (`VITE_API_BASE`).
The dev server proxies `/api` to `http://localhost:8080`.

The backend template lives at [`server/.env.example`](server/.env.example). Development
bootstrap writes the seeded admin credentials to `server/.dev-admin-password`. Never commit
secrets, tokens, or device credentials. Security reports belong in [`SECURITY.md`](SECURITY.md),
not public issues.
