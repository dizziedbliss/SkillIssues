/*
 * The built-in themes. "default" is the app in the landing page's language:
 * warm paper, warm ink, one saturated red, set per mode to pass AA. Its
 * colours and Midnight's are the site's brand colours (web/src/styles/base.css):
 * change them together. A user theme starts from one of these
 * (meta.base) and overrides what it names.
 */
export const presets = {
  default: `
[meta]
schema = 1
name = "Default"
description = "Warm paper and ink, one red."

[colors]
background = "#faf9f6"
surface = "#fdfcfa"
surface-raised = "#f3f1ec"
text = "#161412"
text-muted = "#5a554f"
accent = "#d81b0c"
accent-text = "#faf9f6"
border = "#3a3631"
focus = "#d81b0c"
success = "#16873a"
warning = "#f0a000"
danger = "#d81b0c"
selection = "#d81b0c33"

[background]
image = "none"
pattern = "none"
pattern-color = "#00000010"
pattern-size = "24px"

[typography]
heading-font = "Instrument Serif"
body-font = "JetBrains Mono"
mono-font = "JetBrains Mono"
base-size = "16px"
scale = 1.25
line-height = 1.45
heading-weight = 400
body-weight = 400

[spacing]
unit = "4px"
density = "comfortable"
page-gutter = "45px"
max-width = "1460px"

[radius]
small = "3px"
medium = "0px"
large = "0px"
pill = "999px"

[border]
width = "1px"
style = "solid"

[shadow]
small = "1px 1px 2px #00000040"
medium = "0px 6px 20px #00000024"
large = "0px 24px 64px #00000030"

[motion]
duration = "380ms"
easing = "ease-out"
reduce = false
transition = "wipe"

# Midnight's colours, after dark.
[dark]
background = "#0f0e0d"
surface = "#151413"
surface-raised = "#1c1a18"
text = "#f3f0ea"
text-muted = "#a8a39b"
accent = "#ff3322"
accent-text = "#0f0e0d"
border = "#9c968e"
focus = "#ff3322"
success = "#43c463"
warning = "#f0a000"
danger = "#ff3322"
selection = "#ff332240"
`,
  midnight: `
[meta]
schema = 1
name = "Midnight"
base = "default"
description = "The same page, after dark."

[colors]
background = "#0f0e0d"
surface = "#151413"
surface-raised = "#1c1a18"
text = "#f3f0ea"
text-muted = "#a8a39b"
accent = "#ff3322"
accent-text = "#0f0e0d"
border = "#9c968e"
focus = "#ff3322"
success = "#43c463"
danger = "#ff3322"
selection = "#ff332240"

# Default's paper and ink, by day.
[light]
background = "#faf9f6"
surface = "#fdfcfa"
surface-raised = "#f3f1ec"
text = "#161412"
text-muted = "#5a554f"
accent = "#d81b0c"
accent-text = "#faf9f6"
border = "#3a3631"
focus = "#d81b0c"
success = "#16873a"
warning = "#f0a000"
danger = "#d81b0c"
selection = "#d81b0c33"
`,
  terminal: `
[meta]
schema = 1
name = "Terminal"
base = "default"
description = "Green on black, all mono."

[colors]
background = "#050805"
surface = "#050805"
surface-raised = "#0b120b"
text = "#7dff7a"
text-muted = "#3fa03c"
accent = "#7dff7a"
accent-text = "#050805"
border = "#3fa03c"
focus = "#ffffff"

[typography]
heading-font = "JetBrains Mono"
scale = 1.15

[background]
pattern = "lines"
pattern-color = "#7dff7a14"
pattern-size = "4px"

# Green on paper.
[light]
background = "#f3fbf2"
surface = "#f3fbf2"
surface-raised = "#e4f2e2"
text = "#0b4f0b"
text-muted = "#2d6b2b"
accent = "#0b7a0b"
accent-text = "#f3fbf2"
border = "#2d6b2b"
focus = "#000000"
success = "#0b7a0b"
warning = "#8a6d00"
danger = "#b00020"
selection = "#0b7a0b33"
`,
  contrast: `
[meta]
schema = 1
name = "High contrast"
base = "default"
description = "Larger type, heavier lines."

[colors]
text-muted = "#000000"
accent = "#d00000"
accent-text = "#ffffff"
focus = "#0047ff"

[typography]
base-size = "18px"
line-height = 1.6

[border]
width = "2px"

# Black, white and yellow focus.
[dark]
background = "#000000"
surface = "#000000"
surface-raised = "#111111"
text = "#ffffff"
text-muted = "#ffffff"
accent = "#ff4d4d"
accent-text = "#000000"
border = "#ffffff"
focus = "#ffd400"
success = "#3ddc84"
warning = "#ffd400"
danger = "#ff4d4d"
selection = "#ff4d4d55"
`,
  "catppuccin-mocha": `
[meta]
schema = 1
name = "Catppuccin Mocha"
base = "default"
description = "Soothing pastels after dark (catppuccin.com)."

[colors]
background = "#1e1e2e"
surface = "#181825"
surface-raised = "#313244"
text = "#cdd6f4"
text-muted = "#a6adc8"
accent = "#cba6f7"
accent-text = "#1e1e2e"
border = "#585b70"
focus = "#b4befe"
success = "#a6e3a1"
warning = "#f9e2af"
danger = "#f38ba8"
selection = "#cba6f740"

[radius]
small = "6px"
medium = "8px"
large = "12px"

[motion]
transition = "fade"

# Catppuccin Latte.
[light]
background = "#eff1f5"
surface = "#e6e9ef"
surface-raised = "#ccd0da"
text = "#4c4f69"
text-muted = "#5c5f77"
accent = "#8839ef"
accent-text = "#eff1f5"
border = "#acb0be"
focus = "#7287fd"
success = "#40a02b"
warning = "#df8e1d"
danger = "#d20f39"
selection = "#8839ef33"
`,
  "catppuccin-latte": `
[meta]
schema = 1
name = "Catppuccin Latte"
base = "default"
description = "Catppuccin's light flavour (catppuccin.com)."

[colors]
background = "#eff1f5"
surface = "#e6e9ef"
surface-raised = "#ccd0da"
text = "#4c4f69"
text-muted = "#5c5f77"
accent = "#8839ef"
accent-text = "#eff1f5"
border = "#acb0be"
focus = "#7287fd"
success = "#40a02b"
warning = "#df8e1d"
danger = "#d20f39"
selection = "#8839ef33"

[radius]
small = "6px"
medium = "8px"
large = "12px"

[motion]
transition = "fade"

# Catppuccin Mocha.
[dark]
background = "#1e1e2e"
surface = "#181825"
surface-raised = "#313244"
text = "#cdd6f4"
text-muted = "#a6adc8"
accent = "#cba6f7"
accent-text = "#1e1e2e"
border = "#585b70"
focus = "#b4befe"
success = "#a6e3a1"
warning = "#f9e2af"
danger = "#f38ba8"
selection = "#cba6f740"
`,
  "dracula": `
[meta]
schema = 1
name = "Dracula"
base = "default"
description = "The dark theme for everything (draculatheme.com)."

[colors]
background = "#282a36"
surface = "#21222c"
surface-raised = "#44475a"
text = "#f8f8f2"
text-muted = "#bfbfd0"
accent = "#ff79c6"
accent-text = "#282a36"
border = "#6272a4"
focus = "#bd93f9"
success = "#50fa7b"
warning = "#f1fa8c"
danger = "#ff5555"
selection = "#ff79c640"

[radius]
small = "4px"
medium = "6px"
large = "8px"

[motion]
transition = "fade"

# Alucard, Dracula's light theme.
[light]
background = "#fffbeb"
surface = "#fff6d6"
surface-raised = "#efe9d0"
text = "#1f1f1f"
text-muted = "#635d97"
accent = "#a3144d"
accent-text = "#fffbeb"
border = "#cfcfde"
focus = "#644ac9"
success = "#14710a"
warning = "#846e15"
danger = "#cb3a2a"
selection = "#a3144d33"
`,
  "nord": `
[meta]
schema = 1
name = "Nord"
base = "default"
description = "An arctic, north-bluish palette (nordtheme.com)."

[colors]
background = "#2e3440"
surface = "#3b4252"
surface-raised = "#434c5e"
text = "#eceff4"
text-muted = "#d8dee9"
accent = "#88c0d0"
accent-text = "#2e3440"
border = "#4c566a"
focus = "#81a1c1"
success = "#a3be8c"
warning = "#ebcb8b"
danger = "#bf616a"
selection = "#88c0d040"

[radius]
small = "4px"
medium = "6px"
large = "8px"

[motion]
transition = "fade"

# Nord's Snow Storm (the frost blue darkened so buttons read).
[light]
background = "#eceff4"
surface = "#e5e9f0"
surface-raised = "#d8dee9"
text = "#2e3440"
text-muted = "#4c566a"
accent = "#4c6a92"
accent-text = "#eceff4"
border = "#9aa5b8"
focus = "#81a1c1"
success = "#5f7a46"
warning = "#b06a3c"
danger = "#bf616a"
selection = "#5e81ac33"
`,
  "gruvbox": `
[meta]
schema = 1
name = "Gruvbox"
base = "default"
description = "Retro groove, warm and earthy (gruvbox, dark)."

[colors]
background = "#282828"
surface = "#32302f"
surface-raised = "#3c3836"
text = "#ebdbb2"
text-muted = "#bdae93"
accent = "#fe8019"
accent-text = "#282828"
border = "#665c54"
focus = "#fabd2f"
success = "#b8bb26"
warning = "#fabd2f"
danger = "#fb4934"
selection = "#fe801940"

# Gruvbox light.
[light]
background = "#fbf1c7"
surface = "#f9f5d7"
surface-raised = "#ebdbb2"
text = "#3c3836"
text-muted = "#665c54"
accent = "#af3a03"
accent-text = "#fbf1c7"
border = "#bdae93"
focus = "#b57614"
success = "#79740e"
warning = "#b57614"
danger = "#9d0006"
selection = "#af3a0333"
`,
  "solarized-light": `
[meta]
schema = 1
name = "Solarized Light"
base = "default"
description = "Precision colours for machines and people (ethanschoonover.com/solarized)."

[colors]
background = "#fdf6e3"
surface = "#fdf6e3"
surface-raised = "#eee8d5"
text = "#073642"
text-muted = "#586e75"
accent = "#cb4b16"
accent-text = "#fdf6e3"
border = "#93a1a1"
focus = "#268bd2"
success = "#859900"
warning = "#b58900"
danger = "#dc322f"
selection = "#cb4b1633"

[radius]
small = "2px"
medium = "3px"
large = "4px"

# Solarized Dark.
[dark]
background = "#002b36"
surface = "#073642"
surface-raised = "#0a4050"
text = "#93a1a1"
text-muted = "#839496"
accent = "#cb4b16"
accent-text = "#ffffff"
border = "#586e75"
focus = "#268bd2"
success = "#859900"
warning = "#b58900"
danger = "#dc322f"
selection = "#cb4b1640"
`,
  "tokyo-night": `
[meta]
schema = 1
name = "Tokyo Night"
base = "default"
description = "Downtown Tokyo's lights at night."

[colors]
background = "#1a1b26"
surface = "#16161e"
surface-raised = "#24283b"
text = "#c0caf5"
text-muted = "#a9b1d6"
accent = "#7aa2f7"
accent-text = "#1a1b26"
border = "#414868"
focus = "#bb9af7"
success = "#9ece6a"
warning = "#e0af68"
danger = "#f7768e"
selection = "#7aa2f740"

[radius]
small = "4px"
medium = "6px"
large = "10px"

[motion]
transition = "fade"

# Tokyo Night Day.
[light]
background = "#e1e2e7"
surface = "#e9e9ed"
surface-raised = "#d5d6db"
text = "#2f52a8"
text-muted = "#4a5a96"
accent = "#7847bd"
accent-text = "#ffffff"
border = "#a8aecb"
focus = "#2e7de9"
success = "#587539"
warning = "#8c6c3e"
danger = "#f52a65"
selection = "#7847bd33"
`,
  "rose-pine": `
[meta]
schema = 1
name = "Rosé Pine"
base = "default"
description = "All natural pine, faux fur and a bit of soho vibes (rosepinetheme.com)."

[colors]
background = "#191724"
surface = "#1f1d2e"
surface-raised = "#26233a"
text = "#e0def4"
text-muted = "#908caa"
accent = "#eb6f92"
accent-text = "#191724"
border = "#524f67"
focus = "#c4a7e7"
success = "#9ccfd8"
warning = "#f6c177"
danger = "#eb6f92"
selection = "#eb6f9240"

[radius]
small = "6px"
medium = "8px"
large = "12px"

[motion]
transition = "fade"

# Rosé Pine Dawn.
[light]
background = "#faf4ed"
surface = "#fffaf3"
surface-raised = "#f2e9e1"
text = "#575279"
text-muted = "#625e7d"
accent = "#286983"
accent-text = "#faf4ed"
border = "#dfdad9"
focus = "#907aa9"
success = "#56949f"
warning = "#ea9d34"
danger = "#b4637a"
selection = "#28698333"
`,
} as const;

export type PresetName = keyof typeof presets;
