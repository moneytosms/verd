// Package stress hunts for counterexamples: gen <seed> feeds a Solution and a brute force until they disagree.
package stress

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/runner"
)

type Program struct {
	Path string
	Lang config.Lang
}

type Spec struct {
	Gen, Solution, Brute Program
	CacheDir             string
	Mode                 runner.Mode
	FloatEps             float64
	TimeLimitMS          int           // per Solution run; 0 = 10 s
	MaxIter              int           // default 1000
	Timeout              time.Duration // default 30 s
}

// Outcome kinds.
const (
	Limit       = "limit"    // no counterexample within the limits
	Mismatch    = "mismatch" // Solution disagrees with brute
	SolutionRE  = "solution-re"
	SolutionTLE = "solution-tle"
	Error       = "error" // gen or brute failed, or a build failed: not the Solution's fault
	Cancelled   = "cancelled"
)

type Result struct {
	Kind       string
	Iterations int
	Seed       int
	Input      string
	Want, Got  string // brute and Solution output
	Mismatch   *runner.Mismatch
	Err        string
}

// Event reports progress; the last one has Done set and carries the Result.
type Event struct {
	Iter, Seed int
	Done       bool
	Result     Result
}

// Run loops seeds 1, 2, ... and streams events until a stop condition. The channel closes after Done.
func Run(ctx context.Context, spec Spec) <-chan Event {
	ch := make(chan Event, 16)
	go func() {
		defer close(ch)
		res := loop(ctx, spec, func(i, seed int) {
			select {
			case ch <- Event{Iter: i, Seed: seed}:
			case <-ctx.Done():
			}
		})
		ch <- Event{Iter: res.Iterations, Seed: res.Seed, Done: true, Result: res}
	}()
	return ch
}

func exec1(ctx context.Context, argv []string, stdin string, limit time.Duration) (out, errOut string, code int, tle bool) {
	c, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(c, argv[0], argv[1:]...)
	var o, e bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader([]byte(stdin)), &o, &e
	err := cmd.Run()
	if c.Err() == context.DeadlineExceeded {
		return o.String(), e.String(), -1, true
	}
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return o.String(), e.String(), code, false
}

func loop(ctx context.Context, spec Spec, progress func(i, seed int)) Result {
	if spec.MaxIter == 0 {
		spec.MaxIter = 1000
	}
	if spec.Timeout == 0 {
		spec.Timeout = 30 * time.Second
	}
	limit := 10 * time.Second
	if spec.TimeLimitMS > 0 {
		limit = time.Duration(spec.TimeLimitMS) * time.Millisecond
	}
	eps := spec.FloatEps
	if eps == 0 {
		eps = 1e-6
	}
	mode := spec.Mode
	if mode == "" {
		mode = runner.Tokens
	}
	var argv [3][]string
	for i, p := range []Program{spec.Gen, spec.Solution, spec.Brute} {
		a, _, err := runner.Build(ctx, p.Lang, p.Path, spec.CacheDir)
		if err != nil {
			return Result{Kind: Error, Err: fmt.Sprintf("building %s: %v", p.Path, err)}
		}
		argv[i] = a
	}
	deadline := time.Now().Add(spec.Timeout)
	for i := 1; ; i++ {
		if ctx.Err() != nil {
			return Result{Kind: Cancelled, Iterations: i - 1}
		}
		if i > spec.MaxIter || time.Now().After(deadline) {
			return Result{Kind: Limit, Iterations: i - 1}
		}
		progress(i, i)
		in, ge, gc, gtle := exec1(ctx, append(append([]string{}, argv[0]...), strconv.Itoa(i)), "", limit)
		if ctx.Err() != nil {
			return Result{Kind: Cancelled, Iterations: i - 1}
		}
		if gc != 0 || gtle {
			return Result{Kind: Error, Iterations: i, Seed: i, Err: "gen failed: " + ge}
		}
		got, se, sc, stle := exec1(ctx, argv[1], in, limit)
		switch {
		case ctx.Err() != nil:
			return Result{Kind: Cancelled, Iterations: i - 1}
		case stle:
			return Result{Kind: SolutionTLE, Iterations: i, Seed: i, Input: in}
		case sc != 0:
			return Result{Kind: SolutionRE, Iterations: i, Seed: i, Input: in, Err: se}
		}
		want, be, bc, btle := exec1(ctx, argv[2], in, 10*limit)
		if bc != 0 || btle {
			return Result{Kind: Error, Iterations: i, Seed: i, Input: in, Err: "brute failed: " + be}
		}
		if ok, mm := runner.Compare(mode, want, got, eps); !ok {
			return Result{Kind: Mismatch, Iterations: i, Seed: i, Input: in, Want: want, Got: got, Mismatch: mm}
		}
	}
}
