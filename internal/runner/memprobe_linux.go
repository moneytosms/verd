package runner

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// memProbe measures a child's peak memory. Rusage.Maxrss alone is wrong on Linux: a child
// inherits its parent's peak RSS at spawn, so every run would show at least verd's own footprint
// (and trip the Memory Limit on small problems). Instead, sample the child's own VmHWM.
type memProbe struct {
	stop chan struct{}
	done chan float64
}

func startMemProbe(cmd *exec.Cmd) *memProbe {
	p := &memProbe{make(chan struct{}), make(chan float64, 1)}
	pid := cmd.Process.Pid
	self := procField("/proc/self/status", "Name:")
	go func() {
		var peak float64
		for {
			// Until exec the child still shares verd's address space and name: skip those samples.
			st := "/proc/" + strconv.Itoa(pid) + "/status"
			if name := procField(st, "Name:"); name != "" && name != self {
				peak = max(peak, kbToMB(procField(st, "VmHWM:")))
			}
			select {
			case <-p.stop:
				p.done <- peak
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	return p
}

// result is the child's peak in MB. Maxrss is trusted only when it exceeds verd's own peak (then
// it cannot be the inherited value); otherwise the sampled peak is used (0 if the child ended
// before the first sample).
func (p *memProbe) result(ps *os.ProcessState) float64 {
	close(p.stop)
	sampled := <-p.done
	if ru := maxRSSMB(ps); ru > kbToMB(procField("/proc/self/status", "VmHWM:")) {
		return ru
	}
	return sampled
}

// procField is the value of a "Key:\tvalue" line of a /proc status file ("" if unreadable).
func procField(path, key string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, key); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// kbToMB parses "1234 kB".
func kbToMB(s string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSuffix(s, " kB"), 64)
	return n / 1024
}
