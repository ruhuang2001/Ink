![Ink Logo](assets/logo.png)

# Ink

Ink is a self-hosted workspace for collecting content and printing it on Memobird thermal printers. It combines a Vue web console, a Go API, PostgreSQL persistence, scheduled source plugins, and a server-side PNG print pipeline.

> Ink is an early `0.x` project. APIs, database schemas, and plugin contracts may change before `1.0`.

## What it does

- Manage Memobird devices, print jobs, and recurring print schedules.
- Install trusted Node.js or Python source plugins from ZIP archives or allowlisted Git hosts.
- Fetch external content on a plugin-defined interval, deduplicate it, and deliver it through independent print schedules.
- Render previews and physical jobs through the same 384px-wide PNG pipeline.

## Self-hosting

See [Self-hosting](docs/SELF_HOSTING.md) for prerequisites, local development, production configuration, reverse-proxy guidance, database operations, and a first printer binding.

The shortest local path uses two terminals:

```bash
make dev-api
make dev-web
```

`make dev-api` creates a local `server/.env` when needed, starts PostgreSQL, applies migrations, seeds the `admin` development account, and starts the API on `http://localhost:8080`. The generated credentials are written to `server/.dev-admin-password`; keep that file private.

## Documentation

- [Self-hosting guide](docs/SELF_HOSTING.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Plugin development and protocol](docs/PLUGIN_SPEC.md)
- [Server reference](server/README.md)
- [Frontend reference](web/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## Security limits

Plugins are trusted server-side programs. Their dependency installers and entrypoints can run with the API process's privileges. Permission declarations are review metadata, not a sandbox. Install only code and dependencies you trust; do not expose plugin installation to untrusted users.

The browser currently stores access and refresh session tokens in Web Storage. Deploy Ink behind HTTPS, use a restrictive Content Security Policy, and treat browser XSS as a session-compromise risk until cookie-based refresh sessions are implemented.

## Quality checks

```bash
make check-web
make check-api
make check-local-ci
```

`make check-local-ci` runs the web and server CI jobs locally with `act`, plus the API smoke test against an isolated PostgreSQL container.

## License

MIT. See [LICENSE](LICENSE).
