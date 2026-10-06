//go:build !linux

package runner

import (
	"os"
	"os/exec"
)

type memProbe struct{}

func startMemProbe(*exec.Cmd) *memProbe { return &memProbe{} }

func (*memProbe) result(ps *os.ProcessState) float64 { return maxRSSMB(ps) }
