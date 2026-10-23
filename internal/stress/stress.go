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

// hunt is one counterexample search: the built argv of each program and the limits.
type hunt struct {
	spec   Spec
	argv   [4][]string // gen, solution, brute, validator
	limit  time.Duration
	eps    float64
	mode   runner.Mode
	step   int      // seeds per unit of work
	chunks sync.Map // batch number -> *batchChunk

	next     atomic.Int64 // next unit to claim
	minFail  atomic.Int64 // lowest failing seed seen so far; higher seeds need not start
	deadline time.Time
}

// batchChunk is one batch gen run, shared by the seeds it covers.
type batchChunk struct {
	once  sync.Once
	cases []string
	err   string
}

// doneUnit is the outcome of seeds lo..hi; r is the first failure, nil if all passed.
type doneUnit struct {
	lo, hi int
	r      *Result
}

// newHunt applies the defaults and builds every program. A build failure comes back as the Result.
func newHunt(ctx context.Context, spec Spec) (*hunt, *Result) {
	if spec.MaxIter == 0 {
		spec.MaxIter = 1000
	}
	if spec.Timeout == 0 {
		spec.Timeout = 30 * time.Second
	}
	h := &hunt{spec: spec, limit: 10 * time.Second, eps: spec.FloatEps, mode: spec.Mode, step: max(spec.Multi, 1)}
	if spec.TimeLimitMS > 0 {
		h.limit = time.Duration(spec.TimeLimitMS) * time.Millisecond
	}
	if h.eps == 0 {
		h.eps = 1e-6
	}
	if h.mode == "" {
		h.mode = runner.Tokens
	}
	for i, p := range []Program{spec.Gen, spec.Solution, spec.Brute, spec.Validator} {
		if p.Path == "" {
			continue
		}
		a, _, err := runner.Build(ctx, p.Lang, p.Path, spec.CacheDir)
		if err != nil {
			return nil, &Result{Kind: Error, Err: fmt.Sprintf("building %s: %v", p.Path, err)}
		}
		h.argv[i] = a
	}
	return h, nil
}

func loop(ctx context.Context, spec Spec, progress func(i, seed int)) Result {
	h, bad := newHunt(ctx, spec)
	if bad != nil {
		return *bad
	}
	return h.search(ctx, progress)
}

// search runs the workers and reports their units in seed order.
func (h *hunt) search(ctx context.Context, progress func(i, seed int)) Result {
	jobs := h.spec.Jobs
	if jobs <= 0 {
		jobs = min(runtime.NumCPU(), 16)
	}
	jobs = min(jobs, h.spec.MaxIter)
	wctx, stop := context.WithCancel(ctx)
	defer stop()
	h.deadline = time.Now().Add(h.spec.Timeout)
	h.minFail.Store(math.MaxInt64)
	// Workers claim seeds in increasing order; results are consumed in seed order so the
	// lowest failing seed wins and stop conditions match the sequential loop.
	results := make(chan doneUnit)
	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.worker(wctx, results)
		}()
	}
	go func() { wg.Wait(); close(results) }()
	return h.report(ctx, results, progress)
}

func (h *hunt) worker(wctx context.Context, results chan<- doneUnit) {
	for {
		lo := int(h.next.Add(1)-1)*h.step + 1
		hi := min(lo+h.step-1, h.spec.MaxIter)
		if lo > h.spec.MaxIter || int64(lo) > h.minFail.Load() || time.Now().After(h.deadline) || wctx.Err() != nil {
			return
		}
		r, ok := h.unit(wctx, lo, hi)
		if !ok {
			return
		}
		if r != nil {
			for m := h.minFail.Load(); int64(r.Seed) < m && !h.minFail.CompareAndSwap(m, int64(r.Seed)); m = h.minFail.Load() {
			}
		}
		select {
		case results <- doneUnit{lo, hi, r}:
		case <-wctx.Done():
			return
		}
	}
}

