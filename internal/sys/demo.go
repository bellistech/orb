package sys

import (
	"math"
	"net/netip"
	"time"
)

// Demo returns a synthetic snapshot of a busy 12-thread box with full
// histories, for design work and screenshots. Nothing is read from the host.
func Demo(now time.Time) *Snapshot {
	t := float64(now.UnixMilli()) / 1000
	wave := func(i, phase, period, lo, hi float64) float64 {
		v := (math.Sin((t-float64(History)+i)/period+phase) + 1) / 2
		v = v*0.75 + 0.25*(math.Sin((t+i)*1.7+phase*3)+1)/2
		return lo + (hi-lo)*v
	}
	hist := func(phase, period, lo, hi float64) []float64 {
		out := make([]float64, History)
		for i := range out {
			out[i] = wave(float64(i), phase, period, lo, hi)
		}
		return out
	}
	last := func(v []float64) float64 { return v[len(v)-1] }

	s := &Snapshot{
		Time: now,
		Host: Host{
			Hostname: "west", OS: "Ubuntu 25.10", Kernel: "6.17.0-41-generic",
			CPUModel: "AMD Ryzen 5 7600X 6-Core Processor",
			Uptime:   3*24*time.Hour + 4*time.Hour + 12*time.Minute,
			Load:     [3]float64{7.82, 6.10, 4.95}, Running: 9, Tasks: 612,
		},
	}
	s.CPUHist = hist(0, 18, 0.05, 0.92)
	s.CPU = last(s.CPUHist)
	for i := range 12 {
		h := hist(float64(i)*0.9, 9+float64(i), 0.02, 1)
		s.CoreHist = append(s.CoreHist, h)
		s.Cores = append(s.Cores, last(h))
		s.CoreMHz = append(s.CoreMHz, 3000+1700*last(h))
	}
	const gib = 1 << 30
	s.MemHist = hist(1, 60, 0.35, 0.78)
	total := uint64(64 * gib)
	used := uint64(last(s.MemHist) * float64(total))
	s.Mem = Mem{Total: total, Used: used, Available: total - used, Cached: 18 * gib, Buffers: gib, SwapTotal: 8 * gib, SwapUsed: gib / 3}

	s.NetRx = hist(2, 7, 2e5, 9e7)
	s.NetTx = hist(4, 11, 5e4, 2.2e7)
	s.Net = []NetIf{
		{Name: "enp16s0", Up: true, RxRate: last(s.NetRx) * 0.9, TxRate: last(s.NetTx) * 0.8, Addrs: []string{"192.168.1.20", "2001:db8::20"}},
		{Name: "wg0", Up: true, RxRate: last(s.NetRx) * 0.1, TxRate: last(s.NetTx) * 0.2, Addrs: []string{"10.44.0.1"}},
		{Name: "wlp17s0", Up: false},
	}
	s.DiskRead = hist(3, 5, 0, 4.2e8)
	s.DiskWrite = hist(5, 13, 0, 1.6e8)
	s.Mounts = []Mount{
		{Path: "/", Size: 334 * gib, Used: 290 * gib, Avail: 44 * gib},
		{Path: "/var", Size: 1800 * gib, Used: 610 * gib, Avail: 1190 * gib},
		{Path: "/boot", Size: 2 * gib, Used: gib / 4, Avail: 7 * gib / 4},
		{Path: "/srv/backup", Size: 3600 * gib, Used: 3400 * gib, Avail: 200 * gib},
	}
	s.DiskIO = []DiskIO{
		{Name: "nvme0n1", ReadRate: last(s.DiskRead), WriteRate: last(s.DiskWrite)},
		{Name: "sda", ReadRate: last(s.DiskRead) / 9, WriteRate: 0},
	}
	s.Temps = []Temp{
		{Chip: "cpu", Label: "Tctl", Celsius: 40 + 50*s.CPU},
		{Chip: "cpu", Label: "Tccd1", Celsius: 36 + 46*s.CPU},
		{Chip: "gpu", Label: "edge", Celsius: 58},
		{Chip: "gpu", Label: "junction", Celsius: 71},
		{Chip: "gpu", Label: "mem", Celsius: 66},
		{Chip: "nvme", Label: "Composite", Celsius: 47},
		{Chip: "dimm", Celsius: 41},
	}
	s.Fans = []Fan{{Chip: "gpu", RPM: 1480}}
	s.TempHist = make([]float64, History)
	for i, v := range s.CPUHist {
		s.TempHist[i] = 40 + 50*v
	}
	g := hist(6, 6, 0.1, 0.98)
	s.GPUs = []GPU{
		{Card: "card1", Discrete: true, Busy: last(g), VRAMUsed: 9 * gib, VRAMTotal: 12 * gib, PowerW: 40 + 180*last(g), SclkMHz: 2450, TempC: 58},
		{Card: "card0", Busy: 0.04, GTTUsed: gib, GTTTotal: 7 * gib, PowerW: 12, SclkMHz: 600, TempC: 44},
	}
	s.GPUHist = [][]float64{g, hist(7, 30, 0, 0.1)}
	ap := netip.MustParseAddrPort
	sock := func(proto, local, remote, state string, pid int, proc string) Socket {
		return Socket{Proto: proto, Local: ap(local), Remote: ap(remote), State: state, PID: pid, Proc: proc}
	}
	s.Sockets = []Socket{
		sock("tcp", "0.0.0.0:22", "0.0.0.0:0", "LISTEN", 1873, "sshd"),
		sock("udp", "0.0.0.0:53", "0.0.0.0:0", "UNCONN", 812, "unbound"),
		sock("tcp", "192.168.210.2:80", "0.0.0.0:0", "LISTEN", 2088, "haproxy"),
		sock("tcp", "0.0.0.0:179", "0.0.0.0:0", "LISTEN", 1550, "bird"),
		sock("tcp", "192.168.210.2:443", "0.0.0.0:0", "LISTEN", 2088, "haproxy"),
		sock("tcp", "127.0.0.1:5432", "0.0.0.0:0", "LISTEN", 1990, "postgres"),
		sock("tcp6", "[::1]:9090", "[::]:0", "LISTEN", 0, ""),
		sock("udp", "10.44.0.1:51820", "0.0.0.0:0", "UNCONN", 0, ""),
		sock("tcp", "192.168.210.2:22", "192.168.1.50:51234", "ESTAB", 5367, "sshd"),
		sock("tcp", "192.168.210.2:443", "203.0.113.7:40112", "ESTAB", 2088, "haproxy"),
		sock("tcp", "192.168.210.2:443", "203.0.113.9:40990", "ESTAB", 2088, "haproxy"),
		sock("tcp", "192.168.210.2:443", "198.51.100.23:51002", "TIME-WAIT", 0, ""),
		sock("tcp", "192.168.210.2:179", "192.168.210.1:179", "ESTAB", 1550, "bird"),
		sock("tcp", "127.0.0.1:43122", "127.0.0.1:5432", "ESTAB", 2604, "wotan"),
		sock("tcp", "192.168.210.2:52210", "203.0.113.80:443", "ESTAB", 5367, "orb"),
		sock("tcp", "192.168.210.2:60122", "140.82.112.4:443", "CLOSE-WAIT", 4980, "fwupd"),
		sock("tcp6", "[2001:db8::20]:443", "[2001:db8::99]:61000", "ESTAB", 2088, "haproxy"),
	}
	s.Links = demoLinks(wave)
	names := []string{"llama-server", "go", "rustc", "postgres", "prometheus", "haproxy", "dockerd", "wotan", "sshd", "systemd-journal"}
	for i, n := range names {
		s.Procs = append(s.Procs, Proc{PID: 1200 + i*331, Name: n, CPU: 3.4 / float64(i+1) * (0.6 + 0.4*s.CPU), RSS: uint64(9-i) * gib / 3})
	}
	return s
}

