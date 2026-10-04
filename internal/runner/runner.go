// Package runner compiles a Solution (cached) and runs it against tests, one at a time.
package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/workspace"
)

const (
	defaultOutputCap = 64 << 20
	stderrCap        = 64 << 10
)

// Local Verdicts.
const (
	AC  = "AC"
	WA  = "WA"
	TLE = "TLE"
	RE  = "RE"
	MLE = "MLE"
	CE  = "CE"
	Unk = "??" // mode none: output shown, not judged
)

type Spec struct {
	Solution string // path to the source file
	Lang     config.Lang
	CacheDir string // compiled binaries live here
	Tests    []workspace.Test

	TimeLimitMS int // the Problem's limit; 0 = unlimited
	MemoryMB    int // soft: reported and flagged MLE, not enforced; 0 = unchecked
	Multiplier  float64
	Mode        Mode
	FloatEps    float64 // default 1e-6
	OutputCap   int     // bytes of stdout kept; default 64 MB
}

type Result struct {
	Name     string
	Verdict  string
	TimeMS   int
	MemoryMB float64
	ExitCode int
	Mismatch *Mismatch
	Expected string
	Output   string
	Stderr   string
	Note     string // e.g. "output limit exceeded", "killed by signal"
}

type EventKind int

const (
	CompileStarted EventKind = iota
	CompileFinished
	TestFinished
	Done
)

type Event struct {
	Kind    EventKind
	Cached  bool   // CompileFinished: binary reused
	Err     string // CompileFinished: compiler output when it failed
	Result  Result // TestFinished
	Verdict string // Done: overall (first failing verdict, else AC; CE if compilation failed)
}

// Run compiles then runs every test sequentially, streaming events. The channel is closed after Done.
func Run(ctx context.Context, spec Spec) <-chan Event {
	ch := make(chan Event, 8)
	go func() {
		defer close(ch)
		r := &run{spec: spec, ctx: ctx}
		ch <- Event{Kind: CompileStarted}
		cached, err := r.compile()
		if err != nil {
			ch <- Event{Kind: CompileFinished, Err: err.Error()}
			ch <- Event{Kind: Done, Verdict: CE}
			return
		}
		ch <- Event{Kind: CompileFinished, Cached: cached}
		overall := AC
		for _, t := range spec.Tests {
			if ctx.Err() != nil {
				break
			}
			res := r.runTest(t)
			ch <- Event{Kind: TestFinished, Result: res}
			if overall == AC && res.Verdict != AC {
				overall = res.Verdict
			}
		}
		ch <- Event{Kind: Done, Verdict: overall}
	}()
	return ch
}

type run struct {
	spec Spec
	ctx  context.Context
	bin  string
}

func expand(argv []string, vars map[string]string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		for k, v := range vars {
			a = strings.ReplaceAll(a, k, v)
		}
		out[i] = a
	}
	return out
}

func (r *run) vars() map[string]string {
	return map[string]string{"{src}": r.spec.Solution, "{bin}": r.bin, "{dir}": filepath.Dir(r.spec.Solution)}
}

// compile builds the Solution unless an identical build (source + compile argv) is cached.
func (r *run) compile() (cached bool, err error) {
	if len(r.spec.Lang.Compile) == 0 {
		return true, nil // interpreted
	}
	src, err := os.ReadFile(r.spec.Solution)
	if err != nil {
		return false, err
	}
	h := sha256.New()
	h.Write(src)
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(r.spec.Lang.Compile, "\x00")))
	r.bin = filepath.Join(r.spec.CacheDir, hex.EncodeToString(h.Sum(nil)))
	if st, err := os.Stat(r.bin); err == nil && st.Mode().IsRegular() {
		return true, nil
	}
	if err := os.MkdirAll(r.spec.CacheDir, 0o755); err != nil {
		return false, err
	}
	tmp := r.bin + ".tmp" // a failed or interrupted build must not look cached
	final := r.bin
	r.bin = tmp
	argv := expand(r.spec.Lang.Compile, r.vars())
	cmd := exec.CommandContext(r.ctx, argv[0], argv[1:]...)
	cmd.Dir = filepath.Dir(r.spec.Solution)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tmp)
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return false, errors.New(msg)
	}
	if err := os.Rename(tmp, final); err != nil {
		return false, err
	}
	r.bin = final
	return false, nil
}

