package sys

import (
	"path/filepath"
	"sort"
	"strings"
)

// Temp is one temperature sensor.
type Temp struct {
	Chip, Label string
	Celsius     float64
	Crit        float64 // 0 when the sensor reports none
}

// Fan is one fan sensor.
type Fan struct {
	Chip, Label string
	RPM         float64
}

// friendly names for common hwmon chips.
var chipNames = map[string]string{
	"k10temp":  "cpu",
	"coretemp": "cpu",
	"amdgpu":   "gpu",
	"nvme":     "nvme",
	"spd5118":  "dimm",
	"jc42":     "dimm",
}

func readSensors() ([]Temp, []Fan) {
	var temps []Temp
	var fans []Fan
	dirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	sort.Strings(dirs)
	for _, d := range dirs {
		chip := readString(d + "/name")
		if n, ok := chipNames[chip]; ok {
			chip = n
		} else if strings.HasPrefix(chip, "mt79") || strings.HasPrefix(chip, "iwlwifi") {
			chip = "wifi"
		}
		inputs, _ := filepath.Glob(d + "/temp*_input")
		sort.Strings(inputs)
		for _, in := range inputs {
			mc, ok := readInt(in)
			if !ok {
				continue
			}
			base := strings.TrimSuffix(in, "_input")
			t := Temp{Chip: chip, Label: readString(base + "_label"), Celsius: float64(mc) / 1000}
			if c, ok := readInt(base + "_crit"); ok && c > 0 {
				t.Crit = float64(c) / 1000
			}
			temps = append(temps, t)
		}
		fanIn, _ := filepath.Glob(d + "/fan*_input")
		sort.Strings(fanIn)
		for _, in := range fanIn {
			if rpm, ok := readUint(in); ok {
				fans = append(fans, Fan{Chip: chip, Label: readString(strings.TrimSuffix(in, "_input") + "_label"), RPM: float64(rpm)})
			}
		}
	}
	return temps, fans
}
