---
title: Themes
slug: /docs/themes
section: Guides
summary: How to restyle the signed-in app with a theme file of design tokens and an optional stylesheet, with every key and rule spelled out.
draft: false
updated: 2026-10-27
---

A theme restyles the signed-in app. It has two parts:

1. A **TOML file of design tokens**: colours, fonts, spacing and so on. This is required, but it can be tiny.
2. An optional **stylesheet** of CSS, for changes the tokens can't express.

Themes are saved to your account, so they follow you from one device to another. You can keep them private or make them public and share them by link.

The landing page is never themed. A theme applies only inside the signed-in app.

The format on this page is what the server and the app both enforce.

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
| `default` | Warm paper and ink, one red. This is the default if you set no base. |
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

The palettes come from each project's published colours, and every built-in has a light and a dark half (see `[light]` and `[dark]` below). A few colours were darkened so text and buttons pass a 4.5:1 contrast check, such as Nord's frost blue in light mode. The rounded ones also round corners and fade between pages.

## Starting a theme in the app

In the app, **Settings → Themes** has four tabs:

- **Built-in** shows every built-in as a miniature of itself: its background, a panel, its text and its accent. Built-ins can't be changed: **Use** applies one as it is, and **Customize** makes your own copy and opens it in the editor.
- **Yours** lists your themes: use, edit, duplicate or delete one, or start a **New theme** from the theme you're using or any built-in.
- **Community** lists the themes people have shared, most used or newest, searchable by name: use one, duplicate it, or report it.
- **Shared with you** finds a public theme by its id: use it as it is, or **Duplicate** it into your themes to change it.

A new theme, a customised built-in and a duplicate all start as a full copy: every token written out, so you change only the lines you want. Delete a line and it falls back to the base preset.

**Appearance** in the profile menu lists your last few themes, to switch between in one click, and **See more themes** opens this page. With nothing chosen, the app uses Midnight. The switch beside the theme in use, and the theme button in the footer, switch it between its light and dark halves.

### What your colours paint

The app keeps its accent rare, so a theme's accent marks what matters rather than everything:

- **`accent`** colours the main button on a page, where you are (the current nav item and settings view), what you've earned or what's live (level bars, XP, badge dots), and section headings. Page titles and body text use `text`.
- **Lines and chips** (the hairlines between rows, label chips, hover backgrounds) are mixed from `text` into `background` and `surface`, not from the accent, so a loud accent doesn't flood the page.
- **`surface`** is for things that float or hold input: menus, dialogs, the search palette, search boxes and fields.
- **`radius.small`** rounds chips, badges and progress bars; **`radius.medium`** buttons, fields and segmented controls. Set them to `"0px"` for square corners.

### The editor

The editor checks your theme as you type, by the same rules and in the same words as the server, with the line each problem is on. Beside it, a **preview** shows the app's own pieces (the nav, an issue card, buttons, a field, a notice, code) in your draft, without changing the theme the app is using. Light and Dark choose which half the preview shows. Under the preview, a token map says which token paints what.

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
| `accent-hover` | Buttons under the pointer. Without it, the text colour. |
| `link` | Links in issue descriptions. Without it, the text colour. |
| `info` | Notices. Without it, the accent. |
| `code-background` | Code in issue descriptions. Without it, `surface-raised`. |

### `[light]` and `[dark]`

The same keys as `[colors]`, for a theme's light and dark halves. The app has a light/dark mode, switched with the theme button in the footer and dark until you switch it. In light mode the keys in `[light]` replace those in `[colors]`, and in dark mode the keys in `[dark]` do; the rest stay. A theme with neither looks the same in both modes.

Every built-in has both halves: the dark ones bring their projects' official light variants (Catppuccin Latte, Alucard for Dracula, Nord's Snow Storm, Gruvbox light, Tokyo Night Day, Rosé Pine Dawn) and the light ones their dark ones. Each half of a theme is applied on top of the same half of its base preset, so a dark theme's own colours always win in dark mode.

The app also sets the CSS `color-scheme` of its root to `dark` or `light` from the background colour in use, so scrollbars and form controls match. Inside your stylesheet, `light-dark()` follows it.

### `[background]`

| Key | Type | Allowed values |
|---|---|---|
| `image` | image | `none`, or a `data:image/...;base64,...` URI of a PNG, JPEG, GIF or WebP image. |
| `pattern` | choice | `none`, `grid`, `dots` or `lines`. |
| `pattern-color` | colour | Any colour. |
| `pattern-size` | length | `4px` to `96px`. |
| `image-size` | choice | `cover` (the default), `contain` or `auto`. |
| `image-position` | choice | `center` (the default), `top`, `bottom`, `left` or `right`. |
| `image-repeat` | choice | `no-repeat` (the default), `repeat`, `repeat-x` or `repeat-y`. |
| `image-attachment` | choice | `fixed` (the default: it stays put as the page scrolls) or `scroll`. |

