# nocterm

View-only host health dashboard for the terminal. Built for a big monitor across the desk, works the same over SSH.

```bash
nocterm                     # live dashboard
nocterm -theme lantern      # gizzard (default), lantern, phosphor
nocterm -interval 2s        # sample every 2s
nocterm -demo               # synthetic data
nocterm -once 215x45        # print one frame and exit
```

Keys: `t` next theme, `q` quit.

## Install

```bash
git clone <repo> && cd nocterm
make build && sudo make install
```

Requires: Linux, Go 1.27+ to build. Truecolor terminal recommended; the bare Linux console falls back to 16 colors.

## Panels

| Panel | Source |
|-------|--------|
| Header | hostname, OS, kernel, uptime, load, clock, cpu/mem/disk/temp at a glance |
| CPU | `/proc/stat` per-core bars + per-core heatmap + total history, `cpufreq` |
| Memory | `/proc/meminfo` used, cache, swap + history |
| GPU | amdgpu sysfs: busy, VRAM/GTT, power, clock, temp + history |
| Network | `/proc/net/dev` per interface, rx/tx mirrored graph |
| Disks | `/proc/mounts` + `statfs` usage, `/proc/diskstats` read/write |
| Thermal | hwmon temperatures and fans |
| Processes | `/proc/[pid]/stat` top by CPU |

Layout adapts to the terminal: four columns at 180+ cells wide, three rows of two to three panels at 100+, a single stack below that.

## Development

```bash
make test      # race-enabled tests
make lint      # go vet + staticcheck
make demo      # run against synthetic data
```

Design notes: [docs/DESIGN.md](docs/DESIGN.md). Decisions: [docs/adr/](docs/adr/).
