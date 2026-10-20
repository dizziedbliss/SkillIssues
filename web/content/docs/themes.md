---
title: Themes
slug: /docs/themes
section: Guides
summary: How to restyle the signed-in app with a theme file of design tokens and an optional stylesheet, with every key and rule spelled out.
draft: false
updated: 2026-10-20
---

A theme restyles the signed-in app. It has two parts:

1. A **TOML file of design tokens**: colours, fonts, spacing and so on. This is required, but it can be tiny.
2. An optional **stylesheet** of CSS, for changes the tokens can't express.

Themes are saved to your account, so they follow you from one device to another. You can keep them private or make them public and share them by link.

The landing page is never themed. A theme applies only inside the signed-in app.

The signed-in app is being rebuilt, so where you create and apply themes in it may change. The format on this page is what the server and the app both enforce, and it is the same whichever screen you use.

## How a theme is checked

A shared theme is someone else's code running on your page, so nothing is trusted.

- Every value in the TOML file is checked against a fixed list of tables, keys and allowed values. A key that isn't listed is an error, so a typo is reported rather than silently ignored.
- The stylesheet is checked, token by token, against an allowlist.
- The server checks a theme when it's saved. The app checks it again before it applies it.
- The browser's security policy is a second line of defence. It blocks a theme from loading anything from elsewhere even if a rule got through.

When a theme is rejected, you get the complete list of problems in one pass, not just the first one.

## A minimal theme

A theme starts from a base preset and changes only what it names. This is a complete, valid theme:

```toml
[colors]
accent = "#2b6bff"
```

Everything you leave out comes from the base preset.

## Base presets

Set `base` in the `[meta]` table to start from one of these:

| Preset | Description |
|---|---|
| `default` | Paper and ink, one red. This is the default if you set no base. |
| `midnight` | The same page, after dark. |
| `terminal` | Green on black, all mono. |
| `contrast` | Larger type, heavier lines. |
| `catppuccin-mocha` | Catppuccin's dark pastels. |
| `catppuccin-latte` | Catppuccin's light flavour. |
| `dracula` | Dracula's purples and pinks. |
| `nord` | Nord's arctic blues. |
| `gruvbox` | Gruvbox dark, warm and earthy. |
| `solarized-light` | Solarized, light, with its orange as the accent so buttons stay readable. |
| `tokyo-night` | Tokyo Night's blues. |
| `rose-pine` | Rosé Pine's muted dark. |

The palettes come from each project's published colours. The rounded ones also round corners and fade between pages.

## Starting a theme in the app

In the app, **Settings → Themes** lists every built-in with its colours; **Use** applies one. **New theme** starts from the theme you're using, or any built-in, as a full copy: every token written out, so you change only the lines you want. Delete a line and it falls back to the base preset.

## Tables and keys

A theme file may contain only the tables and keys below. Each token becomes a CSS custom property named `--<table>-<key>`, so `[colors] accent` is `--colors-accent` and `[radius] small` is `--radius-small`. The `[meta]` table describes the theme and doesn't become a property.

### `[meta]`

| Key | Type | Allowed values |
|---|---|---|
| `name` | text | Up to 60 bytes. |
| `author` | text | Up to 60 bytes. |
| `version` | text | Up to 20 bytes. |
| `description` | text | Up to 280 bytes. |
| `base` | choice | One of the base presets below: `default`, `midnight`, `terminal`, `contrast`, `catppuccin-mocha`, `catppuccin-latte`, `dracula`, `nord`, `gruvbox`, `solarized-light`, `tokyo-night` or `rose-pine`. |
| `schema` | number | Exactly `1`. This is the file format version. |

Text limits count bytes, so plain English text uses one per character and accented letters or emoji use more. A theme without a `name` is called "Untitled". Text can't contain `<`, `>` or null characters.

### `[colors]`

Every key here takes a colour.

| Key | What it colours |
|---|---|
| `background` | The page background. |
| `surface` | Cards and panels. |
| `surface-raised` | Surfaces that sit above other surfaces. |
| `text` | Body text. |
| `text-muted` | Secondary text. |
| `accent` | The accent colour. |
| `accent-text` | Text drawn on the accent colour. |
| `border` | Borders and dividers. |
| `focus` | The keyboard focus ring. |
| `success` | Success states. |
| `warning` | Warning states. |
| `danger` | Errors and destructive actions. |
| `selection` | Selected text. |

### `[background]`

| Key | Type | Allowed values |
|---|---|---|
| `image` | image | `none`, or a `data:image/...;base64,...` URI of a PNG, JPEG, GIF or WebP image. |
| `pattern` | choice | `none`, `grid`, `dots` or `lines`. |
| `pattern-color` | colour | Any colour. |
| `pattern-size` | length | `4px` to `96px`. |

### `[typography]`