// report emits progress for passed seeds in order and returns at the first failure, or when results run out.
func (h *hunt) report(ctx context.Context, results <-chan doneUnit, progress func(i, seed int)) Result {
	pending := map[int]*doneUnit{}
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

// unit runs seeds lo..hi; nil means all passed.
func (h *hunt) unit(wctx context.Context, lo, hi int) (r *Result, ok bool) {
	if h.step > 1 && h.fast(wctx, lo, hi) {
		return nil, wctx.Err() == nil
	}
	for i := lo; i <= hi; i++ {
		if r, ok = h.iterate(wctx, i); r != nil || !ok {
			return
		}
	}
	return nil, true
}

// fast runs seeds lo..hi as one multi-test input; true means all passed. Anything else falls back to per-seed runs.
func (h *hunt) fast(wctx context.Context, lo, hi int) bool {
	var b strings.Builder
	b.WriteString(strconv.Itoa(hi-lo+1) + "\n")
	for i := lo; i <= hi; i++ {
		in, _, ok := h.genCase(wctx, i)
		if !ok {
			return false
		}
		b.WriteString(in)
	}
	n := time.Duration(hi - lo + 1)
	got, _, sc, stle := exec1(wctx, h.argv[1], b.String(), h.limit*n)
	if sc != 0 || stle {
		return false
	}
	want, _, bc, btle := exec1(wctx, h.argv[2], b.String(), 10*h.limit*n)
	if bc != 0 || btle {
		return false
	}
	ok, _ := runner.Compare(h.mode, want, got, h.eps)
	return ok
}

// iterate runs one seed; nil means it passed. ok is false if ctx was cancelled mid-run.
func (h *hunt) iterate(wctx context.Context, i int) (*Result, bool) {
	in, ge, gok := h.genCase(wctx, i)
	if wctx.Err() != nil {
		return nil, false
	}
	if h.spec.Multi > 1 {
		in = "1\n" + in
	}
	if !gok {
		return &Result{Kind: Error, Iterations: i, Seed: i, Err: "gen failed: " + ge}, true
	}
	got, r, ok := h.solve(wctx, i, in)
	if r != nil || !ok {
		return r, ok
	}
	want, r, ok := h.brute(wctx, i, in)
	if r != nil || !ok {
		return r, ok
	}
	return h.compare(i, in, want, got), true
}

// genCase is gen, then the validator (if any) must accept the input.
func (h *hunt) genCase(wctx context.Context, i int) (in, errOut string, ok bool) {
	in, errOut, ok = h.gen(wctx, i)
	if ok && h.argv[3] != nil {
		_, ve, vc, vtle := exec1(wctx, h.argv[3], in, h.limit)
		if (vc != 0 || vtle) && wctx.Err() == nil {
			return in, "validator rejected the generated input: " + strings.TrimSpace(ve), false
		}
	}
	return
}

// gen returns the Input for seed i. With Batch, one gen run serves a chunk of seeds.
func (h *hunt) gen(wctx context.Context, i int) (in, errOut string, ok bool) {
	if h.spec.Batch <= 1 {
		in, ge, gc, gtle := exec1(wctx, append(append([]string{}, h.argv[0]...), strconv.Itoa(i)), "", h.limit, "SIZE="+strconv.Itoa(sizeFor(i)))
		return in, ge, gc == 0 && !gtle
	}
	n, k := (i-1)/h.spec.Batch, (i-1)%h.spec.Batch
	c, _ := h.chunks.LoadOrStore(n, &batchChunk{})
	ck := c.(*batchChunk)
	ck.once.Do(func() { h.runChunk(wctx, ck, n) })
	if ck.err != "" || k >= len(ck.cases) {
		return "", ck.err, false
	}
	return ck.cases[k], "", true
}

// runChunk makes one batch gen run for chunk n and splits its output into cases.
func (h *hunt) runChunk(wctx context.Context, ck *batchChunk, n int) {
	first := n*h.spec.Batch + 1
	out, ge, gc, gtle := exec1(wctx, append(append([]string{}, h.argv[0]...), strconv.Itoa(first), strconv.Itoa(h.spec.Batch)), "", 10*h.limit, "SIZE="+strconv.Itoa(sizeFor(first)))
	if wctx.Err() != nil {
		return
	}
	if gc != 0 || gtle {
		ck.err = ge
		return
	}
	if ck.cases = splitCases(out); len(ck.cases) != h.spec.Batch {
		ck.err = fmt.Sprintf("batch gen printed %d cases, want %d (separate cases with a --- line)", len(ck.cases), h.spec.Batch)
	}
}

// solve runs the Solution on in. A failure comes back as the Result; ok is false if ctx was cancelled.
func (h *hunt) solve(wctx context.Context, i int, in string) (got string, r *Result, ok bool) {
	got, se, sc, stle := exec1(wctx, h.argv[1], in, h.limit)
	switch {
	case wctx.Err() != nil:
		return "", nil, false
	case stle:
		return "", &Result{Kind: SolutionTLE, Iterations: i, Seed: i, Input: in}, true
	case sc != 0:
		return "", &Result{Kind: SolutionRE, Iterations: i, Seed: i, Input: in, Err: se}, true
	}
	return got, nil, true
}

// brute runs the brute force on in. A failure comes back as the Result; ok is false if ctx was cancelled.
func (h *hunt) brute(wctx context.Context, i int, in string) (want string, r *Result, ok bool) {
	want, be, bc, btle := exec1(wctx, h.argv[2], in, 10*h.limit)
	switch {
	case wctx.Err() != nil:
		return "", nil, false
	case bc != 0 || btle:
		return "", &Result{Kind: Error, Iterations: i, Seed: i, Input: in, Err: "brute failed: " + be}, true
	}
	return want, nil, true
}

// compare is a Mismatch Result when want and got differ, else nil.
func (h *hunt) compare(i int, in, want, got string) *Result {
	if ok, mm := runner.Compare(h.mode, want, got, h.eps); !ok {
		return &Result{Kind: Mismatch, Iterations: i, Seed: i, Input: in, Want: want, Got: got, Mismatch: mm}
	}
	return nil
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
