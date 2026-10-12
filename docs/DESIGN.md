# orb design

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

## News & weather page

`w` switches to it, `d` back; tabs in the header's top border show which
page is up. Weather on the left: current conditions with 12x8 pixel art
(rain, snow and storms fall, one step per second), the hourly temperature as
a heat-colored curve over temperatures, mini icons and hours, and a five-day
forecast with range bars on a shared min..max track. Temperatures use the
heat ramp from 20°F (cool) to 105°F (hot).

On the right, six headline windows on top and the reader below: one story at a time in a random order
(seeded by the fetch, so stable between frames), only items whose feed description holds a real summary (80+
characters after stripping HTML and boilerplate). The headline shows for
1.5 s, then the body types at 40 characters/s into
lines of at most 72 characters; each typed page holds 5 s, with a blinking
▼ when another follows. The schedule is a pure function of time since the
news was fetched, so every frame is computable without state. Space pauses
it (`ui.ReaderClock`): the clock freezes and the reader keeps the news it
was showing, so a refresh can't swap the story mid-read. Arrows move a
story back (up/left) or on (down/right) from its headline, resuming if
paused. In graphics mode the reader's text is double size: each glyph spans
2x2 tiles (`canvas.Cell.Quad`) and every tile draws its own quarter, so tile
diffing still works. The headline
windows show one line per headline, pages rotating every 15 s with
staggered starts. The page lays out every frame while typing; tile diffing
keeps a frame at ~0.15 ms. Feeds are JSON caches,
configured in `~/.config/orb/config`.

## Network page

`n`. Listening sockets (TCP LISTEN, UDP unconnected) sorted by port, with
service name, address (`*` for any) and owner; connections with state
colored by meaning (established calm, half-closed warm, TIME-WAIT faint);
counts by state and top peers; the interfaces panel. Sockets come from
`/proc/net/*`, owners by matching `socket:[inode]` fd links under
`/proc/<pid>/fd` (bounded walk, ~5 ms). The sampler reads them every 2 s,
and only while the page asks.

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

Console keys arrive raw; `gfx.ParseKeys` consumes whole escape sequences
so an arrow (ESC [ A) is never read as ESC or a letter. ESC does not quit;
q and Ctrl+C do.

The orb: a glass sphere at the left of the header, drawn as pixels over
the header's full height. Its mist swirls through a few quantized bands
(palette cycling, `ui.OrbMist`) from deep accent through the accent to the
hot end of the theme ramp, with a glowing core, darker limb, highlight, rim
and a pulsing halo blended over the frame behind it. Terminals get a 6x6
block-art version.

Overlays (`gfx/overlay.go`) are pixel effects that ignore the tile grid:
the orb, and sparkles, four-pointed glints that bloom and fade on window
bevels on staggered periods. Tiles under an overlay this frame or last are
restored and redrawn before overlays are painted, so nothing leaves trails.
`TestIncrementalMatchesFullRender` holds the renderer to that: after any
sequence of frames, the incremental frame equals a fresh full render.

In the reader, letters glow as they are typed and cool to the text color
over 0.35 s.

Animation (default 15 fps): values ease out over 600 ms after each sample,
a light sweeps each title every 7 s (staggered), stars twinkle in the gaps.
The backdrop and windows are a cached base layer; a tile is restored and
redrawn only when its cell or shimmer changed, stars twinkle in 6 steps, and
only changed spans are copied to the framebuffer. Layout reruns only during a
tween or when the clock second ticks. Idle frames cost ~0.1 ms at any scale.

## Themes

`phosphor` (default, amber CRT), `gizzard` (psychedelic), `lantern` (soft
pastel), `chrono` (blue RPG windows), `arcane` (violet, mystical). Each theme is a background, text tones,
a value ramp, one accent per panel, and optionally explicit window colors.

CPU also gets a per-core heatmap: one strip per core over time, two cores per
row via upper half blocks (fg = top core, bg = bottom core).

## Later

- Unheaded panels: BGP sessions, eBPF/Shield counters, Wotan, The Well,
  log tail with severity color.
- Autostart graphics mode on the desk VT (systemd unit or login hook).
- DRM/KMS backend and hosting other binaries fullscreen: [ADR-0001](adr/0001-native-kiosk-renderer.md).
