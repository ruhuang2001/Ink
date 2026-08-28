# Security Policy

## Supported versions

Ink is still in early development. Security fixes are applied to:

| Version                     | Supported |
| --------------------------- | --------- |
| `main`                      | yes       |
| latest `0.x` release        | yes       |
| older pre-release snapshots | no        |

## Reporting a vulnerability

Please use GitHub private vulnerability reporting for this repository:

- https://github.com/ruhuang2001/Ink/security/advisories/new

1. Go to the repository Security tab.
2. Choose **Report a vulnerability**.
3. Include reproduction steps, impact, affected endpoints or files, and any proposed mitigation.

Do not open a public issue for undisclosed vulnerabilities, credentials, token leaks, or exploit details.

## Plugin threat boundary

Ink plugins are trusted server-side code. Plugin installation, dependency resolution and build hooks, and runtime entrypoints can execute code with the server process's privileges. Restricting installation endpoints to administrators controls who may install a plugin; it does not make an untrusted plugin or dependency safe.

Install plugins only from sources whose code and dependency chain you trust. Ink must not offer an untrusted public plugin marketplace until plugin installation and execution have an appropriate sandbox and enforceable capability model.

## Browser session boundary

The current web client stores access and refresh session tokens in browser Web Storage. This keeps the early self-hosted flow simple, but JavaScript running through an XSS vulnerability could read those credentials. Deploy Ink only over HTTPS, use a restrictive Content Security Policy at the reverse proxy, avoid injecting untrusted HTML, and keep frontend dependencies patched.

Moving refresh credentials to `HttpOnly`, `Secure`, and appropriately `SameSite` cookies with an explicit CSRF strategy is planned but not implemented in the current release.

## Dependency vulnerability handling

Pull requests run npm dependency review, `pnpm audit`, Go reachability analysis through `govulncheck`, and CodeQL analysis for Go and JavaScript/TypeScript. Vulnerability exceptions must be narrow, evidence-based, documented beside the affected manifest, and have an expiry date.

`server/osv-scanner.toml` currently records one temporary exception for `GO-2026-5932`: `openpgp` is present transitively through `go-git`, but Ink does not import that package and `govulncheck` reports no reachable package or symbol. The exception expires on 2026-11-09 and must not be renewed without another applicability review.

## What to include

- clear reproduction steps
- affected version, commit, or environment
- impact and attack preconditions
- logs, request IDs, screenshots, or proof of concept if relevant

## Response expectations

- We aim to acknowledge valid vulnerability reports within 3 business days.
- We aim to provide an initial triage update within 7 calendar days.
- We may ask follow-up questions or request a minimal reproduction.
- We will coordinate disclosure timing after a fix or mitigation is ready.
- When coordinated disclosure is needed, we target public disclosure within 90 days after confirmation unless a longer mitigation window is required.
