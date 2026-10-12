# orb

View-only host health dashboard for the terminal. Built for a big monitor across the desk, works the same over SSH.

```bash
orb                     # live dashboard in the terminal
orb -gfx                # pixel graphics on the local console (no desktop)
orb -page news          # start on news & weather (or -page net)
orb -theme chrono       # phosphor (default), gizzard, lantern, chrono
orb -interval 2s        # sample every 2s
orb -demo               # synthetic data
orb -once 215x45        # print one frame and exit
orb -gfx -once 3440x1440 > frame.png
```

Keys: `d` dashboard, `w` news & weather, `n` network, `space` pause/resume the news reader,
`↑`/`←` previous story, `↓`/`→` next story, `t` next theme, `q` quit.

## Configuration

`~/.config/orb/config` (or `-config`), `key = value`, `#` comments.
Flags override it. Keep it `chmod 600`: orb warns otherwise, since feed
URLs may be private.

```
theme = phosphor            # phosphor, gizzard, lantern, chrono
page = dash                 # dash, news or net
interval = 1s
scale = 4                   # graphics mode
fps = 15                    # graphics mode, 0 = no animation
news_url = https://example.com/news.json
weather_url = https://example.com/weather_forecast.json
```

Feed URLs must be https (http only for localhost). News refreshes every 15
minutes, weather every 10; failures retry from 30s, backing off. Errors show
on screen in a few words and never include the URL.

## Graphics mode

`-gfx` draws straight to the Linux framebuffer (`/dev/fb0`) from a text
console login, no desktop or compositor: 8x8 tiles scaled up with hard pixel
edges, a hand-drawn pixel font, and windows styled after 16-bit console RPG
menus. Values glide between samples, titles shimmer, stars twinkle behind the
windows.

```bash
orb -gfx -scale 4       # pixel size, 1..8 (4 = 107x45 tiles on 3440x1440)
orb -gfx -fps 0         # no animation, redraw once per sample
```

Needs membership in the `video` group and must start from the monitor's own
VT, not SSH. Ctrl+Alt+F-key switches work; orb pauses while away.
If a killed process leaves the console blank:

```bash
orb -reset-console -              # typed blind on that console
sudo orb -reset-console /dev/tty1 # or from SSH
```

## Install

```bash
git clone <repo> && cd orb
make build && sudo make install
```

Requires: Linux, Go 1.27+ to build. Truecolor terminal recommended; on the bare Linux console use `-gfx`.

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

News & weather page (`w`): current conditions with animated weather art,
hourly temperature curve, five-day range bars, six headline windows that
rotate every 15 seconds, and below them a reader that tells one
story at a time like an RPG dialog box: headline, then the summary typed out
at 40 characters a second, a 5 second pause after each page, a blinking ▼ when another follows.
Stories come in a random order, reshuffled with each news fetch. Summaries come only from the feed's own `description`
field; orb never fetches the articles.

Network page (`n`): listening sockets with port, service name (from
`/etc/services`), address and owning process, like `ss -tulpn`; active
connections with state, local and peer address and process; socket counts
by state with the top peers; interfaces with addresses and throughput. Read
from `/proc/net/{tcp,tcp6,udp,udp6}` and `/proc/<pid>/fd` every 2 seconds,
only while the page is up. Owning processes of other users show as `-`
unless orb runs as root, as with `ss -p`. No DNS lookups.

Layout adapts to the size: two rows of three columns at 180+ cells wide, three rows of two at 100+, a single stack below that.

## Development

```bash
make test      # race-enabled tests
make lint      # go vet + staticcheck
make demo      # run against synthetic data
```

Design notes: [docs/DESIGN.md](docs/DESIGN.md). Decisions: [docs/adr/](docs/adr/).