| Key | Type | Allowed values |
|---|---|---|
| `heading-font` | font | A font from the list below. |
| `body-font` | font | A font from the list below. |
| `mono-font` | font | A font from the list below. |
| `base-size` | length | `12px` to `22px`. |
| `scale` | number | `1` to `1.6`. |
| `line-height` | number | `1` to `2`. |
| `heading-weight` | number | `100` to `900`. |
| `body-weight` | number | `100` to `900`. |

Fonts a theme may name: `Instrument Serif`, `JetBrains Mono`, `Krona One`, `system-ui`, `serif`, `sans-serif` and `monospace`. The first three are served by SkillIssues itself, so a theme never makes your browser fetch a font from another site. A font that isn't on the list is refused.

### `[spacing]`

| Key | Type | Allowed values |
|---|---|---|
| `unit` | length | `2px` to `8px`. |
| `density` | choice | `compact`, `comfortable` or `spacious`. |
| `page-gutter` | length | `8px` to `96px`. |
| `max-width` | length | `640px` to `2400px`. |

### `[radius]`

| Key | Type | Allowed values |
|---|---|---|
| `small` | length | `0px` to `48px`. |
| `medium` | length | `0px` to `48px`. |
| `large` | length | `0px` to `48px`. |
| `pill` | length | `0px` to `999px`. |

In the app, `small` rounds tags and chips, `medium` buttons and fields, `large` panels, menus and dialogs, and `pill` the switches.

### `[border]`

| Key | Type | Allowed values |
|---|---|---|
| `width` | length | `0px` to `4px`. |
| `style` | choice | `solid`, `dashed`, `dotted` or `none`. |

### `[shadow]`

`small`, `medium` and `large` each take a shadow. In the app, `small` sits under tags and buttons, `medium` under menus and `large` under dialogs such as search.

### `[motion]`

| Key | Type | Allowed values |
|---|---|---|
| `duration` | duration | A whole number of milliseconds from `0ms` to `2000ms`, such as `160ms`. |
| `easing` | choice | `linear`, `ease`, `ease-in`, `ease-out` or `ease-in-out`. |
| `reduce` | boolean | `true` or `false`. |
| `transition` | choice | How the app moves from one page to the next: `wipe` (the new page sweeps in from the left), `fade`, `slide` or `none`. Every transition stops when your system asks for reduced motion. |

## Value types

| Type | What is accepted |
|---|---|
| colour | A hex colour (`#rgb`, `#rgba`, `#rrggbb` or `#rrggbbaa`). Or `rgb()`, `rgba()`, `hsl()`, `hsla()`, `oklch()` or `oklab()` holding only numbers, `%`, `deg`, spaces, commas, slashes, `+` and `-`. Or one of `transparent`, `currentcolor`, `black` and `white`. |
| length | A number followed by `px`, `rem` or `em`, such as `15px` or `1.5rem`. The range limits above are in pixels. `rem` and `em` count as 16 pixels each, so `1rem` is `16px`. A bare number without a unit is refused. |
| number | A plain TOML number inside the range, such as `1.25` or `400`. Don't put it in quotes. |
| choice | One of the listed words, in quotes. |
| font | One of the seven font names above, in quotes. |
| image | `none`, or a base64 data URI of a PNG, JPEG, GIF or WebP image. A web address is refused. |
| shadow | `none`, or an optional `inset` followed by two to four lengths in pixels and then a colour, such as `0px 2px 8px rgba(0, 0, 0, 0.4)`. The colour is a hex colour or `rgb()`, `rgba()`, `hsl()`, `hsla()` or `oklch()`. A length of `0` needs its unit, so write `0px`. |
| duration | A whole number of milliseconds followed by `ms`, up to `2000ms`. |
| boolean | `true` or `false`. |

## The stylesheet

The stylesheet is plain CSS for anything the tokens can't do. It is optional. Its rules reach only elements inside the signed-in app.

The stylesheet depends on the app's markup, which can change as the app is rebuilt. Prefer tokens where you can, and use the custom properties (`var(--colors-accent)`) inside your rules so they follow the theme.

### What is allowed

- Ordinary rules, selectors, declarations and comments.
- These at-rules: `@media`, `@supports`, `@keyframes`, `@container` and `@layer`.
- `url()` only when it holds a base64 `data:image` of a PNG, JPEG, GIF or WebP image. Quotes are optional.
- Custom properties and `var()`.
- These functions:
  - Maths: `calc`, `min`, `max`, `clamp`, `round`, `mod`, `rem`, `abs`, `sign`, `sin`, `cos`, `tan`.
  - Colour: `rgb`, `rgba`, `hsl`, `hsla`, `hwb`, `lab`, `lch`, `oklab`, `oklch`, `color`, `color-mix`, `light-dark`.
  - Gradients: `linear-gradient`, `radial-gradient`, `conic-gradient` and their `repeating-` forms.
  - Transforms: `translate`, `translatex`, `translatey`, `translatez`, `translate3d`, `rotate`, `rotatex`, `rotatey`, `rotatez`, `rotate3d`, `scale`, `scalex`, `scaley`, `scalez`, `scale3d`, `skew`, `skewx`, `skewy`, `matrix`, `matrix3d`, `perspective`.
  - Timing: `cubic-bezier`, `steps`, `linear`.
  - Filters: `blur`, `brightness`, `contrast`, `drop-shadow`, `grayscale`, `hue-rotate`, `invert`, `opacity`, `saturate`, `sepia`.
  - Layout: `repeat`, `minmax`, `fit-content`, `counter`, `counters`.
  - Shapes: `inset`, `circle`, `ellipse`, `polygon`.
  - Selectors: `not`, `is`, `where`, `has`, `nth-child`, `nth-last-child`, `nth-of-type`, `nth-last-of-type`, `lang`, `dir`.

