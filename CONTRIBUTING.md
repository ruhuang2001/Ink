# Contributing to Ink

Thanks for your interest in contributing to Ink.

The 1.0 candidate focuses on single-instance self-hosting of accounts, devices, drafts/conversations, AI assistance and manual printing. Please keep contributions focused, easy to review, and aligned with that scope. Plugins and plugin schedules are removed; retained legacy records and files must remain recoverable. Structured content blocks remain supported by the core renderer.

## Ways to contribute

You can help by:

- Reporting bugs
- Improving documentation
- Fixing frontend issues
- Adding tests
- Refining UX and developer experience

## Repository overview

- `web/` contains the Vue 3 + TypeScript frontend built with Vite.
- `server/` contains the Go API, PostgreSQL repositories, migrations, and print-status synchronization.
- `docs/` contains self-hosting, architecture, API and archived plugin documentation.
- `Makefile` provides the quickest way to run common local commands.

## Before you start

Please check existing issues and pull requests before starting work.

You can open a pull request directly for:

- Bug fixes
- Documentation improvements
- Small refactors
- Focused test additions

Please open an issue first before implementing:

- New features
- Large refactors
- Behavior changes
- Anything that changes the project direction or public surface area

This helps avoid duplicate work and keeps the project direction consistent.

## Local setup

### Prerequisites

- Node.js 22
- pnpm 10
- Go 1.26.10 or newer
- Docker with PostgreSQL 16 support
- Git

Fork the repository, clone your fork, and create a branch from `main`.

Example:

```bash
git checkout main
git pull origin main
git checkout -b feat/your-change
```

## Development commands

Install dependencies:

```bash
pnpm install
cd web
pnpm install
```

The root install sets up the repository Git hooks. The `web/` install provides the frontend toolchain used by the pre-commit checks.

Start the full development stack in separate terminals:

```bash
make dev-api
make dev-web
```

Run the relevant quality checks before opening a pull request:

```bash
make check-web
make check-api
```

Useful frontend commands:

```bash
cd web
pnpm dev
pnpm lint
pnpm format:check
pnpm test:run
pnpm build
```

### Environment variables

Do not commit secrets, tokens, or device credentials.

## Pull request expectations

Keep pull requests focused on one change. Smaller pull requests are easier to review and merge.

When opening a pull request:

- Explain what changed
- Explain why the change is needed
- Link the related issue when applicable
- Include the exact test commands you ran
- Add screenshots for UI changes
- Update docs or tests when your change affects behavior

If your pull request changes visible frontend behavior, include before/after screenshots when possible.

## Reporting bugs

When reporting a bug, include:

- Clear reproduction steps
- Expected behavior
- Actual behavior
- Relevant logs, screenshots, or request details
- Environment details if they matter

One issue should describe one problem.

## Commit message format

Commit messages must follow this format:

```text
type(scope): summary
```

### Allowed types

Only these commit types are supported:

- `feat`
- `fix`
- `docs`
- `refactor`
- `chore`
- `perf`

### Scope

The `scope` is required.

Use a short scope that matches the area you changed. Common examples include:

- `web`
- `readme`
- `docs`
- `ci`
- `license`

### Summary rules

The summary must:

- Use the imperative mood
- Start with a lowercase letter
- Not end with a period

### Valid examples

```text
feat(web): add printer status cards
docs(readme): link contributing guide
refactor(web): simplify dashboard layout state
chore(ci): run web quality checks on pull requests
perf(web): reduce dashboard render work
```

### Invalid examples

```text
feature(web): add printer status cards
feat: add printer status cards
feat(web): Added printer status cards.
```

Please keep commits readable and consistent with the existing project history.

## Testing before submission

Before opening a pull request, run the checks relevant to your change.

Run `make check-local-ci` before a commit or push; Docker and `act` are required. It runs the web, server, and Go lint jobs used in CI, followed by the isolated API smoke test. Do not bypass the repository hooks. For database/recovery changes, also run `bash scripts/check-backup-restore.sh`. Release candidates run additional checks against a fixed commit as described in the [release procedure](docs/SELF_HOSTING.md#release-candidates-and-version-policy).

## License

By contributing to Ink, you agree that your contributions will be licensed under the [MIT License](LICENSE).
