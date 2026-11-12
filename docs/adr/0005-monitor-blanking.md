# ADR-0005: Monitor blanking and the input helper

Status: Accepted
Date: 2026-11-12

## Context

Stevie wants the desk monitor off after five minutes without input and
back on at a key press or a mouse move. orb in graphics mode holds its VT
in KD_GRAPHICS, where the kernel's own console blanker (`consoleblank=`)
does not run, so nothing else will do it.

Two inputs matter. Keys already reach orb as bytes on the console. The
mouse does not: a VT carries no pointer. The only place orb can learn
that the mouse moved is the kernel's event devices, `/dev/input/event*`,
readable by root and the `input` group.

An event descriptor is a strong thing to hold. It delivers every event of
its device whichever VT is in front, so a keyboard's descriptor reads
passwords typed at a login prompt on another console. orb's main process
is the one that fetches and parses feeds; ADR-0002 drops it to the
invoking user before any of that precisely so a parsing bug gains little.
Opening the event devices as root in that process and keeping them across
the drop would hand such a bug a keylogger.

## Decision

- `-blank <duration>` (config `blank`), graphics mode only, 0 (the
  default) for never, otherwise 10 s to 24 h. Off by default: the
  dashboard exists to be glanced at, so sleeping is the host owner's
  choice.
- Blanking is FBIOBLANK with FB_BLANK_POWERDOWN on the framebuffer orb
  already has open (DPMS off through the DRM fbdev emulation); waking is
  FB_BLANK_UNBLANK and a full redraw. A driver that refuses gets a black
  screen instead. The monitor is woken on every exit and before a VT
  switch away.
- `gfx.Blanker` is a pure state machine fed input times and ticks. The
  key that wakes the screen does nothing else, and neither does a key
  within 0.5 s of the wake-up: one press arrives twice, as an input event
  and as a console byte, and `q` must not quit a dashboard someone only
  meant to wake.
- A third helper, the same shape as the other two: a root orb re-executes
  itself (`/proc/self/exe -input-helper`) with an empty environment in an
  empty network namespace. The helper opens the event devices that have
  keys/buttons or relative axes (keyboards, mice; not lid switches or
  accelerometers), each checked to be an evdev character device before it
  is opened, then drops to nobody with **no** capabilities and
  no_new_privs. It reads events only to throw them away and writes one
  `.` to its pipe per burst of activity, at most ten a second per device.
- The main process takes nothing from the helper but those ticks; any
  other byte ends the helper. It never opens an input device as root.
- Not root: no helper. orb opens the event devices itself if its user may
  (the `input` group); there is no privilege to separate, since that user
  could read them anyway. If it may not, only the keyboard wakes the
  monitor.
- Devices are opened once, at start. A mouse plugged in later is not
  seen until orb restarts; the keyboard wakes the monitor regardless,
  through the console.

## Verification

`make test-blank` (real root on a console host; `scripts/test-blank.py`)
creates a uinput mouse and keyboard, runs orb under strace with
`-blank 10s` and checks: the helper is uid nobody, CapEff 0, no_new_privs,
in its own network namespace, and the only process holding input devices
(the main process holds none and is not root); no blank before the idle
time; mouse movement postpones it; FBIOBLANK powerdown returns 0 after
10 s idle; mouse movement wakes; it blanks again; `q` wakes without
quitting; a second `q` quits with status 0. Live on west (amdgpudrmfb),
2026-11-12: all pass, with no monitor attached at the time, so the DPMS
effect on a real panel is still to be seen by eye. `Blanker`, the
watcher, the device filter and the tick pump have unit tests.

## Found on a real monitor, the same day

The first test ran with no monitor attached. With one (a DisplayPort
ultrawide), `-blank 1m` powered the panel down and 18 seconds later it was
lit again and stayed lit, showing the last frame: Stevie's test ran twelve
minutes without staying dark. No input device had spoken. A monitor going
into deep sleep drops its link; amdgpu takes that for a hotplug and the
fbdev emulation switches the output back on. orb still held itself to be
blanked, so it neither drew nor blanked again.

- A blanked orb now asks the kernel on every tick whether a monitor of its
  framebuffer is lit (`FB.Lit`: sysfs, the connectors under the
  framebuffer's DRM device, `status` connected and `dpms` On) and, if so,
  powers it down again. Nobody having touched anything is known: input
  would have woken orb through the other path.
- Measured on west: off after the idle time, relit by the kernel once 18 s
  later, down again within a frame (not visible at five samples a second),
  then dark for as long as watched (148 s in one run). The monitor drops
  its link once, not repeatedly.
- `make test-blank` did not catch this and still would not without a
  monitor: its blanks are shorter than 18 s. Checking a real panel means
  watching `/sys/class/drm/card*-*/dpms` for a few minutes.

## Consequences

The process that reads keyboards can do nothing but write dots to a pipe,
and the process that parses hostile text never sees a keystroke it was
not sent by the console. What leaks through the pipe is timing: that
someone is at the desk, to a tenth of a second. The main process needs
that to do the job at all.

Hot-plugged pointers are not watched; a udev monitor would need a netlink
socket in the helper and is not worth it for a desk.
