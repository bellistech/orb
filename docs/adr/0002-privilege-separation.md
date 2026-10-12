# ADR-0002: Privilege separation when run as root

Status: Accepted
Date: 2026-10-12

## Context

The network page shows which process owns each socket. For other users'
processes that means reading `/proc/<pid>/fd`, which needs root. Graphics
mode started with sudo also needs root to take a console. But the same
process fetches and parses feed JSON from the network, and a bug there must
not be a root bug.

The first plan, keeping only CAP_SYS_PTRACE, does not work and would not
help: listing `/proc/<pid>/fd` is a plain DAC check (CAP_DAC_READ_SEARCH);
reading the links is a ptrace check (CAP_SYS_PTRACE). Both are effectively
root: one reads any file, the other can attach to any process.

## Decision

Split the privilege (OpenSSH-style):

- **Helper**: orb re-executes itself (`/proc/self/exe -sock-helper`) while
  still root, in a new, empty network namespace with an empty environment.
  It drops to nobody keeping exactly CAP_DAC_READ_SEARCH and
  CAP_SYS_PTRACE, and answers one request over a pipe: socket inode ->
  pid and command name. It never touches the network, a feed or the screen,
  and exits when the pipe closes.
- **Main process**: drops to the invoking user (`SUDO_UID`/`SUDO_GID`, or
  nobody) with no capabilities, except CAP_SYS_TTY_CONFIG when it took a
  free console, which the kernel requires to give the console back. Feeds
  start only after the drop. In graphics mode the drop happens right after
  the framebuffer and console are open, before the first frame.
- **The drop** (`internal/priv`): PR_SET_KEEPCAPS, bounding set reduced to
  the kept caps, groups cleared, setresgid/setresuid, capset, ambient
  cleared, no_new_privs, all on every OS thread (AllThreadsSyscall; needs
  the static CGO_ENABLED=0 build). Then every thread's `/proc` status is
  checked; any mismatch is fatal.
- Replies from the helper are parsed strictly (bounded lines, counts and
  fields; fuzzed). If the helper dies, orb falls back to the unprivileged
  scan.

## Verification

`make test-priv` runs the real drops as root of a user namespace (one
process per role) and an end-to-end check: a listener owned by namespace
root shows its process when orb starts as root and drops, and does not when
orb runs unprivileged. Sabotaging the capset step turns all three red.

## Consequences

A compromise of the main process gets the invoking user's privileges, not
root's. The helper's attack surface is one fixed request on a pipe from its
parent.
