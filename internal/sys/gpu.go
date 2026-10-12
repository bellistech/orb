package sys

import (
	"path/filepath"
	"sort"
)

// GPU is one amdgpu device.
type GPU struct {
	Card                string
	Discrete            bool
	Busy                float64 // 0..1
	VRAMUsed, VRAMTotal uint64
	GTTUsed, GTTTotal   uint64
	PowerW              float64
	SclkMHz             float64
	TempC               float64
}

// readGPUs reads every DRM card exposing amdgpu's gpu_busy_percent.
func readGPUs() []GPU {
	var out []GPU
	cards, _ := filepath.Glob("/sys/class/drm/card[0-9]*")
	sort.Strings(cards)
	for _, c := range cards {
		dev := c + "/device"
		busyPct, ok := readUint(dev + "/gpu_busy_percent")
		if !ok {
			continue
		}
		g := GPU{Card: filepath.Base(c), Busy: clamp01(float64(busyPct) / 100)}
		g.VRAMUsed, _ = readUint(dev + "/mem_info_vram_used")
		g.VRAMTotal, _ = readUint(dev + "/mem_info_vram_total")
		g.GTTUsed, _ = readUint(dev + "/mem_info_gtt_used")
		g.GTTTotal, _ = readUint(dev + "/mem_info_gtt_total")
		// An APU carves a small VRAM window out of system memory.
		g.Discrete = g.VRAMTotal > 2<<30
		if hw, _ := filepath.Glob(dev + "/hwmon/hwmon*"); len(hw) > 0 {
			h := hw[0]
			if uw, ok := readUint(h + "/power1_average"); ok {
				g.PowerW = float64(uw) / 1e6
			} else if uw, ok := readUint(h + "/power1_input"); ok {
				g.PowerW = float64(uw) / 1e6
			}
			if hz, ok := readUint(h + "/freq1_input"); ok {
				g.SclkMHz = float64(hz) / 1e6
			}
			if mc, ok := readInt(h + "/temp1_input"); ok {
				g.TempC = float64(mc) / 1000
			}
		}
		out = append(out, g)
	}
	return out
}
