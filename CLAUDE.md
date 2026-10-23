# orb — host dashboard for the terminal and the Linux console (binary `orb`)

View-only system health dashboard for the terminal. One Go binary, same view over SSH and on the desk monitor (3440x1440 ultrawide). Host metrics from /proc and /sys today; Unheaded data (BGP, eBPF, app logs, The Well) later as more panels.

## North Star

> **Glance at the monitor from across the desk and know how the box is doing — without reading.**

- **Color carries meaning.** One accent hue per panel; one cool -> hot ramp for every value. Bars and graphs color by position, so "reaches pink" means hot everywhere.
- **Big and few.** Big clock, bold numbers, short labels. If it needs squinting it failed.
- **History over snapshots.** Every rate gets a graph.
- **View only.** No interaction required. Keys exist only for SSH use: `t` theme, `q` quit.
- **Light.** No agents, no plugins, no runtime deps beyond the binary.

See `docs/DESIGN.md` for layout, themes and targets. Decisions go in `docs/adr/`.

## Build

```bash
make build          # ./orb
make test           # go test ./... -count=1 -race
make lint           # go vet + staticcheck
make test-priv      # real privilege drops + end-to-end, in a user namespace
make fmt            # gofmt -s -w .
make demo           # run with synthetic data (both pages)
./orb -gfx -demo -once 3440x1440 > f.png      # graphics-mode frame as PNG
./orb -once 215x45 [-demo] [-theme lantern]   # print one frame and exit
```

## Architecture

- `cmd/orb/main.go` — entry point, stdlib `flag`
- `internal/sys/` — metric readers (`/proc/stat`, `meminfo`, `net/dev`, `diskstats`, `mounts`, hwmon, amdgpu sysfs, `/proc/[pid]/stat`; sockets from `/proc/net/*` + fd inode owners, only while `Sampler.WantSockets` is set; links from `/sys/class/net` + rtnetlink dumps of neighbors, routes and the bridge FDB, only while `Sampler.WantLinks` is set), `Sampler` turns counters into rates + 600-sample histories, `Demo` synthesizes a busy host
- `internal/canvas/` — cell grid (rune + fg + bg + bold), clipped regions, truecolor SGR serializer
- `internal/ui/` — themes, widgets (frame, gauge, braille graph, heatmap, block-font clock, range bar), pages (`Dash`, `News`, `Net`, `Links`), panels, responsive grid, weather sprites, attract mode (`Attract`, a state machine both front ends step). Pure: `ui.View` in, canvas out
- `internal/feeds/` — news + weather JSON: fetch (UA `orb/<version>`, https only, 8 MiB cap, redirects re-checked), parse + validate, `Clean` (HTML unescape, transliterate, printable ASCII only, emoji dropped; fuzzed), `Poll` with backoff, `Store` (RWMutex snapshot + version), `Brief` (screen-safe errors, never the URL), `Demo`
- `internal/config/` — `~/.config/orb/config` key = value, strict (unknown keys, duplicates, bad values rejected), warns unless mode 600; flags override; as root only via `LoadOwned` (regular file owned by the invoking user, no symlink, checked before open)
- `internal/priv/` — privilege separation (ADR-0002): `Drop` (all-thread uid/gid/caps/no_new_privs, verified per thread), socket-owner helper (`-sock-helper`, nobody + CAP_DAC_READ_SEARCH + CAP_SYS_PTRACE, empty netns) and its pipe protocol; LLDP helper (`-lldp-helper`, ADR-0003: opens the packet socket as root, then nobody with no caps, host netns, JSON lines)
- `internal/lldp/` — LLDPDU decoder (TLVs, printable-only strings, fuzzed), neighbor table with TTL expiry, packet-socket listener (`listen_linux.go`)
- `internal/tui/` — Bubble Tea v2 program: ticks the sampler, renders, handles resize and keys
- `internal/gfx/` — graphics mode: hand-drawn 5x7 font in 8x8 tiles (2x via `Cell.Quad`), procedural blocks/braille, RPG-style windows (from `canvas.Windows`), overlays (orb from `canvas.Orbs`, sparkles), animation (tween via `sys.Lerp`, title shimmer, stars), `ParseKeys`, fbdev driver with validated geometry and dirty-row blits, VT control (KD_GRAPHICS, VT_PROCESS switching, raw keys, restore on every exit), PNG output

Rendering is decoupled from output: `ui.Render` returns a canvas; `tui` turns it into terminal output, `gfx` into pixels (ADR-0001). Panels never know which.

## Conventions

- Go 1.27.3 (toolchain pinned in `go.mod`); compile against latest
- License GPL-3.0-or-later (`LICENSE`); no per-file license headers
- Deps: bubbletea v2, go-colorful, golang.org/x/sys. Nothing else without a reason; graphics mode is stdlib + x/sys only
- `unsafe` only for ioctl struct pointers in `gfx/*_linux.go`; every kernel-reported size is validated (`checkLayout`) before indexing
- No cobra (stdlib `flag`), no logging framework (stderr)
- Build flags: `CGO_ENABLED=0 -trimpath -s -w` (static, no libc), version via `-X main.version=$(VERSION)`
- As root, nothing network- or feed-related may run before the main process drops (`startFeeds` after `drop`); never add root work after it
- Ubuntu 25.10's sudo is sudo-rs, which always runs commands in a pty: `sudo orb -gfx` goes through the free-VT path, never `/dev/tty`
- Readers are best-effort: a missing file is an empty value, never an error or panic
- Everything shown is untrusted text (process names, feeds): the canvas turns control/format runes into `?` in `Set`; never bypass it
- No network lookups for display (no reverse DNS); service names come from `/etc/services`
- Feed content is hostile: every string through `feeds.Clean`, every number range-checked, nothing from a feed reaches the terminal unsanitized
- The news URL is private: it lives only in the user's config file, never in the repo, docs, tests or on screen
- Never fetch article pages or anything beyond the two configured JSON feeds; summaries come from the feed's `description` only
- Exactly one sample in flight; the next tick is scheduled when a sample lands
- One sampler, built in main and handed to both front ends (`tui.New` and `gfx.Run` take its source): socket demand and the root helper hang off it
- New panels: a `drawFn` in `internal/ui/` (dashboard panels wrap with `dash()`), a slot in `grid()`/`newsGrid()` per breakpoint, demo data in `sys.Demo` / `feeds.Demo`
- `TestRenderAnySize` must stay green: every panel draws into any box and clips
- Check visuals with `-once` at 215x45 (desk), 120x40, 80x60, and `-gfx -once 3440x1440 > f.png`, before calling a UI change done
- Every rune a panel draws needs a glyph or shape in `gfx` (`TestEveryDashboardRuneSupported`)
- The incremental renderer must equal a full render after any frame sequence (`TestIncrementalMatchesFullRender`); provoke it red when touching tile diffing or overlays
- Run test pipelines with `set -o pipefail`: `go test | grep` hides failures otherwise
- Keep graphics frames cheap: `go test -bench "AnimatedFrameBlit|TweenFrame|NewsFrame" ./internal/gfx` (idle ~0.1 ms, tween ~0.4 ms, news ~0.15 ms). The base layer (backdrop + windows) is cached; tiles redraw only when their cell changes; blits copy changed spans only
