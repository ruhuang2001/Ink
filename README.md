<p align="center">
  <img src="assets/logo.png" alt="Ink" width="180" />
</p>

# Ink

Ink is a self-hosted workspace for writing content, using AI assistance, and printing it on Memobird thermal printers. It combines a Vue web console, a Go API, PostgreSQL persistence, and a server-side PNG print pipeline.

The 1.0 candidate scope is single-instance self-hosting: accounts, devices, drafts and conversations, AI configuration and replies, PNG preview, manual printing, and print status/history. Plugin installation, fetching and plugin schedules are removed from the running application. Existing plugin tables, stored state and installation files are preserved for migration and archival recovery.

Current development focuses on the basic printing flow: sign in, bind a device,
write and preview content, submit a job, and check its status and history.
See [Maintenance scope](docs/MAINTENANCE.md) for the remaining real-device acceptance checks. A passing build is not evidence of a production or physical-printer deployment.

## What it does

- Manage Memobird devices and manual print jobs.
- Save drafts, conversations and preferences with revision conflict protection.
- Configure an AI provider and generate replies.
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
- [Maintenance scope and stability checklist](docs/MAINTENANCE.md)
- [Workspace synchronization API](docs/WORKSPACE_API.md)
- [Database backup and restore verification](docs/BACKUP_RESTORE.md)
- [Historical plugin protocol](docs/PLUGIN_SPEC.md) (legacy reference; unavailable in 1.0)
- [Server reference](server/README.md)
- [Frontend reference](web/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## Security limits

The 1.0 API does not install or execute plugins. Retained plugin backups may still contain executable code; keep them private and do not run unreviewed files during recovery.

The browser currently stores access and refresh session tokens in Web Storage. Deploy Ink behind HTTPS, use a restrictive Content Security Policy, and treat browser XSS as a session-compromise risk until cookie-based refresh sessions are implemented.

## Quality checks

```bash
make check-web
make check-api
make check-local-ci
```

`make check-local-ci` runs the web and server CI jobs locally with `act`, plus the API smoke test against an isolated PostgreSQL container.

Version tags run quality checks against the exact candidate commit and create a draft release. Publish an RC or stable release through the explicit release workflow only after reviewing its scope and evidence. See [Release procedure](docs/SELF_HOSTING.md#release-candidates-and-version-policy).

## License

MIT. See [LICENSE](LICENSE).
