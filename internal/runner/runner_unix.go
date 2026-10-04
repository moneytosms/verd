//go:build unix

package runner

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// prepare puts the child in its own process group so kill reaches grandchildren too.
func prepare(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Process.Kill()
	}
}

// maxRSSMB is the child's peak resident set in MB. Rusage.Maxrss is KB on Linux, bytes on macOS.
func maxRSSMB(ps *os.ProcessState) float64 {
	ru, ok := ps.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return float64(ru.Maxrss) / (1 << 20)
	}
	return float64(ru.Maxrss) / 1024
}
