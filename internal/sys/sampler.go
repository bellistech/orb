// Package sys reads host metrics from /proc and /sys and keeps short
// histories of them. Every reader is best-effort: a missing file yields an
// empty value, never an error.
package sys

import (
	"sort"
	"sync/atomic"
	"time"
)

// History is how many samples each series keeps.
const History = 600

// Snapshot is everything the UI draws for one frame. Slices are copies, so a
// snapshot is safe to read while the next sample is taken.
type Snapshot struct {
	Time time.Time
	Host Host

	CPU      float64   // aggregate busy, 0..1
	Cores    []float64 // per-core busy, 0..1
	CoreMHz  []float64
	CPUHist  []float64
	CoreHist [][]float64

	Mem     Mem
	MemHist []float64

	Net       []NetIf
	NetRx     []float64 // aggregate bytes/s history
	NetTx     []float64
	Mounts    []Mount
	DiskIO    []DiskIO
	DiskRead  []float64 // aggregate bytes/s history
	DiskWrite []float64

	Temps    []Temp
	Fans     []Fan
	TempHist []float64 // hottest sensor, °C

	GPUs    []GPU
	GPUHist [][]float64

	Procs []Proc

	Sockets []Socket // only while the network page asks for them
}

// Sampler takes successive readings and turns counters into rates. It is not
// safe for concurrent use; call Sample from one goroutine at a time.
type Sampler struct {
	static Host
	last   time.Time

	cpu   []cpuTimes
	net   map[string]netCounters
	disk  map[string]diskCounters
	procs map[int]procTicks

	cpuHist  *Ring
	coreHist []*Ring
	memHist  *Ring
	netRx    *Ring
	netTx    *Ring
	diskR    *Ring
	diskW    *Ring
	gpuHist  map[string]*Ring
	tempHist *Ring

	// WantSockets turns socket reading on; the UI sets it while the
	// network page is up. Safe to set from any goroutine.
	WantSockets atomic.Bool
	sockets     []Socket
	socketsAt   time.Time
}

// socketEvery is how often sockets are re-read while wanted: walking every
// process's fds is the most expensive read orb does.
const socketEvery = 2 * time.Second

// NewSampler primes the counters so the first Sample has rates.
func NewSampler() *Sampler {
	s := &Sampler{
		static:   readHostStatic(),
		cpuHist:  NewRing(History),
		memHist:  NewRing(History),
		netRx:    NewRing(History),
		netTx:    NewRing(History),
		diskR:    NewRing(History),
		diskW:    NewRing(History),
		gpuHist:  map[string]*Ring{},
		tempHist: NewRing(History),
	}
	s.last = time.Now()
	s.cpu = readCPUTimes()
	s.net = readNet()
	s.disk = readDiskStats()
	s.procs = readProcs()
	return s
}

// Sample reads everything once.
func (s *Sampler) Sample() *Snapshot {
	now := time.Now()
	dt := now.Sub(s.last).Seconds()
	s.last = now
	snap := &Snapshot{Time: now, Host: s.static}
	readHostDynamic(&snap.Host)

	cpu := readCPUTimes()
	if len(cpu) > 0 && len(s.cpu) == len(cpu) {
		snap.CPU = busy(s.cpu[0], cpu[0])
		for i := 1; i < len(cpu); i++ {
			snap.Cores = append(snap.Cores, busy(s.cpu[i], cpu[i]))
			snap.CoreMHz = append(snap.CoreMHz, coreFreqMHz(i-1))
		}
	}
	s.cpu = cpu
	s.cpuHist.Push(snap.CPU)
	for len(s.coreHist) < len(snap.Cores) {
		s.coreHist = append(s.coreHist, NewRing(History))
	}
	for i, v := range snap.Cores {
		s.coreHist[i].Push(v)
		snap.CoreHist = append(snap.CoreHist, s.coreHist[i].Values())
	}
	snap.CPUHist = s.cpuHist.Values()

	snap.Mem = readMem()
	s.memHist.Push(snap.Mem.UsedFrac())
	snap.MemHist = s.memHist.Values()

	net := readNet()
	var rx, tx float64
	for name, c := range net {
		n := NetIf{Name: name, Up: ifUp(name), Addrs: ifAddrs(name)}
		if p, ok := s.net[name]; ok && dt > 0 {
			n.RxRate = rate(p.rx, c.rx, dt)
			n.TxRate = rate(p.tx, c.tx, dt)
		}
		rx += n.RxRate
		tx += n.TxRate
		snap.Net = append(snap.Net, n)
	}
	s.net = net
	sort.Slice(snap.Net, func(i, j int) bool {
		a, b := snap.Net[i], snap.Net[j]
		if a.Up != b.Up {
			return a.Up
		}
		return a.Name < b.Name
	})
	s.netRx.Push(rx)
	s.netTx.Push(tx)
	snap.NetRx, snap.NetTx = s.netRx.Values(), s.netTx.Values()

	snap.Mounts = readMounts()
	disk := readDiskStats()
	var dr, dw float64
	for name, c := range disk {
		d := DiskIO{Name: name}
		if p, ok := s.disk[name]; ok && dt > 0 {
			d.ReadRate = rate(p.read, c.read, dt)
			d.WriteRate = rate(p.write, c.write, dt)
		}
		dr += d.ReadRate
		dw += d.WriteRate
		snap.DiskIO = append(snap.DiskIO, d)
	}
	s.disk = disk
	sort.Slice(snap.DiskIO, func(i, j int) bool { return snap.DiskIO[i].Name < snap.DiskIO[j].Name })
	s.diskR.Push(dr)
	s.diskW.Push(dw)
	snap.DiskRead, snap.DiskWrite = s.diskR.Values(), s.diskW.Values()

	snap.Temps, snap.Fans = readSensors()
	hot := 0.0
	for _, t := range snap.Temps {
		hot = max(hot, t.Celsius)
	}
	s.tempHist.Push(hot)
	snap.TempHist = s.tempHist.Values()

	snap.GPUs = readGPUs()
	for _, g := range snap.GPUs {
		r, ok := s.gpuHist[g.Card]
		if !ok {
			r = NewRing(History)
			s.gpuHist[g.Card] = r
		}
		r.Push(g.Busy)
		snap.GPUHist = append(snap.GPUHist, r.Values())
	}

	procs := readProcs()
	snap.Procs = topProcs(s.procs, procs, dt, 32)
	s.procs = procs

	if s.WantSockets.Load() {
		if s.sockets == nil || now.Sub(s.socketsAt) >= socketEvery {
			s.sockets, s.socketsAt = ReadSockets(), now
			if s.sockets == nil {
				s.sockets = []Socket{}
			}
		}
		snap.Sockets = s.sockets
	} else {
		s.sockets = nil
	}
	return snap
}

func rate(prev, cur uint64, dt float64) float64 {
	if cur < prev {
		return 0 // counter reset
	}
	return float64(cur-prev) / dt
}