// capBuffer keeps at most max bytes and records overflow.
type capBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	max      int
	overflow bool
	onFull   func()
}

func (c *capBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := c.max - c.buf.Len(); len(p) > room {
		c.buf.Write(p[:max(room, 0)])
		if !c.overflow {
			c.overflow = true
			if c.onFull != nil {
				c.onFull()
			}
		}
	} else {
		c.buf.Write(p)
	}
	return len(p), nil // never error: the child must not die of EPIPE instead of being killed by us
}

func (r *run) runTest(t workspace.Test) Result {
	res := Result{Name: t.Name}
	want, err := os.ReadFile(t.Ans)
	if err != nil {
		return Result{Name: t.Name, Verdict: RE, Note: err.Error()}
	}
	res.Expected = string(want)
	in, err := os.Open(t.In)
	if err != nil {
		return Result{Name: t.Name, Verdict: RE, Note: err.Error()}
	}
	defer in.Close()

	argv := expand(r.spec.Lang.Run, r.vars())
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = filepath.Dir(r.spec.Solution)
	cmd.Stdin = in
	outCap := r.spec.OutputCap
	if outCap == 0 {
		outCap = defaultOutputCap
	}
	full := make(chan struct{})
	stdout := &capBuffer{max: outCap, onFull: func() { close(full) }}
	stderr := &capBuffer{max: stderrCap}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	prepare(cmd)

	mult := r.spec.Multiplier
	if mult == 0 {
		mult = 1
	}
	limit := time.Duration(float64(r.spec.TimeLimitMS)*mult) * time.Millisecond
	hard := max(2*limit, limit+time.Second)

	start := time.Now()
	if err := cmd.Start(); err != nil {
		res.Verdict, res.Note = RE, err.Error()
		return res
	}
	done := make(chan struct{})
	defer close(done)
	go func() { // runaway printer: kill it rather than buffer forever
		select {
		case <-full:
			kill(cmd)
		case <-done:
		}
	}()
	killedByUs := false
	var mu sync.Mutex
	if r.spec.TimeLimitMS > 0 {
		timer := time.AfterFunc(hard, func() { mu.Lock(); killedByUs = true; mu.Unlock(); kill(cmd) })
		defer timer.Stop()
	}
	stopCtx := context.AfterFunc(r.ctx, func() { kill(cmd) })
	defer stopCtx()
	waitErr := cmd.Wait()
	elapsed := time.Since(start)

	res.TimeMS = int(elapsed / time.Millisecond)
	res.ExitCode = cmd.ProcessState.ExitCode()
	res.MemoryMB = maxRSSMB(cmd.ProcessState)
	res.Output, res.Stderr = stdout.buf.String(), stderr.buf.String()
	mu.Lock()
	killed := killedByUs
	mu.Unlock()

	switch {
	case killed || (r.spec.TimeLimitMS > 0 && elapsed > limit):
		res.Verdict = TLE
	case stdout.overflow:
		res.Verdict, res.Note = RE, fmt.Sprintf("output limit exceeded (%d bytes)", outCap)
	case waitErr != nil:
		res.Verdict = RE
		if res.ExitCode == -1 {
			res.Note = "killed by signal"
		}
	case r.spec.MemoryMB > 0 && res.MemoryMB > float64(r.spec.MemoryMB):
		res.Verdict = MLE
	default:
		eps := r.spec.FloatEps
		if eps == 0 {
			eps = 1e-6
		}
		mode := r.spec.Mode
		if mode == "" {
			mode = Tokens
		}
		ok, mm := Compare(mode, res.Expected, res.Output, eps)
		switch {
		case mode == None:
			res.Verdict = Unk
		case ok:
			res.Verdict = AC
		default:
			res.Verdict, res.Mismatch = WA, mm
		}
	}
	return res
}
