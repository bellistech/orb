# orb

View-only host health dashboard for the terminal and the bare Linux console. Built for a big monitor across the desk; works the same over SSH. One static binary, no agents.

![orb in graphics mode: the dashboard in all six themes, then news & weather, network and links pages](docs/media/orb.webp)

<sub>`orb -gfx -demo` (synthetic data), 143x60 tiles: the layout of a 3440x1440 monitor at `-scale 3`. Dashboard in phosphor, gizzard, lantern, chrono, arcane and crimson, then news & weather, network and links.</sub>

```bash
orb                     # live dashboard in the terminal
orb -gfx                # pixel graphics on the local console (no desktop)
orb -page news          # start on news & weather (or -page net, -page links)
orb -theme arcane       # phosphor, gizzard (default), lantern, chrono, arcane, crimson
orb -interval 2s        # sample every 2s
orb -demo               # synthetic data
orb -once 215x45        # print one frame and exit
orb -gfx -once 3440x1440 > frame.png
```

## Pages and keys

| Key | Page | Shows |
|-----|------|-------|
| `d` | Dashboard | CPU, memory, GPU, network, disks, thermal, processes |
| `w` | News & weather | headline windows, a story reader, current conditions, hourly curve, five-day ranges |
| `n` | Network | listening sockets, active connections, socket states, interfaces |
| `l` | Links | interfaces as ports, bridges, LLDP neighbors, ARP/NDP, routes |

`t` next theme, `space` pause/resume the news reader, `↑`/`←` previous story, `↓`/`→` next story, `q` quit.

## Graphics mode

`-gfx` draws straight to the Linux framebuffer (`/dev/fb0`) from a text
console login, no desktop or compositor: 8x8 tiles scaled up with hard pixel
edges, a hand-drawn pixel font, and windows styled after 16-bit console RPG
menus. A crystal orb swirls in the header, sparkles glint along window
edges, values glide between samples, titles shimmer, letters glow as the
reader types them, stars twinkle behind the windows.

```bash
orb -gfx -scale 3       # pixel size, 1..8 (3, the default = 143x60 tiles on 3440x1440)
orb -gfx -fps 0         # no animation, redraw once per sample
```

Start it from the monitor's own console login (needs the `video` group),
or as root from anywhere, including SSH: `sudo orb -gfx` takes a free
console, switches the monitor to it, and switches back when you quit.
Ctrl+Alt+F-key switches work; orb pauses while away. `q` or Ctrl+C quits.

If a killed process leaves the console blank:

```bash
orb -reset-console -              # typed blind on that console
sudo orb -reset-console /dev/tty1 # or from SSH; then sudo chvt 1
```

## Panels

| Panel | Source |
|-------|--------|
| Header | hostname, OS, kernel, uptime, load, clock, cpu/mem/disk/temp at a glance |
| CPU | `/proc/stat` per-core bars + per-core heatmap + total history, `cpufreq` |
| Memory | `/proc/meminfo` used, cache, swap + history |
| GPU | amdgpu sysfs: busy, VRAM/GTT, power, clock, temp + history |
| Network | `/proc/net/dev` per interface with addresses, rx/tx mirrored graph |
| Disks | `/proc/mounts` + `statfs` usage, `/proc/diskstats` read/write |
| Thermal | hwmon temperatures and fans, hottest-sensor history |
| Processes | `/proc/[pid]/stat` top by CPU |

Layout adapts to the size: two rows of three columns at 180+ cells wide, three rows of two at 100+, a single stack below that.

**News & weather** (`w`): current conditions with animated weather art,
hourly temperature curve, five-day range bars, six headline windows that
rotate every 15 seconds, and below them a reader that tells one story at a
time like an RPG dialog box: headline, then the summary typed out at 40
characters a second, a 5 second pause after each page, a blinking ▼ when
another follows. Stories come in a random order, reshuffled with each news
fetch. Summaries come only from the feed's own `description` field; orb
never fetches the articles.

**Network** (`n`): listening sockets with port, service name (from
`/etc/services`), address and owning process, like `ss -tulpn`; active
connections with state, local and peer address and process; socket counts
by state with the top peers; interfaces with addresses and throughput. Read
from `/proc/net/{tcp,tcp6,udp,udp6}` and `/proc/<pid>/fd` every 2 seconds,
only while the page is up. Owning processes of other users show as `-`
unless orb runs as root, as with `ss -p`. No DNS lookups.

**Links** (`l`): layer 2 and 3 the way a switch or router shows it.
Interfaces as ports (state, kind, line speed, MTU, rx/tx, load against line
rate, error/drop totals), bridges with their member ports and learned MACs,
LLDP neighbors, the ARP/NDP table and the main routing table. Read from
`/sys/class/net` and rtnetlink dumps, only while the page is up. LLDP comes
from orb's own listener, so only when started as root
([ADR-0003](docs/adr/0003-lldp-listener.md)).

## Configuration

`~/.config/orb/config` (or `-config`), `key = value`, `#` comments.
Flags override it. Keep it `chmod 600`: orb warns otherwise, since feed
URLs may be private.

```
theme = gizzard             # phosphor, gizzard, lantern, chrono, arcane, crimson
page = dash                 # dash, news, net or links
interval = 1s
scale = 3                   # graphics mode
fps = 15                    # graphics mode, 0 = no animation
news_url = https://example.com/news.json
weather_url = https://example.com/weather.json
```

Feed URLs must be https (http only for localhost). News refreshes every 15
minutes, weather every 10; failures retry from 30s, backing off. Errors show
on screen in a few words and never include the URL.

## Privileges

Run as root to see every process on the network page and to hear LLDP. orb
does not stay root: a small helper keeps only what reading other processes'
sockets needs (as nobody, with no network), the LLDP helper drops to nobody
with no capabilities once its socket is open, and the main process drops to
you with no capabilities before it fetches anything. See
[ADR-0002](docs/adr/0002-privilege-separation.md). Under sudo, orb reads the
invoking user's `~/.config/orb/config`, only if it is a regular file
owned by that user (as root it follows no symlink there).

## Install

```bash
git clone https://github.com/bellistech/orb.git && cd orb
make build && sudo make install
```

Requires: Linux, Go 1.27+ to build. The binary is static (no libc). Truecolor terminal recommended; on the bare Linux console use `-gfx`.

## Development

```bash
make test      # race-enabled tests
make lint      # go vet + staticcheck
make test-priv # the root drop for real, inside a user namespace
make demo      # run against synthetic data
```

Design notes: [docs/DESIGN.md](docs/DESIGN.md). Decisions: [docs/adr/](docs/adr/).

## License

Copyright (C) 2026 Stevie Bellis. GPL-3.0-or-later; see [LICENSE](LICENSE).
