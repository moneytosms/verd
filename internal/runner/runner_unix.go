//go:build unix

package runner

import (
	"fmt"
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

// signalName is the fatal signal that ended the child ("" if it exited normally).
func signalName(ps *os.ProcessState) string {
	ws, ok := ps.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		return ""
	}
	sig := ws.Signal()
	names := map[syscall.Signal]string{
		syscall.SIGSEGV: "SIGSEGV", syscall.SIGABRT: "SIGABRT", syscall.SIGFPE: "SIGFPE", syscall.SIGILL: "SIGILL",
		syscall.SIGBUS: "SIGBUS", syscall.SIGKILL: "SIGKILL", syscall.SIGTERM: "SIGTERM", syscall.SIGPIPE: "SIGPIPE",
		syscall.SIGINT: "SIGINT", syscall.SIGHUP: "SIGHUP", syscall.SIGXCPU: "SIGXCPU", syscall.SIGTRAP: "SIGTRAP",
	}
	if n, ok := names[sig]; ok {
		return n
	}
	return fmt.Sprintf("signal %d", int(sig))
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
