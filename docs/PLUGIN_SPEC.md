# Ink Source Plugin Guide and Spec v2

This document is both the implementation guide and the normative contract for Ink source plugins.

- `schemaVersion` must be `2`
- Plugins own data collection cadence through `fetchPolicy`
- Print schedules only decide when already-collected items are printed
- `scheduleConfig` and `scheduleConfigSchema` do not exist in v2

> **Trust boundary:** Ink plugins are server-side programs, not browser extensions. Installation and runtime may execute arbitrary code with the API process's privileges. Develop, audit, and install only trusted plugins and dependency trees.

## Build a Plugin

Choose one supported package layout.

Node.js:

```text
example-plugin/
├── ink-plugin.json
├── package.json
├── pnpm-lock.yaml
├── validate.mjs
└── fetch.mjs
```

Python:

```text
example-plugin/
├── ink-plugin.json
├── pyproject.toml
├── uv.lock
├── validate.py
└── fetch.py
```

Development sequence:

1. Define configuration and entrypoints in `ink-plugin.json`.
2. Implement `validate` to reject unusable binding configuration.
3. Implement `fetch` to return stable item identifiers and semantic blocks.
4. Test both entrypoints locally with JSON stdin fixtures.
5. Package the directory as a ZIP, or push it to an allowlisted HTTPS Git host.
6. Install it from **Settings → Plugins**, configure the binding, validate it, and perform a manual fetch.
7. Create a print schedule for the connected binding.

The fixture at [`server/testdata/plugins/python-hello-plugin`](../server/testdata/plugins/python-hello-plugin) is a minimal working Python example used by the server's real subprocess tests.

## Mental Model

Ink now splits plugin work into two independent loops:

1. Binding fetch loop
   - Every enabled binding is fetched automatically on the cadence declared by the plugin manifest.
   - Fetching stores deduplicated items in `plugin_items`.
   - Fetching never creates print jobs.

2. Print schedule loop
   - Each schedule selects already-fetched items for its binding.
   - Delivery tracking is per schedule in `print_schedule_deliveries`.
   - Multiple schedules on the same binding can print the same collected item independently.

## Manifest

### Required fields

```json
{
  "schemaVersion": 2,
  "kind": "source",
  "pluginKey": "example-source",
  "name": "Example Source",
  "version": "1.0.0",
  "description": "Fetches printable content for Ink.",
  "runtime": {
    "type": "node"
  },
  "fetchPolicy": {
    "type": "fixed_interval",
    "minutes": 15
  },
  "entrypoints": {
    "validate": {
      "command": ["node", "validate.mjs"]
    },
    "fetch": {
      "command": ["node", "fetch.mjs"]
    }
  },
  "permissions": {
    "network": {
      "mode": "declared_hosts",
      "hosts": ["api.example.com"]
    },
    "filesystem": {
      "temp": true,
      "cache": false
    },
    "installScripts": false
  },
  "workspaceConfigSchema": [
    {
      "key": "feedUrl",
      "label": "Feed URL",
      "type": "url",
      "required": true
    }
  ]
}
```

### Field rules

- `schemaVersion`: must be `2`
- `kind`: must be `source`
- `pluginKey`: lowercase letters, digits, and dashes
- `runtime.type`: `node` or `python`
- `fetchPolicy.type`: must be `fixed_interval`
- `fetchPolicy.minutes`: positive integer
- `permissions`: optional capability declaration for administrator review
- `workspaceConfigSchema`: binding-level config only

### Supported config field types

- `text`
- `textarea`
- `url`
- `number`
- `select`
- `checkbox`
- `secret`

`secret` fields are allowed only in `workspaceConfigSchema`. Their values are encrypted in binding storage and are passed to the plugin through the `secrets` object.

Configuration keys not declared by the manifest are rejected. Values submitted through the `secrets` object must correspond to fields explicitly declared with `type: "secret"`.

## Entrypoints

