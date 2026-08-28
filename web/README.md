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
```

## Application boundaries

- API state is accessed through modules in `src/services`.
- Shared workspace orchestration currently lives in the Pinia workspace store.
- Routes and page metadata are defined in `src/router`.
- User-facing product copy is localized in `src/i18n/messages`.
- Print preview images come from the API; the browser does not independently reproduce physical print layout.
