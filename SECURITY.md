# Security Policy

## Supported versions

For the 1.0 line, security fixes apply to the latest released 1.x version and `main`. Before a stable 1.0 is published, release candidates remain pre-release software and do not constitute a stable support promise.

| Version                                  | Supported                                   |
| ---------------------------------------- | ------------------------------------------- |
| `main`                                   | yes                                         |
| latest stable `1.x` release              | yes, after publication                      |
| older releases and pre-release snapshots | no; upgrade to the latest supported release |

## Reporting a vulnerability

Please use GitHub private vulnerability reporting for this repository:

- https://github.com/ruhuang2001/Ink/security/advisories/new

1. Go to the repository Security tab.
2. Choose **Report a vulnerability**.
3. Include reproduction steps, impact, affected endpoints or files, and any proposed mitigation.

Do not open a public issue for undisclosed vulnerabilities, credentials, token leaks, or exploit details.

## Legacy plugin data

Ink 1.0 removes plugin installation, execution and scheduling from the running application while preserving historical database rows and files. Archival plugin files and dependencies can contain executable code; treat them as private backup material and do not execute them during ordinary recovery. Any future plugin feature needs a separately reviewed security and compatibility contract.

## Browser session boundary

The current web client stores access and refresh session tokens in browser Web Storage. This keeps the early self-hosted flow simple, but JavaScript running through an XSS vulnerability could read those credentials. Deploy Ink only over HTTPS, use a restrictive Content Security Policy at the reverse proxy, avoid injecting untrusted HTML, and keep frontend dependencies patched.

Moving refresh credentials to `HttpOnly`, `Secure`, and appropriately `SameSite` cookies with an explicit CSRF strategy is planned but not implemented in the current release.

## Dependency vulnerability handling

Pull requests run npm dependency review, `pnpm audit`, Go reachability analysis through `govulncheck`, and CodeQL analysis for Go and JavaScript/TypeScript. Vulnerability exceptions must be narrow, evidence-based, documented beside the affected manifest, and have an expiry date.

The removed plugin runtime no longer depends on `go-git` or its OpenPGP dependencies. The former `GO-2026-5932` exception has been removed; there are no active vulnerability exceptions.

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
