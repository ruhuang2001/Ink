# CI performance

Reproduce timing collection with `python3 scripts/measure-actions.py <output-directory>`.
The initial evidence covers the latest 50 workflow runs as of 2026-11-03, including
Dependabot, cancelled and failed runs. Per-job raw step timestamps and phase totals
are in `before/`. Successful-job medians exclude failures/cancellations. Queue time
includes dependency waits; composite action internals cannot be precisely separated
without parsing their logs. Setup includes teardown and runner overhead.

## Safety and scope

All tests, coverage thresholds, audits, lint, builds, binary vulnerability scans,
CodeQL languages and release recovery checks remain. All runners were already
Ubuntu. Short test suites are not sharded: their setup overhead exceeds useful test
time. Independent release suites now run on separate machines, checking the exact
candidate SHA, with every suite required by the publishing job.

Only PR runs share a cancellable concurrency group. Other events use run IDs so
neither active nor pending main commits are superseded. Main runs all checks for
non-documentation changes. PR and main workflows ignore root Markdown and `docs/**`;
web Markdown still runs formatting checks. Scheduled security checks remain active.

On 2026-11-03 the main branch protection API had no required status checks and the
active ruleset contained deletion, non-fast-forward and Copilot review rules only.
No protection settings were changed. **Before requiring any path-filtered workflow's
check, introduce an always-triggered change-detection/aggregate gate and require
that gate instead.** Otherwise docs-only PRs can remain pending indefinitely.

## Caches and tools

- pnpm/action-setup installs the lockfile's package-manager version before setup-node
  restores its built-in pnpm store cache. No node_modules or dist cache.
- setup-go retains its built-in cache. Smoke adds its script to the dependency hash
  to keep race-build entries separate from the ordinary Go job's immutable cache.
- golangci-lint uses the action's prebuilt v2.11.4 binary, including releases.
- Upstream golang/vuln releases have no binary assets. govulncheck v1.7.0 is compiled
  only on a miss and its small binary is cached by OS/architecture/Go module version;
  vulnerability data and scan results are never cached.
- No Docker-image or Rust cache is added. Inspect usage with `gh cache list` before
  expanding caches; the repository budget is 10 GB.

## Local repeated builds

`local-builds.json` records two consecutive `pnpm --dir web build` and
`go -C server build ./...` invocations. Go's second build was 1.88 s (first 3.89 s).
Vite deliberately rebundles each invocation (~3 s); TypeScript already uses
incremental build metadata. There is no tracked application Dockerfile. Avoid an
unsafe timestamp-only "skip build" cache merely to make this number approach zero.

## E2E evidence

API smoke still executes race-enabled PostgreSQL/migration checks and the complete
HTTP/persistence flow. CI uploads its log even on failure, with a seven-day retention.
Reproduce with `make smoke-api` against an isolated disposable database.
