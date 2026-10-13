// Package stress hunts for counterexamples: gen <seed> feeds a Solution and a brute force until they disagree.
package stress

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
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
	Jobs                 int           // parallel iterations; 0 = NumCPU (max 16)
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
	jobs := spec.Jobs
	if jobs <= 0 {
		jobs = min(runtime.NumCPU(), 16)
	}
	jobs = min(jobs, spec.MaxIter)
	// iterate runs one seed; nil means it passed. ok is false if ctx was cancelled mid-run.
	iterate := func(wctx context.Context, i int) (r *Result, ok bool) {
		in, ge, gc, gtle := exec1(wctx, append(append([]string{}, argv[0]...), strconv.Itoa(i)), "", limit)
		if wctx.Err() != nil {
			return nil, false
		}
		if gc != 0 || gtle {
			return &Result{Kind: Error, Iterations: i, Seed: i, Err: "gen failed: " + ge}, true
		}
		got, se, sc, stle := exec1(wctx, argv[1], in, limit)
		switch {
		case wctx.Err() != nil:
			return nil, false
		case stle:
			return &Result{Kind: SolutionTLE, Iterations: i, Seed: i, Input: in}, true
		case sc != 0:
			return &Result{Kind: SolutionRE, Iterations: i, Seed: i, Input: in, Err: se}, true
		}
		want, be, bc, btle := exec1(wctx, argv[2], in, 10*limit)
		if wctx.Err() != nil {
			return nil, false
		}
		if bc != 0 || btle {
			return &Result{Kind: Error, Iterations: i, Seed: i, Input: in, Err: "brute failed: " + be}, true
		}
		if ok, mm := runner.Compare(mode, want, got, eps); !ok {
			return &Result{Kind: Mismatch, Iterations: i, Seed: i, Input: in, Want: want, Got: got, Mismatch: mm}, true
		}
		return nil, true
	}

	// Workers claim seeds in increasing order; results are consumed in seed order so the
	// lowest failing seed wins and stop conditions match the sequential loop.
	type done struct {
		seed int
		r    *Result
	}
	wctx, stop := context.WithCancel(ctx)
	defer stop()
	deadline := time.Now().Add(spec.Timeout)
	var next atomic.Int64
	var minFail atomic.Int64 // lowest failing seed seen so far; higher seeds need not start
	minFail.Store(math.MaxInt64)
	results := make(chan done)
	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1))
				if i > spec.MaxIter || int64(i) > minFail.Load() || time.Now().After(deadline) || wctx.Err() != nil {
					return
				}
				r, ok := iterate(wctx, i)
				if !ok {
					return
				}
				if r != nil {
					for m := minFail.Load(); int64(i) < m && !minFail.CompareAndSwap(m, int64(i)); m = minFail.Load() {
					}
				}
				select {
				case results <- done{i, r}:
				case <-wctx.Done():
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	pending := map[int]*done{}
	emit := 1 // next seed to report
	for d := range results {
		pending[d.seed] = &d
		for p := pending[emit]; p != nil; p = pending[emit] {
			if p.r != nil {
				return *p.r
			}
			progress(emit, emit)
			delete(pending, emit)
			emit++
		}
	}
	if ctx.Err() != nil {
		return Result{Kind: Cancelled, Iterations: emit - 1}
	}
	return Result{Kind: Limit, Iterations: emit - 1}
}
