# Themes And Terminal Fonts

amux theme settings control human-facing status glyphs and status-line shape.
They do not change pane state, structured capture JSON, or event payloads.
Agents should keep using semantic fields such as pane state, host, tracked PRs,
and tracked issues instead of parsing rendered glyphs.

Config file: `~/.config/amux/config.toml` unless `AMUX_CONFIG` points
somewhere else.

## Icon Mode

`[theme] icons` selects the renderer icon preset:

```toml
[theme]
icons = "unicode" # ascii | unicode | nerd
```

Valid values:

| Value | Use when | Notes |
| --- | --- | --- |
| `unicode` | Your terminal handles common Unicode symbols. | Default. Uses compact symbols such as `●`, `◇`, `⚡`, and `[copy]`. |
| `ascii` | You need the safest fallback for plain terminals, CI logs, serial consoles, or remote environments with unknown font support. | Uses printable single-cell ASCII markers such as `*`, `.`, `+`, `#`, `I`, and `T`. |
| `nerd` | Your terminal is configured to use a Nerd Font-compatible patched font. | Uses Private Use Area glyphs for pane state, hosts, PRs, issues, tasks, copy mode, and connection state. |

Examples:

```toml
[theme]
icons = "ascii"
```

```toml
[theme]
icons = "unicode"
```

```toml
[theme]
icons = "nerd"
```

amux does not install, select, or manage terminal fonts. `icons = "nerd"` only
tells amux to emit Nerd Font glyphs. The terminal emulator still decides which
font is used to draw those glyphs.

Nerd Font icons and Powerline separators use Unicode Private Use Area code
points. Without a compatible patched font, those glyphs may render as boxes,
question marks, blank cells, or mismatched-width characters. That is a terminal
font setup problem, not a pane, PTY, or capture bug.

## Status Style

`[theme] status_style` selects the status-line preset:

```toml
[theme]
status_style = "compact" # compact | plain | powerline
```

Valid values:

| Value | Use when | Notes |
| --- | --- | --- |
| `compact` | You want the default status-line layout. | Default. Uses normal separators and works with `ascii`, `unicode`, or `nerd` icons. |
| `plain` | You want to avoid Powerline separators. | A non-Powerline fallback style for terminals where separator glyphs are unreliable. |
| `powerline` | Your terminal font renders Powerline separator glyphs correctly. | Uses Powerline separators in pane status lines and the global bar. |

Icon mode and status style are independent:

```toml
[theme]
icons = "nerd"
status_style = "powerline"
```

If pane status separators render as boxes or look misaligned, keep the icon mode
you want and switch the status style back to a non-Powerline value:

```toml
[theme]
icons = "nerd"
status_style = "compact"
```

## Font Diagnostic

Run the local diagnostic to see what your current terminal font renders:

```bash
amux doctor fonts
```

The command prints samples for the `ascii`, `unicode`, and `nerd` icon presets,
plus the Powerline separators used by `status_style = "powerline"`. It does not
connect to an amux server, install fonts, change config, or mutate terminal
settings.

If any sample appears as a box, question mark, missing glyph, blank cell, or
badly aligned glyph, choose a fallback config until the terminal font is fixed:

```toml
[theme]
icons = "ascii"
status_style = "compact"
```

## Text Captures

Text-mode captures are useful for checking the shape of rendered status lines.
The exact colors are omitted here.

Default Unicode icons with compact status:

```text
● [pane-1] #42, LAB-1651 @gpu build



 amux │ SESSION          1 panes │ ? help │ 00:00
```

Nerd icons with compact status:

```text
 [pane-1] 42, LAB-1651 gpu  build



 amux │ SESSION          1 panes │ ? help │ 00:00
```

Nerd icons with Powerline status:

```text
 pane-142, LAB-1651gpu build



 amux  SESSION          1 panes  ? help  00:00
```

If the Nerd or Powerline examples look wrong in your editor or terminal, use the
diagnostic command in the terminal where you run amux. Markdown renderers,
browsers, and code review tools often use different fonts from your terminal.

## Fallback Guidance

