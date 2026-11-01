# Shared packages (`@skillissues/*`)

npm workspace members, imported by name (a real `node_modules` symlink), never by a relative path
reaching outside the importing project. Install with `npm ci` at the repo root.

```bash
cd packages/theme && npm run typecheck && npm test   # theme has the only scripts
cd web && npm run typecheck                          # client and markdown are checked through their users
cd desktop && npm run check
```

## `client`: the typed API client

- `ApiError` carries `status` and `problems`; **status 0 means the server never answered.**
- Pluggable `Transport` (`fetchTransport` in the browser; the desktop swaps in its IPC proxy).
- **Cursor paging always sends `?cursor=`, even empty:** that is what selects the For you feed.
- A mutation sends a JSON body even when it has nothing to say (`body: {}` on a DELETE): the API refuses
  cookie-authenticated mutations without `Content-Type: application/json`.

## `theme`: user themes on the client

- Mirrors `backend/internal/theme`: the same TOML schema and stylesheet allowlist, **run on the same
  fixtures** (`fixtures/`), with problems worded identically (`fixtures/problems`). CI runs both sides.
  **Change both sides in the same PR.**
- `compileTheme`/`applyTheme` re-validate a theme and apply it through constructed stylesheets scoped to
  the app's root. `@property` is allowed only for `--theme-*` names and is lifted out of `@scope`.
- Light/dark halves fold per layer in `src/modes.ts`. Tokens added after the first release are optional,
  and Default doesn't set them, so presets inherit nothing wrong.

## `markdown`: sanitised rendering

- `markdown-it` + DOMPurify. **Issue bodies are attacker-controlled: never hand-roll rendering in either
  client**, and never use `dangerouslySetInnerHTML` outside this package.
- The only `class` that survives is `language-…` on `<code>` (a DOMPurify hook). `highlightCode` colours
  those blocks afterwards with highlight.js (a lazily loaded chunk, a fixed set of languages), reading
  their text, never HTML.
