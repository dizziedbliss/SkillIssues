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
} as const;

export type PresetName = keyof typeof presets;