The image is the app root's background, under the pattern. A data URI is part of the theme file, so it counts towards the size limit.

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
| `heading-transform` | choice | `none`, `uppercase`, `lowercase` or `capitalize`. |
| `heading-tracking` | length | Letter spacing of headings and the nav, `-4px` to `8px` (`-0.25em` to `0.5em`). |
| `body-tracking` | length | Letter spacing of everything else, `-2px` to `6px`. |

Fonts a theme may name: `Instrument Serif` (a display serif), `JetBrains Mono`, `Krona One` (a wide display sans), `Inter` (a neutral grotesque), `Barlow Condensed` (a condensed grotesque), `Silkscreen` (a pixel font), `system-ui`, `serif`, `sans-serif` and `monospace`. The first six are served by SkillIssues itself, and your browser fetches one only when a theme uses it, so a theme never makes it fetch a font from another site. A font that isn't on the list is refused. `@font-face` stays refused: the site's security policy allows fonts only from SkillIssues.

### `[spacing]`

| Key | Type | Allowed values |
|---|---|---|
| `unit` | length | `2px` to `8px`. |
| `density` | choice | `compact`, `comfortable` or `spacious`. |
| `page-gutter` | length | `8px` to `96px`. |
| `max-width` | length | `640px` to `2400px`. |
| `sidebar-width` | length | `160px` to `480px`: the menu beside Settings. |

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

`small`, `medium`, `large` and `focus` each take a shadow. In the app, `small` sits under tags and buttons, `medium` under menus, `large` under dialogs such as search, and `focus` around whatever has keyboard focus, with the focus ring.

### `[motion]`

| Key | Type | Allowed values |
|---|---|---|
| `duration` | duration | A whole number of milliseconds from `0ms` to `2000ms`, such as `160ms`. |
| `easing` | choice | `linear`, `ease`, `ease-in`, `ease-out` or `ease-in-out`. |
| `reduce` | boolean | `true` or `false`. |
| `transition` | choice | How the app moves from one page to the next: `wipe` (the new page sweeps in from the left), `fade`, `slide`, `slide-up`, `iris` (it opens from the centre in a circle), `glitch` (a few offset slices, then the page) or `none`. |
| `stagger` | duration | `0ms` to `200ms`: the delay between one list item and the next as a list appears. `0ms`, the default, shows them together. |

`reduce = true` is the same as your system asking for reduced motion: nothing in the app transitions or animates, page transitions included, and that applies to your stylesheet's animations too. The app's root carries `data-reduce="1"` when it is on, so a rule can key off it as well as off `@media (prefers-reduced-motion: reduce)`.

## Value types

| Type | What is accepted |
|---|---|
| colour | A hex colour (`#rgb`, `#rgba`, `#rrggbb` or `#rrggbbaa`). Or `rgb()`, `rgba()`, `hsl()`, `hsla()`, `oklch()` or `oklab()` holding only numbers, `%`, `deg`, spaces, commas, slashes, `+` and `-`. Or one of `transparent`, `currentcolor`, `black` and `white`. |
| length | A number followed by `px`, `rem` or `em`, such as `15px` or `1.5rem`. The range limits above are in pixels. `rem` and `em` count as 16 pixels each, so `1rem` is `16px`. A bare number without a unit is refused. |
| number | A plain TOML number inside the range, such as `1.25` or `400`. Don't put it in quotes. |
| choice | One of the listed words, in quotes. |
| font | One of the ten font names above, in quotes. |
| image | `none`, or a base64 data URI of a PNG, JPEG, GIF or WebP image. A web address is refused. |
| shadow | `none`, or an optional `inset` followed by two to four lengths in pixels and then a colour, such as `0px 2px 8px rgba(0, 0, 0, 0.4)`. The colour is a hex colour or `rgb()`, `rgba()`, `hsl()`, `hsla()` or `oklch()`. A length of `0` needs its unit, so write `0px`. |
| duration | A whole number of milliseconds followed by `ms`, up to `2000ms`. |
| boolean | `true` or `false`. |

## The stylesheet

The stylesheet is plain CSS for anything the tokens can't do. It is optional. Its rules reach only elements inside the signed-in app.

Prefer tokens where you can, and use the custom properties (`var(--colors-accent)`) inside your rules so they follow the theme. For anything else, select the app's pieces by their hooks below, not by class names: classes change whenever the app is rebuilt, hooks don't.

### Hooks

These `data-ui` attributes are stable. Select them as `[data-ui="card"]`.