Use `icons = "ascii"` and `status_style = "compact"` for CI, logs, remote shells
viewed through unknown terminal stacks, SSH sessions from minimal clients, and
any setup where glyph width matters more than visual density.

Use `icons = "unicode"` with `status_style = "compact"` as the default local
interactive setup. It keeps the existing compact status rendering without
depending on patched font glyphs.

Use `icons = "nerd"` only after confirming that the terminal profile used for
amux is configured with a Nerd Font-compatible patched font. Add
`status_style = "powerline"` only after Powerline separators also render
correctly.

## Color Presets And Overrides

`[theme] preset` selects the base palette used for borders, pane status lines,
the global bar, and modal chrome. Per-pane accent colours (the colour each
pane is assigned at creation, shown in `capture --colors`) are unchanged by
presets.

```toml
[theme]
preset = "soft" # default | soft
```

| Value | Use when |
| --- | --- |
| `default` | The original Catppuccin Mocha palette. |
| `soft` | You run many panes in a terminal such as Ghostty and the default reads as harsh: lower-contrast dim borders, muted status backgrounds, desaturated indicator colours, and `accent_soften = 0.3` so pane accents are toned down when drawn. |

Any role can be overridden on top of the preset in `[theme.colors]`. Values
are 6-digit hex with or without `#`:

```toml
[theme]
preset = "soft"

[theme.colors]
dim = "#3c3f52"        # even softer inactive borders
surface0 = "#20202c"   # status-line background
accent_soften = 0.5    # 0 = server-assigned accent as-is, 1 = fully grey
```

Roles: `dim`, `text`, `surface0`, `surface1`, `blue`, `mauve`, `footer_key`,
`green`, `yellow`, `peach`, `red`, `accent_soften`.

Palette changes are client-side presentation only. Structured capture JSON,
`capture --colors` letters, and event payloads never change.

## Done Flash

When a pane's agent transitions busy → idle (the same edge that emits the
`idle` event) **and the pane is not the focused pane**, amux pulses that pane's
border and title background so completion is visible from across the layout.
The focused pane never flashes — you are already looking at it.

```toml
[theme]
done_flash = true              # default on
done_flash_color = "#a6e3a1"   # border colour while a pulse is on
done_flash_bg = "#3f6b4c"      # title-bar background while a pulse is on; "" = border only
done_flash_duration_ms = 1500  # total flash length (100–10000)
done_flash_pulses = 3          # number of on/off pulses (1–10)
```

Panes that were already idle when you attached do not flash; only live
transitions do. A pane going busy again cancels its flash.

## Needs-Input Glow

After the flash (or immediately, if `done_flash = false`) a pane that finished
while you were elsewhere keeps a **steady, subtle** tint on its border and
title background until you acknowledge it by focusing it or typing into it.
This is the "an agent is waiting on you" signal; it is deliberately distinct
from the done-flash pulse and from a plain idle pane (dim border, default
title background).

```toml
[theme]
needs_input_glow = true        # default on
needs_input_color = "#b4befe"  # border
needs_input_bg = "#4a4478"     # title-bar background; "" = border only
```

The glow clears when the pane is focused, when you type into it, or when its
agent goes busy again. A pane's mirror-connection tint (peach while
reconnecting) always takes precedence over attention tints.

## Alternating Pane Tints

Adjacent panes get a checkerboard content background so their boundaries are
easy to see. Parity is derived from layout position (column left→right, then
row within the column), so any two panes that share a border differ.

```toml
[theme]
alternate_tints = true         # default on
alternate_tint_bg = "#23232e"  # background applied to odd-parity panes
```

Only cells that use the terminal's default background are tinted; application
colours are preserved. The tint is applied by the interactive client's grid
renderer only — `amux capture`, `capture --colors`, and `AMUX_RENDER=full`
are unchanged. Pick a value a few steps lighter or darker than your terminal
background; with `AMUX_COLOR_PROFILE=ansi256` it is snapped to the nearest
256-colour entry.

## Powerline Status Style Note

With `status_style = "powerline"` the done-flash and needs-input tints apply
to the pane border only; the powerline title segments keep their own
backgrounds.
