# ADR-0006: Listening to the airwaves

Status: Accepted (Wi-Fi); Bluetooth open
Date: 2026-11-24

## Context

Stevie wants a page, on key `a`, that "checks the airwaves via any wifi or
bluetooth and shows it - raw radio listener no connection". orb must never
join a network or transmit. The hosts differ:

- west has a MediaTek Wi-Fi card (`wlp17s0`), which is its only uplink, and
  a Bluetooth controller (`hci0`, bluetoothd stopped).
- east has a Ralink card (`wlp7s0`, rt2800pci) that was down, and no
  Bluetooth. Stevie had it brought up for listening only (2026-11-24).
  `/etc/systemd/network/20-wlp7s0-listen.network` keeps it up with no
  address, no DHCP, no link-local and no router discovery, and
  `/etc/sysctl.d/60-wlp7s0-listen.conf` turns IPv6 off on it, so it sends
  nothing.

His answers (2026-11-24): read what the radio has already heard by default,
with passive scans opt-in; show full addresses; the README and demo data
stay synthetic.

## Decision

**Reading is unprivileged.** The main process asks nl80211, over generic
netlink and with no privilege, for the Wi-Fi interfaces and for each one's
scan results. Scan results are the kernel's list of networks it has heard
beacons from (`internal/radio`, `radio.Heard`). The decoder reads beacon
information elements, which are bytes from whoever is in range. It checks
every length, keeps only printable ASCII in names (32 bytes at most) and
stops after a fixed number of attributes and elements; `FuzzParseBSS` runs
over it. A table keeps each network for 10 minutes after it was last heard
(the kernel drops one after about 30 s), with a cap of 1024 networks.

**Scanning is opt-in and only listens.** With `radio_scan = 60s` (flag
`-radio-scan`, 30s..1h; 0, the default, is off) and orb started as root, a
third helper does the scanning:

- orb re-executes itself (`-radio-helper <interval>`) with an empty
  environment, in the host network namespace.
- It drops to nobody keeping only `CAP_NET_ADMIN`, with no_new_privs.
  nl80211 checks that capability on every scan request, on both the socket
  and the caller, so it cannot be dropped after opening a socket the way the
  LLDP helper drops (ADR-0003).
- Every interval it asks each station interface that is up for a scan. The
  request names no SSID, which makes it passive: the radio sends no probe
  requests, it only listens on each channel for beacons. It never brings a
  down interface up and never touches an access-point or monitor interface.
- It reads nothing from the air. Its only input is the kernel's interface
  list, and its output is one JSON line per request, `{"if", "at", "err"}`,
  where err is one word from a fixed set. The main process parses those
  lines strictly; a malformed line ends the helper.

A radio that is also a link (west's uplink) briefly leaves its channel while
it listens. That is the reason scanning is off by default.

**Bluetooth is not built.** Passive Low Energy listening needs a raw HCI
socket. Keeping a scan running means the helper keeps `CAP_NET_RAW`, and
the advertising reports it decodes come from anyone in range. The other
choices are the monitor channel, which needs no capability but only sees
advertisements while something else (bluetoothd) scans, or Wi-Fi only. The
choice is Stevie's, asked 2026-11-24.

## Verification

- **Tests:** the decoders, the table and the helper's line protocol have
  unit tests. `FuzzParseBSS` ran 32 million inputs clean on first run. The
  page renders at every size and every theme (`TestRenderAnySize`), and
  every rune it draws has a glyph (`TestEveryDashboardRuneSupported`).
- **Live on west, not root:** the reader sees `wlp17s0` and the network it
  has joined, on channel 157.
- **Live on east, as root with `-radio-scan 30s`:** the helper's passive scan
  on `wlp7s0` heard two 2.4 GHz networks. The interface's transmit counter
  stayed at 0 packets.

## Consequences

The page shows nearby networks by name and address. That is Stevie's own
screen; the demo data invents names and uses locally administered
addresses (`02:...`), so screenshots and the reel stay free of real ones.
A long-running helper holds `CAP_NET_ADMIN`; its only input is the kernel,
and it never reads anything from the air.
