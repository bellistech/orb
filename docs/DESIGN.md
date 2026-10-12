# nocterm design

View-only host health dashboard for the terminal. Same binary over SSH and on
the desk monitor. Host metrics first; Unheaded data (BGP, eBPF, app logs, The
Well) later as more panels.

## Display targets

| Target | Size | Color |
|---|---|---|
| Desk: 3440x1440 ultrawide, `-gfx` on the VT | 107x45 tiles at scale 4 | 24-bit pixels |
| Desk: bare VT without `-gfx` | 430x90 at 8x16 | 16 colors (auto-downsampled) |
| SSH from a laptop/desktop terminal | anything >= 80x24 | truecolor |

The bare VT cannot do more than 16 colors, so the desk uses graphics mode:
the same panels drawn as pixels on the framebuffer, no compositor or
terminal emulator. SSH keeps the terminal renderer.

## Principles

1. **Readable from across the desk.** Big clock, big numbers, few words.
   Nothing that needs squinting at 3 meters.
2. **Color carries meaning.** One hue per panel (CPU is always cyan, memory is
   always magenta) so the eye finds a panel without reading its title. Values
   use one shared cool -> hot ramp everywhere: a bar that reaches pink is hot,
   whatever panel it is in.
3. **Gradients by position, not by value.** A bar's cell at 70% of its length is
   always the 70% color. A 30% bar is all cool; a 95% bar runs through the
   whole ramp to hot. Same rule for graph height.
4. **History, not just now.** Every rate has a graph (braille: 2x4 dots per
   cell) so trends read at a glance.
5. **View only.** No interaction needed at the desk. Over SSH, `t` cycles
   themes and `q` quits; that is all.
6. **Light.** Metrics read straight from /proc and /sys. No agents, no
   plugins, no Python.

## Layout

Wide (>= 180 cols), the ultrawide:

```
+-- clock ------+-- host / kernel / uptime / load ---------------+-- status --+
+-- CPU (per-core bars + total graph) ----+-- memory ----+-- GPU ------------+
|                                         |              |                   |
+-- network ----------+-- disks ----------+-- thermal ---+-- top processes --+
|                     |                   |              |                   |
+---------------------+-------------------+--------------+-------------------+
```

Medium (100-179) uses three rows of two or three panels; narrow (< 100) stacks
everything. Every panel draws into whatever box it gets and clips; nothing
assumes a size.

## Rendering

A cell canvas (rune + fg + bg per cell) with panels drawing into clipped
sub-regions, serialized to truecolor SGR. Bubble Tea v2 runs the loop and
downsamples colors to what the terminal supports. `ui.Render` never touches
the terminal, so another backend can consume the same canvas.

Widgets: rounded frame with title in the border, eighth-block gauges, braille
area graphs, block-font clock.

## Graphics mode

16-bit console look. Logical frame of 8x8 tiles (107x45 on the ultrawide),
scaled up with hard edges. Hand-drawn 5x7 font with a drop shadow, bold as a
double strike. Panels become RPG-menu windows: dark outline, top-lit bevel,
vertical gradient fill, title cut into the border as a tab. Blocks, braille
and lines are drawn from geometry so they tile without seams; braille dots
are 4x2 pixels and join into solid area graphs.

Animation (default 15 fps): values ease out over 600 ms after each sample,
a light sweeps each title every 7 s (staggered), stars twinkle in the gaps.
Only logical rows that changed are copied to the framebuffer.

## Themes

`phosphor` (default, amber CRT), `gizzard` (psychedelic), `lantern` (soft
pastel), `chrono` (blue RPG windows). Each theme is a background, text tones,
a value ramp, one accent per panel, and optionally explicit window colors.

CPU also gets a per-core heatmap: one strip per core over time, two cores per
row via upper half blocks (fg = top core, bg = bottom core).

## Later

- Unheaded panels: BGP sessions, eBPF/Shield counters, Wotan, The Well,
  log tail with severity color.
- Autostart graphics mode on the desk VT (systemd unit or login hook).
- DRM/KMS backend and hosting other binaries fullscreen: [ADR-0001](adr/0001-native-kiosk-renderer.md).
- Name.
