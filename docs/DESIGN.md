# orb design

View-only host health dashboard for the terminal. Same binary over SSH and on
the desk monitor. Host metrics first; Unheaded data (BGP, eBPF, app logs, The
Well) later as more panels.

## Display targets

| Target | Size | Color |
|---|---|---|
| Desk: 3440x1440 ultrawide, `-gfx` on the VT | 143x60 tiles at scale 3 (default) | 24-bit pixels |
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
page is up. The six headline windows fill the top half across the whole
width; the bottom half holds the weather, narrow, and the reader, widest.
Weather: current conditions with 12x8 pixel art (rain, snow and storms
fall, one step per second; above the reading when the pane is narrow, with
the update time on its bottom line), the hourly temperature as
a heat-colored curve over temperatures, mini icons and hours, and a five-day
forecast with range bars on a shared min..max track. Temperatures use the
heat ramp from 20°F (cool) to 105°F (hot).

The reader shows one story at a time in a random order
(seeded by the fetch, so stable between frames), only items whose feed description holds a real summary (80+
characters after stripping HTML and boilerplate). The headline shows for
1.5 s, then the body types at 200 characters/s into
lines of at most 72 characters; each typed page holds 15 s, with a blinking
▼ when another follows. The schedule is a pure function of time since the
news was fetched, so every frame is computable without state. Space pauses
it (`ui.ReaderClock`): the clock freezes and the reader keeps the news it
was showing, so a refresh can't swap the story mid-read. Arrows move a
story back (up/left) or on (down/right) from its headline, resuming if
paused. In graphics mode the reader's text is double size: each glyph spans
2x2 tiles (`canvas.Cell.Quad`) and every tile draws its own quarter, so tile
diffing still works. The headline
windows show one line per headline, two when every headline fits that way,
pages rotating every 15 s with
staggered starts. The page lays out every frame while typing; tile diffing
keeps a frame at ~0.15 ms. Feeds are JSON caches,
configured in `~/.config/orb/config`.

## Network page

`n`. Listening sockets (TCP LISTEN, UDP unconnected) sorted by port, with
service name, address (`*` for any) and owner; connections with state
colored by meaning (established calm, half-closed warm, TIME-WAIT faint);
counts by state and top peers; the interfaces panel. Connections read
CLIENT -> SERVER: the arrow always runs from the ephemeral port to the
service port. Between 100 and 180 cells wide, connections sit top left
(few, long rows) and listening sockets take the whole bottom width (many,
short rows), flowing into as many 64-cell columns as they need. Sockets
come from
`/proc/net/*`, owners by matching `socket:[inode]` fd links under
`/proc/<pid>/fd` (bounded walk, ~5 ms). The sampler reads them every 2 s,
and only while the page asks.

## Links page

`l`. Layer 2 and 3 the way a switch or router shows it. Interfaces as
ports: state, kind (ether, wifi, bridge, bond, vlan, wireguard, tun), line
speed, MTU, rx/tx rates, a load sparkline (against line rate when the
driver reports one), error/drop totals that turn hot while they climb, and
a utilization bar per physical port. Bridges with their member ports (veths
are shown here, not as ports) and how many MACs each learned. LLDP
neighbors: system name, port, chassis, management addresses, capabilities.
The ARP/NDP neighbor table (link-local after global) and the main routing
table (default first, protocol colored: kernel faint, learned from a
routing protocol bright).

Interfaces come from `/sys/class/net`; neighbors, routes and the bridge
forwarding database from rtnetlink dumps (no privilege needed; parsed by
hand with every length checked, fuzzed). Read every sample, only while the
page is up; per-port history starts when it opens. LLDP comes from orb's
own listener (ADR-0003), so only when started as root.

## Attract mode

`i` arms it, off by default (Stevie: so the news page stays put while
being read). Armed, 30 s without a key starts a cycle through the pages
like an arcade attract screen: on to the next page at once, then 20 s on
the dashboard, 45 s on news (long enough for the reader to tell a story
from its headline), 15 s on network and links. Any key stops the cycle,
restarts the wait and then does its usual job; `i` disarms. Themes stay
put: the theme is the user's choice. `ui.Attract` is a pure state machine
fed key times and ticks (graphics frames, terminal samples), so it is
tested on a virtual clock; the header shows `i AUTO`, faint while armed,
lit while cycling.

## Privileges

Run as root, orb splits itself (ADR-0002): a helper with only
CAP_DAC_READ_SEARCH and CAP_SYS_PTRACE, as nobody, with no network, maps
socket inodes to processes; the main process drops to the invoking user
with no capabilities (CAP_SYS_TTY_CONFIG only if it took a console) before
fetching any feed. A second helper (ADR-0003) opens the LLDP packet socket
as root, then drops to nobody with no capabilities at all, decodes LLDP
frames and sends validated neighbors over a pipe.

## Rendering

A cell canvas (rune + fg + bg per cell) with panels drawing into clipped
sub-regions, serialized to truecolor SGR. Bubble Tea v2 runs the loop and
downsamples colors to what the terminal supports. `ui.Render` never touches
the terminal, so another backend can consume the same canvas.

