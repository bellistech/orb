# nocterm design

View-only host health dashboard for the terminal. Same binary over SSH and on
the desk monitor. Host metrics first; Unheaded data (BGP, eBPF, app logs, The
Well) later as more panels.

## Display targets

| Target | Size | Color |
|---|---|---|
| Desk: 3440x1440 ultrawide, cage + foot kiosk | ~215x45 at 16x32 cells | truecolor |
| Desk: bare VT (fallback) | 430x90 at 8x16 | 16 colors (auto-downsampled) |
| SSH from a laptop/desktop terminal | anything >= 80x24 | truecolor |

The bare VT cannot do more than 16 colors. The kiosk path (a Wayland
compositor running one fullscreen terminal) is how the desk gets the real
palette. No pixels drawn directly: that would lose SSH.

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

## Themes

`gizzard` (default, psychedelic), `lantern` (soft pastel), `phosphor`
(amber CRT). Each theme is a background, text tones, a value ramp, and one
accent per panel.

CPU also gets a per-core heatmap: one strip per core over time, two cores per
row via upper half blocks (fg = top core, bg = bottom core).

## Later

- Unheaded panels: BGP sessions, eBPF/Shield counters, Wotan, The Well,
  log tail with severity color.
- Kiosk unit: systemd service running cage + foot + nocterm on the desk tty.
- Native pixel backend, no terminal at all: [ADR-0001](adr/0001-native-kiosk-renderer.md).
- Name.
