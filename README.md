# nocterm

View-only host health dashboard for the terminal. Built for a big monitor across the desk, works the same over SSH.

```bash
nocterm                     # live dashboard in the terminal
nocterm -gfx                # pixel graphics on the local console (no desktop)
nocterm -theme chrono       # phosphor (default), gizzard, lantern, chrono
nocterm -interval 2s        # sample every 2s
nocterm -demo               # synthetic data
nocterm -once 215x45        # print one frame and exit
nocterm -gfx -once 3440x1440 > frame.png
```

Keys: `t` next theme, `q` quit.

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

Layout adapts to the size: two rows of three columns at 180+ cells wide, three rows of two at 100+, a single stack below that.

## Development

```bash
make test      # race-enabled tests
make lint      # go vet + staticcheck
make demo      # run against synthetic data
```

Design notes: [docs/DESIGN.md](docs/DESIGN.md). Decisions: [docs/adr/](docs/adr/).
