# vendor/

`skillissues-markdown.js` is a generated, checked-in bundle of
`packages/markdown` (markdown-it + DOMPurify) exposed as the global
`window.SkillIssuesMarkdown`. Desktop has no bundler, so it can't `import` the
shared package directly the way the web client does — this is the one place
a pre-built artifact is committed instead of generated at build time.

Regenerate it after changing `packages/markdown/src/index.ts`:

```bash
cd packages/markdown
npm ci
npm run build:global
```

Never edit `skillissues-markdown.js` by hand.