| Hook | What it is |
|---|---|
| `header`, `nav`, `nav-item`, `menu` | The app's header, its sections, one section's link, the profile menu. |
| `page` | The page under the header. |
| `footer` | The footer. |
| `search`, `search-dialog` | A search bar; the search overlay. |
| `filters`, `difficulty`, `pager` | The filter panel, the difficulty buttons, the page numbers. |
| `card`, `card-title`, `card-repo`, `chips`, `tech` | An issue in a list: the card, its title, its repository, its labels, its repository's languages. |
| `button`, `button-primary`, `bookmark` | Buttons, the main one on a page, the save bookmark. |
| `issue`, `issue-body` | An issue's page, its description. |
| `profile-header`, `level`, `stat` | A profile's top band (yours and public ones), the level with its bar, one of the numbers under it. |
| `language-level`, `badge`, `quest` | On a profile: one language's level, a badge (`data-ui-state="earned"` or `"locked"`), a quest (`data-ui-state="open"` or `"done"`). |
| `activity`, `pull-request`, `tracked-pull` | A list of pull requests; one of them (`data-ui-state="open"`, `"merged"` or `"closed"`); one the XP tracker holds, on Your contributions and, while open, in Next up. |
| `path`, `claims`, `graph`, `xp-reveal` | In Next up: your learning path, an issue you claimed; the year's graph on Your contributions; the one-time level reveal. |
| `settings-nav` | The menu beside Settings. |
| `link`, `input`, `notice` | A link, a field, a notice. |

Items of a list that animates in also carry `data-ui-index` and `--ui-index` (their place in the list), which is how `[motion] stagger` works; your own animation can use them.

### How the stylesheet is scoped

Your rules are wrapped in `@scope ([data-app])`, so they reach only the app:

- `:scope` is the app's root element. Use it where you'd reach for `:root` or `body`: `:scope::before` can draw over the whole app.
- `:root`, `html` and `body` are outside the scope and match nothing.
- Any property may be set, `position: fixed` included, and any pseudo-class or pseudo-element, such as `::before`, `::selection` or `::-webkit-scrollbar` (on the app's own scrolling areas, in browsers that support it). The page's main scrollbar belongs to the document, outside the scope.

### What is allowed

- Ordinary rules, selectors, declarations and comments.
- These at-rules: `@media`, `@supports`, `@keyframes`, `@container` and `@layer`.
- `@property`, to register your own custom properties so they can be animated, such as a gradient's angle. Only names that start with `--theme-` may be registered, so the app's own tokens keep behaving as they do. These rules are lifted out of the scope, where the browser requires them.
- `url()` only when it holds a base64 `data:image` of a PNG, JPEG, GIF or WebP image. Quotes are optional.
- Custom properties and `var()`.
- These functions:
  - Maths: `calc`, `min`, `max`, `clamp`, `round`, `mod`, `rem`, `abs`, `sign`, `sin`, `cos`, `tan`.
  - Colour: `rgb`, `rgba`, `hsl`, `hsla`, `hwb`, `lab`, `lch`, `oklab`, `oklch`, `color`, `color-mix`, `light-dark`.
  - Gradients: `linear-gradient`, `radial-gradient`, `conic-gradient` and their `repeating-` forms.
  - Transforms: `translate`, `translatex`, `translatey`, `translatez`, `translate3d`, `rotate`, `rotatex`, `rotatey`, `rotatez`, `rotate3d`, `scale`, `scalex`, `scaley`, `scalez`, `scale3d`, `skew`, `skewx`, `skewy`, `matrix`, `matrix3d`, `perspective`.
  - Timing: `cubic-bezier`, `steps`, `linear`.
  - Scroll-driven animation: `scroll` and `view`, for `animation-timeline`.
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

If a theme is refused, the response lists each reason (up to 20 from the stylesheet), each with its line: `line 7: colors.acent is not a theme key`, `stylesheet line 3: @import is not allowed`. Problems in the file and in the stylesheet come back together.

## Size and number limits

By default, one theme's TOML and stylesheet together can be up to 64 KiB (65,536 bytes), background image included, and one account can keep up to 20 themes. These are live settings, so they can change.

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
| `POST /themes/validate` | Check `{"toml": "...", "css": "..."}` exactly as a save would, storing nothing: `{"valid": true, "name": "..."}`, or 422 with every problem and its line. No sign-in needed. |
| `GET /me/theme` | What your account has applied: a theme, `{"preset": "nord"}` for a built-in used as it is, or `null` (the app's default, Midnight). |
| `PUT /me/theme` | Apply `{"id": "..."}` (your own theme or a public one) or `{"preset": "nord"}` (a built-in). Send `{"id": null}` to go back to the default. |

An invalid theme is answered with status 422 and `{"error": "...", "problems": ["...", "..."]}`. Creating a theme past your account's limit is a 409.

## Sharing

A theme is private until you make it public. A public theme can be opened by anyone with its id, and anyone signed in can apply it to their account. A private theme looks the same to others as a theme that doesn't exist.

A shared theme shows the author's username. When the author saves a change, people using it get the new version.
