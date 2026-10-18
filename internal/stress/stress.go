// Package stress hunts for counterexamples: gen <seed> feeds a Solution and a brute force until they disagree.
package stress

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
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
	Validator            Program // optional: reads an input on stdin, exits non-zero if it breaks the Problem's constraints
	CacheDir             string
	Mode                 runner.Mode
	FloatEps             float64
	TimeLimitMS          int           // per Solution run; 0 = 10 s
	MaxIter              int           // default 1000
	Timeout              time.Duration // default 30 s
	Jobs                 int           // parallel iterations; 0 = NumCPU (max 16)
	Batch                int           // >1: gen runs as "gen <first seed> <Batch>" and prints Batch cases split by "---" lines
	Multi                int           // >1: Solution and brute are multi-test programs (read t, loop); gen prints one case without t; Multi cases run per process pair
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

// sizeFor is the SIZE gen sees for a seed: it grows 1, 2, ... 10 over the first 300 seeds, so the first
// counterexample found is usually a small one.
func sizeFor(seed int) int { return min(1+(seed-1)/30, 10) }

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

func exec1(ctx context.Context, argv []string, stdin string, limit time.Duration, env ...string) (out, errOut string, code int, tle bool) {
	c, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(c, argv[0], argv[1:]...)
	var o, e bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader([]byte(stdin)), &o, &e
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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
	var argv [4][]string
	for i, p := range []Program{spec.Gen, spec.Solution, spec.Brute, spec.Validator} {
		if p.Path == "" {
			continue
		}
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
	// genCase returns the Input for seed i. With Batch, one gen run serves a chunk of seeds.
	var chunks sync.Map // chunk number -> *chunk
	type chunk struct {
		once  sync.Once
		cases []string
		err   string
	}
	genRaw := func(wctx context.Context, i int) (in, errOut string, ok bool) {
		if spec.Batch <= 1 {
			in, ge, gc, gtle := exec1(wctx, append(append([]string{}, argv[0]...), strconv.Itoa(i)), "", limit, "SIZE="+strconv.Itoa(sizeFor(i)))
			return in, ge, gc == 0 && !gtle
		}
		n, k := (i-1)/spec.Batch, (i-1)%spec.Batch
		c, _ := chunks.LoadOrStore(n, &chunk{})
		ck := c.(*chunk)
		ck.once.Do(func() {
			first := n*spec.Batch + 1
			out, ge, gc, gtle := exec1(wctx, append(append([]string{}, argv[0]...), strconv.Itoa(first), strconv.Itoa(spec.Batch)), "", 10*limit, "SIZE="+strconv.Itoa(sizeFor(first)))
			if wctx.Err() != nil {
				return
			}
			if gc != 0 || gtle {
				ck.err = ge
				return
			}
			if ck.cases = splitCases(out); len(ck.cases) != spec.Batch {
				ck.err = fmt.Sprintf("batch gen printed %d cases, want %d (separate cases with a --- line)", len(ck.cases), spec.Batch)
			}
		})
		if ck.err != "" || k >= len(ck.cases) {
			return "", ck.err, false
		}
		return ck.cases[k], "", true
	}
	genCase := func(wctx context.Context, i int) (in, errOut string, ok bool) {
		in, errOut, ok = genRaw(wctx, i)
		if ok && argv[3] != nil {
			_, ve, vc, vtle := exec1(wctx, argv[3], in, limit)
			if (vc != 0 || vtle) && wctx.Err() == nil {
				return in, "validator rejected the generated input: " + strings.TrimSpace(ve), false
			}
		}
		return
	}
	// iterate runs one seed; nil means it passed. ok is false if ctx was cancelled mid-run.
	iterate := func(wctx context.Context, i int) (r *Result, ok bool) {
		in, ge, gok := genCase(wctx, i)
		if wctx.Err() != nil {
			return nil, false
		}
		if spec.Multi > 1 {
			in = "1\n" + in
		}
		if !gok {
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

	step := max(spec.Multi, 1) // seeds per unit of work
	// fast runs seeds lo..hi as one multi-test input; true means all passed. Anything else falls back to per-seed runs.
	fast := func(wctx context.Context, lo, hi int) bool {
		var b strings.Builder
		b.WriteString(strconv.Itoa(hi-lo+1) + "\n")
		for i := lo; i <= hi; i++ {
			in, _, ok := genCase(wctx, i)
			if !ok {
				return false
			}
			b.WriteString(in)
		}
		n := time.Duration(hi - lo + 1)
		got, _, sc, stle := exec1(wctx, argv[1], b.String(), limit*n)
		if sc != 0 || stle {
			return false
		}
		want, _, bc, btle := exec1(wctx, argv[2], b.String(), 10*limit*n)
		if bc != 0 || btle {
			return false
		}
		ok, _ := runner.Compare(mode, want, got, eps)
		return ok
	}
	// unit runs seeds lo..hi; nil means all passed.
	unit := func(wctx context.Context, lo, hi int) (r *Result, ok bool) {
		if step > 1 && fast(wctx, lo, hi) {
			return nil, wctx.Err() == nil
		}
		for i := lo; i <= hi; i++ {
			if r, ok = iterate(wctx, i); r != nil || !ok {
				return
			}
		}
		return nil, true
	}

	// Workers claim seeds in increasing order; results are consumed in seed order so the
	// lowest failing seed wins and stop conditions match the sequential loop.
	type done struct {
		lo, hi int
		r      *Result
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
				lo := int(next.Add(1)-1)*step + 1
				hi := min(lo+step-1, spec.MaxIter)
				if lo > spec.MaxIter || int64(lo) > minFail.Load() || time.Now().After(deadline) || wctx.Err() != nil {
					return
				}
				r, ok := unit(wctx, lo, hi)
				if !ok {
					return
				}
				if r != nil {
					for m := minFail.Load(); int64(r.Seed) < m && !minFail.CompareAndSwap(m, int64(r.Seed)); m = minFail.Load() {
					}
				}
				select {
				case results <- done{lo, hi, r}:
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
		pending[d.lo] = &d
		for p := pending[emit]; p != nil; p = pending[emit] {
			if p.r != nil {
				return *p.r
			}
			for i := p.lo; i <= p.hi; i++ {
				progress(i, i)
			}
			delete(pending, emit)
			emit = p.hi + 1
		}
	}
	if ctx.Err() != nil {
		return Result{Kind: Cancelled, Iterations: emit - 1}
	}
	return Result{Kind: Limit, Iterations: emit - 1}
}

// splitCases splits batch gen output on lines that are exactly "---"; a trailing separator is allowed.
func splitCases(out string) []string {
	var cases []string
	var cur strings.Builder
	for _, l := range strings.SplitAfter(out, "\n") {
		if strings.TrimRight(l, "\r\n") == "---" {
			cases = append(cases, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteString(l)
	}
	if cur.Len() > 0 {
		cases = append(cases, cur.String())
	}
	return cases
}