Entrypoints are executed as local subprocesses from the installed plugin directory. They receive JSON on stdin, must write one JSON object to stdout, and should reserve stderr for diagnostics.

Ink executes entrypoints with a constrained environment by default. Server process environment variables are not inherited wholesale; only a minimal runtime environment and operator-configured allowlisted variables are exposed. Each invocation receives an isolated temporary home/cache directory, and stdout/stderr are capped by server configuration.

Python plugins that rely on `pyproject.toml` dependencies should prefer `uv run python ...` commands:

```json
{
  "entrypoints": {
    "validate": {
      "command": ["uv", "run", "python", "validate.py"]
    },
    "fetch": {
      "command": ["uv", "run", "python", "fetch.py"]
    }
  }
}
```

### `validate`

`validate` checks whether a workspace binding can be enabled.

Input:

```json
{
  "workspaceConfig": {
    "feedUrl": "https://example.com/feed"
  },
  "secrets": {
    "apiToken": "secret"
  }
}
```

Output:

```json
{
  "valid": true,
  "errors": []
}
```

Validation failures should use field-level errors when possible:

```json
{
  "valid": false,
  "errors": [
    {
      "field": "feedUrl",
      "message": "Feed URL is required"
    }
  ]
}
```

### `fetch`

`fetch` collects new content for one binding.

Input:

```json
{
  "workspaceConfig": {
    "feedUrl": "https://example.com/feed"
  },
  "secrets": {
    "apiToken": "secret"
  },
  "cursor": "opaque-cursor-from-last-run",
  "trigger": {
    "kind": "automatic",
    "scheduledFor": "2026-04-20T12:00:00Z",
    "triggeredAt": "2026-04-20T12:00:03Z",
    "timezone": "UTC"
  }
}
```

Notes:

- `cursor` is optional and is whatever the plugin returned last time
- `trigger.kind` is `automatic` for background fetches and `manual` for `POST /api/v1/plugins/{installationID}/run`
- `scheduleConfig` is never provided in v2

Output:

```json
{
  "items": [
    {
      "externalId": "item-123",
      "title": "Daily Digest",
      "sourceLabel": "Example Source",
      "publishedAt": "2026-04-20T11:55:00Z",
      "blocks": [
        { "type": "heading", "level": 1, "text": "Daily Digest" },
        { "type": "paragraph", "text": "Hello from Ink." }
      ]
    }
  ],
  "cursor": "next-opaque-cursor"
}
```

`externalId` is the dedupe key together with `plugin_binding_id`.

## Content Blocks

Supported block types:

- `heading`
- `paragraph`
- `image`
- `link`
- `divider`
- `list` (`style`: `bullet` or `ordered`, with `items`)
- `quote`

Block shapes:

```json
[
  { "type": "heading", "level": 1, "text": "Daily Digest" },
  { "type": "paragraph", "text": "A printable paragraph." },
  { "type": "image", "url": "https://example.com/image.png", "alt": "Cover" },
  { "type": "link", "url": "https://example.com/article", "text": "Read more" },
  { "type": "divider" },
  { "type": "list", "style": "bullet", "items": ["First", "Second"] },
  { "type": "quote", "text": "A short quotation." }
]
```

Rules:

- `heading.level` must be `1`, `2`, or `3`.
- `heading`, `paragraph`, and `quote` require non-empty text.
- `image` and `link` URLs must use HTTP or HTTPS.
- `list.style` may be `bullet` or `ordered`; `items` must be non-empty strings.
- Every item must contain at least one valid block.

The current thermal renderer converts blocks to text before generating the final PNG. `image` blocks therefore print their alternative text and URL; Ink does not yet download and embed remote image pixels. Links print their label and URL. This behavior is identical in preview and physical printing.

Plugins should emit simple, sanitized printable content. Ink renders semantic blocks into the target device's image format; plugins do not calculate paper width or emit printer commands. Unsupported rich source content should be summarized or converted to the closest supported semantic block.

