# Core Maintenance Scope

Updated: 2026-10-14. Version 1.0 covers the core printing, account, device, draft/conversation, and AI features for a single self-hosted instance. Plugins and scheduled plugin jobs have been removed from the running application. Legacy data and installation files are retained for migration, backup, and historical recovery.

## Core Workflow

Log in → bind a device → compose content → preview → confirm and submit a print job → view its status and history.

- Data must remain isolated across logins, logouts, expired sessions, and account switches.
- Failures to save drafts or preferences must be clearly reported; users must not be led to believe unsaved data has synced.
- Manual printing and device management must work independently of AI features.
- Preview requires a valid session, creates no print job, and does not call the physical printing service.
- Print submission failures must retain an explainable status; accepted jobs are updated through background status synchronization.
- Preserve existing database migrations, user data, verified printing optimizations, and the author link in the header.

## Deferred Expansion

The following expansions are not required for the 1.0 release:

- A plugin marketplace or public installation of untrusted plugins.
- A new plugin protocol, standalone SDK, or complex installation approval and version-management system.
- A standalone worker, general-purpose event bus, multiple runtimes, or distributed scheduling.
- Additional AI providers, conversation capabilities, or content sources.

The local `todo.md` contains an early plugin redesign proposal and is not tracked by Git. It is retained as historical reference, not as a release requirement. Do not extend the architecture based on that list until requirements are clarified.

## Next Maintenance Items

Prioritize data correctness and keep each change small and verifiable:

- [x] Workspace saves use atomic revision comparisons. Stale revisions return a conflict, preserve the current page's draft, and pause autosaving until the user confirms a reload. See the [Workspace API](WORKSPACE_API.md).
- [x] Device removal now marks a device offline, preserves print and delivery history, and disables associated schedules. Rebinding reuses the existing identity; version 1.0 does not run old schedules.
- [x] Preview and printing share content and output-size validation. Oversized requests return 413 to prevent oversized PNG generation or silent truncation.
- [ ] Verify binding, manual printing, offline recovery, and completion status on a physical Memobird.
- [x] Verify backup restoration in an isolated PostgreSQL test database, with all 13 tables matching row by row. See the [reproduction procedure](BACKUP_RESTORE.md).
- [x] Verify encrypted configuration and plugin-directory restoration using random test keys and synthetic data. Invalid or missing keys and corrupted nonces fail explicitly. The current restore check compares only the bytes of retained files; it does not run plugins.
- [ ] Verify restoration of real keys, runtime versions, and configuration in an isolated copy of an actual deployment. Legacy plugin files are retained but are not run by the 1.0 API.

Release candidates must pass frontend and backend quality checks, API smoke tests, security scans, and optional monitoring checks against a fixed commit. An RC or draft may be shared before physical-device acceptance is complete, but CI results or manual declarations in a release form must not be treated as hardware verification.

Frontend store decomposition can proceed incrementally when related maintenance work touches those areas. Address data-correctness boundaries first; do not rewrite everything at once.

## Verification

```bash
make check-web
make check-api
make smoke-api
```

By default, `make smoke-api` uses a temporary PostgreSQL container and temporary account credentials. It cleans them up after the test, does not overwrite the local development database, `.env`, or administrator credential file, and does not call the physical printing service.

Run `make check-local-ci` before committing or pushing, and do not bypass repository hooks. Passing unit and smoke tests does not verify a physical device or production deployment.