Widgets: rounded frame with title in the border, eighth-block gauges, braille
area graphs, block-font clock.

## Graphics mode

16-bit console look. Logical frame of 8x8 tiles (143x60 on the ultrawide at the default scale 3),
scaled up with hard edges. Hand-drawn 5x7 font with a drop shadow, bold as a
double strike. Panels become RPG-menu windows: dark outline, top-lit bevel,
vertical gradient fill, title cut into the border as a tab. Blocks, braille
and lines are drawn from geometry so they tile without seams; braille dots
are 4x2 pixels and join into solid area graphs.

Console ownership: orb draws on the controlling terminal when it is a VT.
Otherwise (a pseudo-terminal: sudo-rs always allocates one, as do tmux and
SSH) root can take a free VT: VT_OPENQRY on tty0, VT_ACTIVATE + bounded
VT_WAITACTIVE, and on exit the display goes back to the VT that was
active. Signals are off on the console (ISIG cleared): Ctrl+C arrives as a
key and quits, Ctrl+Z cannot suspend orb with the screen in graphics.

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

Boot intro (`gfx.Intro`), drawn as pixels straight into a frame, not
through the canvas: a studio splash (the `gfx.Studio` wordmark in the
font scaled up, the header's hostname gradient, a light sweep, a glint,
a line drawn out from the middle; 1.7 s), then a title card (stars, the
orb growing out of nothing, a flash, ORB in big letters with a chrome
gradient per scanline, version and hostname typed out, a blinking PRESS
START; 2.1 s), then a 0.3 s fade to the dashboard. The title card holds
until a second sample has landed (rates exist), at most 4 s more, so the
dashboard is ready when it ends. Frames are a pure function of time;
each is diffed against the last and only changed spans are blitted
(~1.2 ms a frame at 3440x1440, scale 3). Any key skips it and is then
handled; `q` quits at once. Off with `-intro=false`, with `-fps 0` (no
animation was asked for, and a still title would only delay the
dashboard) and in terminals (opened over SSH to look at something: the
first sample lands in 250 ms, and block art cannot carry the look).

Animation (default 15 fps): values ease out over 600 ms after each sample,
a light sweeps each title every 7 s (staggered), stars twinkle in the gaps.
The backdrop and windows are a cached base layer; a tile is restored and
redrawn only when its cell or shimmer changed, stars twinkle in 6 steps, and
only changed spans are copied to the framebuffer. Layout reruns only during a
tween or when the clock second ticks. Idle frames cost ~0.1 ms at any scale.

## Themes

`phosphor` (amber CRT), `gizzard` (default, psychedelic), `lantern` (soft
pastel), `chrono` (blue RPG windows), `arcane` (violet, mystical), `crimson`
(red on black), `sylvan` (forest: canopy-to-loam windows, firefly gold,
mushroom teal, foxglove violet). Each theme is a background, text tones,
a value ramp, one accent per panel, and optionally explicit window colors.
Text holds WCAG contrast on panels and window gradients (`TestThemeContrast`:
FG 4.5:1, Dim 3:1). `sylvan` is also picked for the bare VT: its text and
eight accents land on eight different colors of the 16.

`nonagon` (after Nonagon Infinity, which loops into itself) is a mode, not
a palette: it holds each theme for 60 s, then cross-fades to the next over
3 s, forever. A glance lands mid-fade 5% of the time, and panel hues hold
still while you work. The fade runs in OKLCh (lightness, chroma, hue each
move evenly), so red to green passes through amber, not gray; window
gradients blend in OKLab. It is quantized to 48 eased steps, each one
`*Theme` cached for as long as it shows: no color moves more than ~0.02
OKLab (about a just-noticeable difference) per step, the renderers cache it
like any theme, and the set of colors ever drawn stays finite. A step
changes every pixel, so graphics mode redraws and blits the whole frame:
~8.7 ms at 3440x1440 during the 3 s fade, idle frames unchanged; the
margins outside the tile grid follow (`ClearMargins`). The terminal redraws
at most every 100 ms during a fade and sleeps until the next one between.
With `-fps 0` themes switch at the end of each dwell without fading. `t`
reaches `nonagon` after the last theme and starts the fade to the first at
once.

CPU also gets a per-core heatmap: one strip per core over time, two cores per
row via upper half blocks (fg = top core, bg = bottom core).

## Later

- Unheaded panels: BGP sessions, eBPF/Shield counters, Wotan, The Well,
  log tail with severity color.
- Autostart graphics mode on the desk VT (systemd unit or login hook).
- DRM/KMS backend and hosting other binaries fullscreen: [ADR-0001](adr/0001-native-kiosk-renderer.md).

Ideas, undecided:

- Alerts as RPG encounters: a threshold crossing flashes the screen and
  opens a dialog; header bars as HP/MP gauges.
- Monitor power: blank the desk monitor at night (DPMS), wake it on an alert.
- Overnight page: a 24 h in-memory sample ring, day-long sparklines with
  spike and restart markers.