Recommended local checks for plugin authors:

- run the manifest validation and plugin tests provided by this monorepo (and use future in-repo plugin tooling when available)
- execute `validate` with a fixture payload and assert `valid: true`
- execute `fetch` with a fixture payload and validate every emitted item and block

Example shell checks:

```bash
printf '%s' '{"workspaceConfig":{"feedUrl":"https://example.com/feed"},"secrets":{}}' \
  | node validate.mjs

printf '%s' '{"workspaceConfig":{"feedUrl":"https://example.com/feed"},"secrets":{},"cursor":null,"trigger":{"kind":"manual","triggeredAt":"2026-01-01T00:00:00Z","timezone":"UTC"}}' \
  | node fetch.mjs
```

Use `uv run python ...` instead for Python entrypoints. Entrypoints must write only result JSON to stdout; use stderr for diagnostics and never log secrets.

## Install and Update

Administrators can install a plugin in either form:

- upload a ZIP containing the package layout above;
- install from an HTTPS Git URL whose host is in `PLUGIN_GIT_ALLOWED_HOSTS`, with an optional branch/tag and repository subdirectory.

Reinstalling the same `pluginKey` updates the existing installation record. Ink publishes the new version only after dependency installation succeeds and removes the previous published directory after the database update succeeds. Existing user bindings remain associated with the installation ID, so plugin authors should preserve compatible configuration keys or document migration steps.

Ink also applies server-side fetch output limits before ingestion:

- maximum items per fetch
- maximum blocks per item
- maximum text bytes
- maximum URL bytes
- maximum stdout/stderr bytes per entrypoint

Operators can tune these with the `PLUGIN_OUTPUT_MAX_BYTES`, `PLUGIN_FETCH_MAX_ITEMS`, `PLUGIN_FETCH_MAX_BLOCKS_PER_ITEM`, `PLUGIN_FETCH_MAX_TEXT_BYTES`, and `PLUGIN_FETCH_MAX_URL_BYTES` environment variables.

## Permissions

`permissions` is a capability declaration. It lets plugin authors state what the plugin expects and lets administrators review risk before enabling it. Permission badges and manifest values are declarations, not sandbox grants: the current local runner does not enforce network or filesystem restrictions from them. A stricter runner must enforce capabilities before Ink can expose untrusted public plugin installation.

Network modes:

- `none`: plugin should not require outbound network access
- `declared_hosts`: plugin expects outbound access to the listed hostnames
- `all`: plugin expects general outbound network access

Example:

```json
{
  "permissions": {
    "network": {
      "mode": "declared_hosts",
      "hosts": ["api.github.com", "*.example.org"]
    },
    "filesystem": {
      "temp": true,
      "cache": true
    },
    "installScripts": false
  }
}
```

Filesystem flags:

- `temp`: plugin expects invocation-scoped temporary storage
- `cache`: plugin expects persistent cache storage; the local trusted runner currently gives invocation-scoped cache only

For Node plugins, omitted or `false` `installScripts` makes Ink run `pnpm install --frozen-lockfile --ignore-scripts`. Setting it to `true` runs `pnpm install --frozen-lockfile`, allowing package lifecycle scripts. This reduces accidental Node install-time execution by default, but it is not a sandbox and does not make plugin code or dependencies trustworthy.

Python plugins always install with `uv sync --frozen`. Python package build hooks may execute during that operation regardless of `installScripts`; the flag does not claim to control Python builds. Operators must therefore trust Python plugins and their full dependency chains.

## Binding Fetch State

Each binding tracks fetch execution separately from print schedules.

- `next_fetch_at`
- `last_fetch_at`
- `fetch_lease_until`
- `last_fetch_error`

Behavior:

- Enabling a binding schedules immediate fetch
- Disabling a binding clears future automatic fetches
- Successful fetch updates cursor and `next_fetch_at`
- Failed fetch updates `last_fetch_error` and schedules the next retry

