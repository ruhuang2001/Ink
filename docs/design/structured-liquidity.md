# Structured Liquidity for Ink

The default frontend theme uses the visual language from [structured.glass](https://structured.glass/): flat paper surfaces, square corners, defined borders, offset shadows, a warm orange accent (`#E5AA70`), and monospace labels. Reference tokens were checked against the site's `llms.txt` and `r/structured-liquidity.json` on 2026-10-12.

This is a Vue adaptation, not an installation of its React/shadcn component registry. No runtime dependency or external font request was added. `web/src/structured.css` holds the shared tokens and component styling; `InkIcon.vue` supplies local SVG icons. Existing dialog behavior and the prior print-preview work are retained. Product copy stays identical to the original design. No slogans or descriptive copy are added. Attribution is kept in this document.

## Preview

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web dev --host 127.0.0.1 --port 5188 --strictPort
```

Structured Liquidity is the default for browsers without an explicit choice; saved original-design selections are respected. Switch designs in Settings → Appearance; the selection is stored on this browser. For a temporary preview, visit `/conversations?design=structured`, `/prints?design=structured`, or `/login?design=structured`. Anonymous pages use Ink's existing demo data. Account settings, AI generation, and real printer operations retain their existing authentication and backend requirements.

Verify with `pnpm --dir web check`. Inspect desktop and narrow layouts, select conversation messages, switch conversations, open a print preview, and check the login form. Structured Liquidity is light-only. The original design retains light/dark/system controls and its saved preference; switching designs never overwrites that preference. Reduced-motion support is retained.

## Integration

Visual design is a browser-local Pinia preference (`ink.design.v1`) shared by the shell and login page. It is stored separately from the existing synchronized light/dark/system preference and requires no API schema migration. Invalid preview values fall back to the saved design. Valid preview query changes also apply during client-side navigation without persisting. Selecting a design clears the preview query through Vue Router, even if storage is blocked. Both designs retain `/icon.png` for desktop and mobile branding. Theme-color metadata follows the selected design and color mode.

Dialogs teleport to the shell so animated page sections do not constrain their fixed positioning, while inheriting the selected design, falling back to body when there is no shell. The existing print-preview changes are included: a 384px paper canvas scales down on narrow screens, keeps the original explanatory copy, and displays printer-sized text while its image is pending or fails. Mobile action labels wrap in both languages.

## Runtime screenshots

Captured against the local Vite app and development API. Desktop viewport: 1440×1080; mobile viewport: 390×844. These use the existing local development data. Mobile images capture the full page, so the fixed bottom navigation appears at its viewport position. No physical print was sent.

| Page / state | Screenshot |
| --- | --- |
| Theme settings | [Structured settings](screenshots/structured-settings.png) |
| Conversations | [Structured conversations](screenshots/structured-conversations.png) |
| Prints | [Structured prints](screenshots/structured-prints.png) |
| Print preview | [Structured preview](screenshots/structured-preview.png) |
| Devices | [Structured devices](screenshots/structured-devices.png) |
| Guide | [Structured guide](screenshots/structured-guide.png) |
| Login | [Structured login](screenshots/structured-login.png) |
| English settings | [English settings](screenshots/structured-english-settings.png) |
| Original settings | [Original settings](screenshots/original-settings.png) |
| Original conversations | [Original conversations](screenshots/original-conversations.png) |
| Mobile settings | [Mobile settings](screenshots/structured-mobile-settings.png) |
| Mobile conversations | [Mobile conversations](screenshots/structured-mobile-conversations.png) |
| Mobile English | [Mobile English](screenshots/structured-mobile-english.png) |
| Mobile prints | [Mobile prints](screenshots/structured-mobile-prints.png) |

Runtime checks: switch both designs from Settings, retain selection after reload and navigation, retain the original logo, toggle light/dark modes and Chinese/English, open and dismiss a rendered print preview, and verify no horizontal overflow at 390px.
