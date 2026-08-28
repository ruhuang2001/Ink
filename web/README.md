# Ink Web

Vue single-page application for device management, content drafting, plugin configuration, print scheduling, and server-rendered PNG previews.

The development server proxies `/api` to `http://localhost:8080`. Production deployments must serve `dist/` with an SPA fallback and proxy `/api/` to the Go API. See [Self-hosting Ink](../docs/SELF_HOSTING.md).

## Stack

- Vue 3
- TypeScript
- Vite 8
- Vue Router
- Pinia
- Tailwind CSS
- Vitest

## Commands

```bash
pnpm install
pnpm dev
pnpm lint
pnpm lint:fix
pnpm format
pnpm format:check
pnpm check
pnpm test:run
pnpm build
pnpm check:budget
```

## Application boundaries

- API state is accessed through modules in `src/services`.
- Shared workspace orchestration currently lives in the Pinia workspace store.
- Routes and page metadata are defined in `src/router`.
- User-facing product copy is localized in `src/i18n/messages`.
- Print preview images come from the API; the browser does not independently reproduce physical print layout.

`pnpm check:budget` runs after production builds in `pnpm check` and enforces the current release budgets: one entry JavaScript chunk at or below 150 KiB gzip, one entry stylesheet at or below 80 KiB gzip, and each emitted font asset at or below 64 KiB. Route chunks are loaded on demand. The web console uses the operating system's native Chinese font stack, avoiding a multi-megabyte web-font download while preserving native weight rendering.

## Performance baseline

The release branch is measured against a production preview build rather than the Vite development server. The current desktop baseline is first contentful paint around 130 ms. With cache disabled, 150 ms latency, approximately 1.6 Mbps download, and 4x CPU throttling, the measured first contentful paint is about 1.42 s with no web-font transfer. The entry JavaScript is about 105 KiB gzip and the entry stylesheet is about 11 KiB gzip.

Re-run the browser profile after significant changes. Keep the same route, viewport, cache state, network profile, and CPU throttle so results remain comparable.

## Caching and deployment

The service worker caches the application shell and same-origin static assets. API requests are never cached. Hashed build assets may use long-lived immutable cache headers at the reverse proxy; `index.html`, `site.webmanifest`, and `sw.js` should remain revalidatable so new releases can activate promptly. The worker uses a versioned shell cache and stores one navigation fallback rather than every route URL.