### What is refused

- Any at-rule not listed above, including `@import`, `@font-face` and `@namespace`. Nothing can load a stylesheet or a font.
- Any function not listed above, including `attr()`, `image-set()`, `element()` and `expression()`.
- `url()` that points at a web address, or at anything other than a base64 image.
- A backslash anywhere. Escapes are refused outright, because a disguised `url(` would look like something else to a filter and like `url(` to the browser.
- Null characters.
- The words `behavior`, `-ms-behavior` and `-moz-binding`.
- A string containing `javascript:`.
- The HTML comment markers `<!--` and `-->`, and the `<` character.
- Malformed `url()` or strings.

If a theme is refused, the response lists each reason, up to 20.

## Size and number limits

By default, one theme's TOML and stylesheet together can be up to 64 KiB (65,536 bytes), and one account can keep up to 20 themes. These are live settings, so they can change.

## A full example

This theme sets every table. The stylesheet follows it.

```toml
[meta]
schema = 1
name = "Midnight Red"
author = "your-github-username"
version = "1.0.0"
description = "Dark paper, red ink."
base = "midnight"

[colors]
background = "#0b0b0d"
surface = "#141417"
surface-raised = "#1c1c21"
text = "#f2f2f2"
text-muted = "rgb(170 170 178)"
accent = "#ff2b2b"
accent-text = "#000000"
border = "hsl(240 6% 22%)"
focus = "oklch(0.7 0.2 25)"
success = "#5ac800"
warning = "#ffb020"
danger = "#ff2b2b"
selection = "rgba(255, 43, 43, 0.3)"

[background]
image = "none"
pattern = "grid"
pattern-color = "#ffffff10"
pattern-size = "24px"

[typography]
heading-font = "Instrument Serif"
body-font = "JetBrains Mono"
mono-font = "JetBrains Mono"
base-size = "15px"
scale = 1.25
line-height = 1.5
heading-weight = 400
body-weight = 400

[spacing]
unit = "4px"
density = "comfortable"
page-gutter = "1.5rem"
max-width = "1440px"

[radius]
small = "0px"
medium = "2px"
large = "4px"
pill = "999px"

[border]
width = "1px"
style = "solid"

[shadow]
small = "none"
medium = "0px 2px 8px rgba(0, 0, 0, 0.4)"
large = "inset 0px 8px 24px 2px #00000080"

[motion]
duration = "160ms"
easing = "ease-out"
reduce = false
```

And an optional stylesheet to go with it. The class names are examples. Check the app's markup for the ones you want.

```css
/* A soft accent gradient on cards, using the theme's own tokens. */
.card {
  background: linear-gradient(90deg, var(--colors-accent), color-mix(in oklch, red 40%, white));
  border-radius: calc(var(--radius-small) * 2);
}

@media (prefers-reduced-motion: no-preference) {
  .badge {
    animation: pulse 2s steps(4) infinite;
  }
}

@keyframes pulse {
  from { opacity: 0.4; }
  to { opacity: 1; }
}
```

## Managing themes through the API

Themes are stored by the API. These routes need you to be signed in, except where noted.

| Method and path | What it does |
|---|---|
| `GET /themes` | List your themes, most recently changed first. |
| `POST /themes` | Create a theme. Body: `{"toml": "...", "css": "...", "public": false}`. |
| `GET /themes/{id}` | Read a theme. Your own, or any public one. Public themes can be read without signing in. |
| `PUT /themes/{id}` | Replace a theme you own. Same body as `POST`. |
| `DELETE /themes/{id}` | Delete a theme you own. |
| `GET /me/theme` | The theme applied to your account, or `null`. |
| `PUT /me/theme` | Apply `{"id": "..."}`, your own theme or a public one. Send `{"id": null}` to go back to the default. |

An invalid theme is answered with status 422 and `{"error": "...", "problems": ["...", "..."]}`. Creating a theme past your account's limit is a 409.

## Sharing

A theme is private until you make it public. A public theme can be opened by anyone with its id, and anyone signed in can apply it to their account. A private theme looks the same to others as a theme that doesn't exist.

A shared theme shows the author's username. When the author saves a change, people using it get the new version.
