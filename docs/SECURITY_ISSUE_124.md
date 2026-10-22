# Issue 124: Go security rebuild verification

Verified locally on 2026-10-22. Main baseline: `04ace62`; release/1.0 baseline: `ca241a2`.

Both trees now require Go 1.26.10. Release/1.0 also upgrades golang.org/x/net from v0.58.0 to v0.60.0; main already uses v0.60.0. Existing CI vulnerability scans now print compiler and binary metadata. Main candidate release checks record the same metadata.

## Local verification

Both trees passed `make check-api` and `make smoke-api` with isolated PostgreSQL databases. Main smoke verified login, workspace persistence and conflicts, PNG preview, manual printing and paginated print/status/detail reads. Release smoke additionally verified its retained plugin lifecycle and schedule delivery, using `SMOKE_PORT=18081` to run concurrently.

All four command binaries (`api`, `migrate`, `seed`, `smoke-fixture`) were rebuilt for darwin/arm64. `go version -m` reports go1.26.10 for each. Govulncheck v1.7.0 passed all eight binary scans and both source scans: zero reachable vulnerabilities and zero vulnerable imported packages. Module-only GO-2026-5932 remains in golang.org/x/crypto/openpgp, which neither tree imports. This is not a claim of zero module advisories.

Local evidence: `output/issue-124/{main,release}/checks.log`, `toolchain.txt`, `modules.txt`, `govulncheck-source.txt`, and `bin/*.metadata.txt` / `bin/*.govulncheck.txt`.

## Reproduce

From either tree, run `go -C server version`, `make check-api`, and `make smoke-api`, retaining stdout/stderr and exit status. Install `golang.org/x/vuln/cmd/govulncheck@v1.7.0` into a temporary GOBIN with `GOTOOLCHAIN=go1.26.10`. In `server`, run `govulncheck -show verbose ./...`; build each package returned by `go list ./cmd/...` into the temporary directory, then run `go version -m` and `govulncheck -mode=binary` against each executable. The smoke script cleans up its temporary credentials and database.

## Remaining boundaries

These are local source changes and local rebuild results, not remote CI results or a published release. The repository has no application Dockerfile or application container build workflow; Compose runs PostgreSQL and self-hosting builds executables from source. No GitHub releases were listed at verification time. Deployed binaries and physical printers were not inspected or updated. Operators must rebuild/restart deployed services and verify each installed binary with `go version -m`. Keep the issue open until integration and applicable deployment verification are recorded.
