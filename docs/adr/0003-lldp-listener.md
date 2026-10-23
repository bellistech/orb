# ADR-0003: Hearing LLDP neighbors without lldpd

Status: Accepted
Date: 2026-10-23

## Context

The links page shows who is on the other end of each link. LLDP (IEEE
802.1AB) is how switches, routers and hosts announce that: every 30 s an
LLDPDU to 01:80:c2:00:00:0e with chassis, port, name, description,
capabilities and management addresses. Stevie wants LLDP on both hosts
(east runs lldpd from 2026-10-23) and orb to read it off the wire itself
rather than through lldpd's socket: orb stays one binary with no daemon to
depend on, and it works on any host where something nearby speaks LLDP.

Receiving the frames needs a packet socket, which needs CAP_NET_RAW, and
every byte comes from whatever is on the link.

## Decision

A second helper, the same shape as the socket helper (ADR-0002):

- orb re-executes itself (`/proc/self/exe -lldp-helper`) while still root,
  with an empty environment, in the host network namespace (it must see
  the links).
- The helper opens one AF_PACKET SOCK_DGRAM socket for ethertype 0x88cc
  on all interfaces and joins the LLDP group on each physical one (not
  loopback, veths or container bridges), so NICs that filter multicast pass
  the frames. Then it drops to nobody keeping **no** capabilities, with
  no_new_privs: reading an open socket needs none, and changing its
  memberships for interfaces that appear later needs none either.
- It decodes frames itself (`internal/lldp`): the first three TLVs must be
  chassis, port, TTL; every length is checked; strings become printable
  ASCII, capped; at most 64 frames a second are decoded. It writes one
  JSON line per neighbor to its parent.
- The main process re-validates each line (`lldp.Neighbor.Check`, bounded
  line length, interface name charset) and keeps a table that drops a
  neighbor when its TTL runs out or it sends TTL 0. A malformed line ends
  the helper.
- Not root: no helper, and the LLDP pane says it needs root.

## Verification

`make test-priv` opens the socket as root of a user namespace with its own
network namespace, drops, and checks that no capability is left, that a new
packet socket is refused and that the open one still works; skipping the
drop turns it red. The decoder, the pipe protocol and the table have unit
tests and fuzz targets (a fuzz run found that an all-blank chassis ID
decoded to an empty one; Decode now refuses it). Live on west: the helper
runs as nobody with CapEff 0 and one socket; it hears east (lldpd on
enp2s0, over the P2P link) and the home router on Wi-Fi.

## Consequences

Parsing hostile frames happens in a process that can do nothing but write
to its pipe. orb sends nothing: it hears LLDP, it does not announce (west
needs lldpd for that).
