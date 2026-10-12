# nocterm

View-only host health dashboard for the terminal. Built for a big monitor across the desk, works the same over SSH.

```bash
nocterm                     # live dashboard in the terminal
nocterm -gfx                # pixel graphics on the local console (no desktop)
nocterm -page news          # start on news & weather instead of the dashboard
nocterm -theme chrono       # phosphor (default), gizzard, lantern, chrono
nocterm -interval 2s        # sample every 2s
nocterm -demo               # synthetic data
nocterm -once 215x45        # print one frame and exit
nocterm -gfx -once 3440x1440 > frame.png
```

Keys: `d` dashboard, `w` news & weather, `t` next theme, `q` quit.

## Configuration

`~/.config/nocterm/config` (or `-config`), `key = value`, `#` comments.
Flags override it. Keep it `chmod 600`: nocterm warns otherwise, since feed
URLs may be private.

```
theme = phosphor            # phosphor, gizzard, lantern, chrono
page = dash                 # dash or news
interval = 1s
scale = 4                   # graphics mode
fps = 15                    # graphics mode, 0 = no animation
news_url = https://example.com/news.json
weather_url = https://example.com/weather.json
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
nocterm -gfx -scale 4       # pixel size, 1..8 (4 = 107x45 tiles on 3440x1440)
nocterm -gfx -fps 0         # no animation, redraw once per sample
```

Needs membership in the `video` group and must start from the monitor's own
VT, not SSH. Ctrl+Alt+F-key switches work; nocterm pauses while away.
If a killed process leaves the console blank:

```bash
nocterm -reset-console -              # typed blind on that console
sudo nocterm -reset-console /dev/tty1 # or from SSH
```

## Install

```bash
git clone <repo> && cd nocterm
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
hourly temperature curve, five-day range bars, and a reader that tells one
story at a time like an RPG dialog box: headline, then the summary typed out
at reading pace, a blinking ▼ when another page follows. Stories alternate
between categories, newest first. Below it, six headline windows rotate
every 15 seconds. Summaries come only from the feed's own `description`
field; nocterm never fetches the articles.

Layout adapts to the size: two rows of three columns at 180+ cells wide, three rows of two at 100+, a single stack below that.

## Development

```bash
make test      # race-enabled tests
make lint      # go vet + staticcheck
make demo      # run against synthetic data
```

Design notes: [docs/DESIGN.md](docs/DESIGN.md). Decisions: [docs/adr/](docs/adr/).