// demoLinks is a west-like layer 2/3 view: a P2P link to east, Wi-Fi to the
// home router, WireGuard, and container bridges with veth ports.
func demoLinks(wave func(i, phase, period, lo, hi float64) float64) *Links {
	now := float64(History)
	r := func(phase, lo, hi float64) float64 { return wave(now, phase, 40, lo, hi) }
	l := []Link{
		{Name: "enp16s0", Index: 2, Kind: "ether", Up: true, Carrier: true, SpeedMbps: 1000, Duplex: "full", MTU: 1500, MAC: "02:00:00:00:00:01", RxRate: r(0, 2e6, 60e6), TxRate: r(1, 1e6, 30e6), Addrs: []string{"192.168.210.2"}},
		{Name: "wlp17s0", Index: 3, Kind: "wifi", Up: true, Carrier: true, MTU: 1500, MAC: "02:00:00:00:00:02", RxRate: r(2, 2e4, 2e6), TxRate: r(3, 1e4, 4e5), Drops: 4826, DropRate: 0.2, Addrs: []string{"192.168.50.184", "2001:db8:a:5a00::5"}},
		{Name: "br-84fffb3566", Index: 5, Kind: "bridge", Up: true, Carrier: true, MTU: 1500, Ports: []string{"veth259a985", "veth3ce62fe", "veth5f1e6a3", "vethf44b66b"}, RxRate: r(4, 1e3, 4e5), TxRate: r(5, 1e4, 8e5), Addrs: []string{"172.29.5.1"}},
		{Name: "lxdbr0", Index: 6, Kind: "bridge", Up: true, Carrier: true, MTU: 1500, Ports: []string{"vethc4af1d31"}, RxRate: r(6, 0, 4e4), TxRate: r(7, 0, 2e4), Addrs: []string{"10.20.30.1"}},
		{Name: "docker0", Index: 7, Kind: "bridge", MTU: 1500, Addrs: []string{"172.17.0.1"}},
		{Name: "wg0", Index: 8, Kind: "wireguard", Up: true, Carrier: true, MTU: 1420, RxRate: r(8, 1e4, 3e5), TxRate: r(9, 1e4, 2e5), Addrs: []string{"fd12:3456:789a::1"}},
		{Name: "veth259a985", Index: 10, Kind: "veth", Up: true, Carrier: true, SpeedMbps: 10000, MTU: 1500, Master: "br-84fffb3566", RxRate: r(10, 0, 3e5), TxRate: r(11, 0, 6e5)},
		{Name: "veth3ce62fe", Index: 11, Kind: "veth", Up: true, Carrier: true, SpeedMbps: 10000, MTU: 1500, Master: "br-84fffb3566", RxRate: r(12, 0, 5e4)},
		{Name: "veth5f1e6a3", Index: 12, Kind: "veth", Up: true, Carrier: true, SpeedMbps: 10000, MTU: 1500, Master: "br-84fffb3566"},
		{Name: "vethf44b66b", Index: 13, Kind: "veth", Up: true, Carrier: true, SpeedMbps: 10000, MTU: 1500, Master: "br-84fffb3566", TxRate: r(13, 0, 2e5)},
		{Name: "vethc4af1d31", Index: 14, Kind: "veth", Up: true, Carrier: true, SpeedMbps: 10000, MTU: 1500, Master: "lxdbr0", RxRate: r(14, 0, 4e4)},
		{Name: "lo", Index: 1, Kind: "loopback", Up: true, Carrier: true, MTU: 65536, RxRate: r(15, 1e3, 1e5), TxRate: r(15, 1e3, 1e5)},
	}
	for i := range l {
		if l[i].RxRate+l[i].TxRate == 0 {
			continue
		}
		peak := (l[i].RxRate + l[i].TxRate) * 1.4
		l[i].Hist = make([]float64, History)
		for j := range l[i].Hist {
			l[i].Hist[j] = wave(float64(j), float64(i), 25, peak*0.1, peak)
		}
	}
	a, p := netip.MustParseAddr, netip.MustParsePrefix
	return &Links{
		Links: l,
		Neigh: []Neighbor{
			{a("192.168.210.1"), "02:00:00:00:00:03", "enp16s0", "REACHABLE"},
			{a("fe80::3"), "02:00:00:00:00:03", "enp16s0", "STALE"},
			{a("192.168.50.129"), "02:00:00:00:00:04", "wlp17s0", "REACHABLE"},
			{a("192.168.50.135"), "02:00:00:00:00:05", "wlp17s0", "STALE"},
			{a("fe80::4"), "02:00:00:00:00:04", "wlp17s0", "REACHABLE"},
			{a("172.29.5.5"), "02:00:00:00:00:06", "br-84fffb3566", "REACHABLE"},
			{a("172.29.5.7"), "02:00:00:00:00:07", "br-84fffb3566", "DELAY"},
			{a("10.20.30.252"), "02:00:00:00:00:08", "lxdbr0", "REACHABLE"},
			{a("192.168.50.140"), "", "wlp17s0", "FAILED"},
		},
		Routes: []Route{
			{p("0.0.0.0/0"), a("192.168.50.129"), "wlp17s0", 600, "static"},
			{p("::/0"), a("fe80::4"), "wlp17s0", 1024, "ra"},
			{p("10.20.30.0/24"), netip.Addr{}, "lxdbr0", 0, "kernel"},
			{p("172.17.0.0/16"), netip.Addr{}, "docker0", 0, "kernel"},
			{p("172.29.5.0/24"), netip.Addr{}, "br-84fffb3566", 0, "kernel"},
			{p("192.168.210.0/30"), netip.Addr{}, "enp16s0", 0, "kernel"},
			{p("192.168.50.128/25"), netip.Addr{}, "wlp17s0", 600, "kernel"},
			{p("2001:db8:a:5a00::/64"), netip.Addr{}, "wlp17s0", 1024, "ra"},
			{p("fd12:3456:789a:2::/64"), a("fd12:3456:789a::2"), "wg0", 20, "bgp"},
			{p("fd12:3456:789a::2/128"), netip.Addr{}, "wg0", 1024, "boot"},
		},
		FDB: []FDBEntry{
			{"02:00:00:00:00:09", "veth3ce62fe", "br-84fffb3566", 0},
			{"02:00:00:00:00:07", "veth259a985", "br-84fffb3566", 0},
			{"02:00:00:00:00:06", "vethf44b66b", "br-84fffb3566", 0},
			{"02:00:00:00:00:0a", "veth5f1e6a3", "br-84fffb3566", 0},
			{"02:00:00:00:00:08", "vethc4af1d31", "lxdbr0", 1},
		},
		LLDP: []LLDPNeighbor{
			{Iface: "enp16s0", Chassis: "02:00:00:00:00:03", Port: "enp1s0", PortDesc: "p2p to west", Name: "east", Desc: "Ubuntu 25.10 Linux 6.17.0-41-generic x86_64", Caps: []string{"bridge", "router", "station"}, Mgmt: []string{"192.168.210.1", "fd12:3456:789a::2"}, TTL: 120, Age: 12},
		},
		LLDPOn: true,
	}
}
