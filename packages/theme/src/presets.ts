/*
 * The built-in themes. "default" is the app in the landing page's language:
 * white paper, black ink, one red. A user theme starts from one of these
 * (meta.base) and overrides what it names.
 */
export const presets = {
  default: `
[meta]
schema = 1
name = "Default"
description = "Paper and ink, one red."

[colors]
background = "#ffffff"
surface = "#ffffff"
surface-raised = "#f6f6f6"
text = "#000000"
text-muted = "#6b6b6b"
accent = "#ff0000"
accent-text = "#000000"
border = "#000000"
focus = "#ff0000"
success = "#5ac800"
warning = "#f0a000"
danger = "#ff0000"
selection = "#ff000033"

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
`,
  midnight: `
[meta]
schema = 1
name = "Midnight"
base = "default"
description = "The same page, after dark."

[colors]
background = "#0a0a0c"
surface = "#111114"
surface-raised = "#18181c"
text = "#f2f2f2"
text-muted = "#9a9aa2"
accent = "#ff3030"
accent-text = "#000000"
border = "#f2f2f2"
selection = "#ff303040"
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
`,
} as const;

export type PresetName = keyof typeof presets;
