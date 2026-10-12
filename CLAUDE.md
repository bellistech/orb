# nocterm — terminal host dashboard (binary `nocterm`, name not final)

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
make build          # ./nocterm
make test           # go test ./... -count=1 -race
make lint           # go vet + staticcheck
make fmt            # gofmt -s -w .
make demo           # run with synthetic data
./nocterm -once 215x45 [-demo] [-theme lantern]   # print one frame and exit
```

## Architecture

- `cmd/nocterm/main.go` — entry point, stdlib `flag`
- `internal/sys/` — metric readers (`/proc/stat`, `meminfo`, `net/dev`, `diskstats`, `mounts`, hwmon, amdgpu sysfs, `/proc/[pid]/stat`), `Sampler` turns counters into rates + 600-sample histories, `Demo` synthesizes a busy host
- `internal/canvas/` — cell grid (rune + fg + bg + bold), clipped regions, truecolor SGR serializer
- `internal/ui/` — themes, widgets (frame, gauge, braille graph, heatmap, block-font clock), panels, responsive grid. Pure: snapshot in, canvas out
- `internal/tui/` — Bubble Tea v2 program: ticks the sampler, renders, handles resize and keys

Rendering is deliberately decoupled from output: `ui.Render` returns a canvas, and only `tui` knows about terminals. A future pixel backend (ADR-0001) would consume the same panels.

## Conventions

- Go 1.27.2 (toolchain pinned in `go.mod`); compile against latest
- Deps: bubbletea v2, go-colorful. Nothing else without a reason
- No cobra (stdlib `flag`), no logging framework (stderr)
- Build flags: `-trimpath -s -w`, version via `-X main.version=$(VERSION)`
- Readers are best-effort: a missing file is an empty value, never an error or panic
- Exactly one sample in flight; the next tick is scheduled when a sample lands
- New panels: a `drawFn` in `internal/ui/panels.go`, a slot in `grid()` per breakpoint, demo data in `sys.Demo`
- `TestRenderAnySize` must stay green: every panel draws into any box and clips
- Check visuals with `-once` at 215x45 (desk), 120x40, 80x60 before calling a UI change done
