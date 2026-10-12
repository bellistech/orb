package sys

import (
	"os"
	"strings"
	"syscall"
)

// Mount is one filesystem's space usage in bytes.
type Mount struct {
	Path, Device, FSType string
	Size, Used, Avail    uint64
}

// UsedFrac matches df: used / (used + available to unprivileged users).
func (m Mount) UsedFrac() float64 { return frac(m.Used, m.Used+m.Avail) }

// DiskIO is one block device's throughput in bytes/second.
type DiskIO struct {
	Name                string
	ReadRate, WriteRate float64
}

var realFS = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true, "xfs": true, "btrfs": true,
	"vfat": true, "exfat": true, "f2fs": true, "zfs": true, "ntfs3": true, "fuseblk": true,
}

func readMounts() []Mount {
	var out []Mount
	seen := map[string]bool{}
	for _, line := range strings.Split(readString("/proc/self/mounts"), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || !realFS[f[2]] || seen[f[0]] {
			continue
		}
		path := unescapeMount(f[1])
		var st syscall.Statfs_t
		if syscall.Statfs(path, &st) != nil || st.Blocks == 0 {
			continue
		}
		seen[f[0]] = true
		bs := uint64(st.Bsize)
		out = append(out, Mount{
			Path: path, Device: f[0], FSType: f[2],
			Size:  st.Blocks * bs,
			Used:  (st.Blocks - st.Bfree) * bs,
			Avail: st.Bavail * bs,
		})
	}
	return out
}

// unescapeMount decodes the octal escapes /proc/mounts uses for spaces etc.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			var v byte
			ok := true
			for _, c := range []byte(s[i+1 : i+4]) {
				if c < '0' || c > '7' {
					ok = false
					break
				}
				v = v*8 + (c - '0')
			}
			if ok {
				b.WriteByte(v)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

type diskCounters struct{ read, write uint64 }

// readDiskStats returns sector counters (in bytes) for whole block devices.
func readDiskStats() map[string]diskCounters {
	out := map[string]diskCounters{}
	for _, line := range strings.Split(readString("/proc/diskstats"), "\n") {
		f := strings.Fields(line)
		if len(f) < 10 || !wholeDisk(f[2]) {
			continue
		}
		// diskstats sectors are always 512 bytes.
		out[f[2]] = diskCounters{read: atou(f[5]) * 512, write: atou(f[9]) * 512}
	}
	return out
}

func wholeDisk(name string) bool {
	for _, p := range []string{"loop", "ram", "zram", "dm-"} {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	_, err := os.Stat("/sys/block/" + name)
	return err == nil
}