## Collected Items And Deliveries

Fetched items are stored once per binding:

- `plugin_items` is deduplicated on `(plugin_binding_id, external_id)`

Printing is tracked per schedule:

- `print_schedule_deliveries` is deduplicated on `(print_schedule_id, plugin_item_id)`

Delivery behavior:

- Schedule ticks pick the oldest fetched items for the binding that do not yet have a delivery row for that schedule
- Failed print attempts are retried through the delivery row
- A successful delivery for schedule A does not block schedule B

## Print Schedules

Print schedules are platform-owned. Plugins do not define schedule-specific fields.

Schedule payloads use:

```json
{
  "title": "Morning Digest",
  "pluginInstallationId": "plugin-installation-1",
  "frequencyType": "daily",
  "timezone": "Asia/Shanghai",
  "hour": 9,
  "minute": 30,
  "weekdays": [],
  "printPolicy": {
    "batchSize": 1
  },
  "deviceId": "device-1",
  "enabled": true
}
```

`printPolicy` currently supports:

- `batchSize`: positive integer, defaults to `1`

Delivery order is fixed to oldest-first.

## Manual Operations

- `POST /api/v1/plugins/{installationID}/run`
  - Manual fetch only
  - Fetches and ingests items
  - Does not print

- `POST /api/v1/print-preview`
  - Authenticated preview operation; accepts `title` and either plain `content`
    or plugin `blocks`
  - Returns a base64-encoded PNG rendered with the same 384px image pipeline
    used for physical printing
  - Never creates a print job or contacts a printer

- `POST /api/v1/print-schedules/{scheduleID}/run`
  - Manual print tick only
  - Prints already-collected items for that schedule
  - Does not fetch

## Security Model

Ink treats installed plugins and their dependencies as trusted server-side code. Installation, package build hooks, and runtime entrypoints may execute code with the server process's privileges. Admin-only installation limits who can initiate installation, but it does not make untrusted code safe.

Plugin entrypoints run as local subprocesses with a constrained environment, isolated temporary directories, execution timeouts, and output limits. These are defense-in-depth controls, not a security sandbox. Permission declarations are primarily review metadata, except that Node `installScripts` selects whether pnpm lifecycle scripts are ignored during installation. Operators must install only from trusted repositories or uploads, audit dependency chains, keep the Git host allowlist narrow, and avoid any public marketplace until installation and runtime have an appropriate sandbox with enforced capabilities. Python's build-hook limitation makes the trusted-only rule mandatory even at install time.

Plugin authors should:

- minimize dependencies and commit lockfiles;
- avoid install scripts unless strictly necessary;
- bound network response sizes and timeouts inside the plugin;
- treat configuration and remote content as untrusted input;
- never print secrets to stdout, stderr, item content, or error messages;
- use stable, source-derived `externalId` values rather than fetch timestamps.

## Example Node Fetch Entrypoint

```js
import process from "node:process";

async function readJSON() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const raw = Buffer.concat(chunks).toString("utf8").trim();
  return raw ? JSON.parse(raw) : {};
}

const payload = await readJSON();
const sourceName = String(payload.workspaceConfig?.sourceName ?? "Example Source").trim();

process.stdout.write(
  JSON.stringify({
    items: [
      {
        externalId: `example-${payload.trigger?.triggeredAt ?? "default"}`,
        title: `${sourceName} Digest`,
        sourceLabel: sourceName,
        blocks: [
          { type: "heading", level: 1, text: `${sourceName} Digest` },
          { type: "paragraph", text: "Hello from Ink." },
        ],
      },
    ],
    cursor: payload.trigger?.triggeredAt ?? null,
  }),
);
```

## Compatibility

v2 is a breaking contract.

- `schemaVersion: 1` is obsolete
- `scheduleConfigSchema` is obsolete
- `fetch(scheduleConfig)` is obsolete
- New plugins must target `schemaVersion: 2`
