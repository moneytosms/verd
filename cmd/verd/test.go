package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/workspace"
)

// exitError carries a process exit code. An empty msg means "already printed".
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// detailFunc returns a Problem's scraped detail (cache-first).
type detailFunc func(ctx context.Context, contest int, index string) (*scrape.Detail, error)

// modeFunc returns the user's saved Comparison Mode for a Problem ("" = none saved).
type modeFunc func(contest int, index string) string

// testPrep is a resolved, ready-to-run `verd test` request.
type testPrep struct {
	ref    workspace.Ref
	spec   runner.Spec
	header string
}

// prepareTest resolves path to a Problem, loads its detail, materializes Sample Tests and builds
// the Runner spec. Errors are *exitError with code 2.
func prepareTest(ctx context.Context, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string) (*testPrep, error) {
	ws := workspace.New(cfg.Workspace)
	ref, err := ws.Resolve(path, cfg.Lang)
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	d, err := detail(ctx, ref.Contest, ref.Index)
	if err != nil {
		return nil, &exitError{2, fmt.Sprintf("loading %s: %v", ref.Code(), err)}
	}
	if d.Interactive {
		return nil, &exitError{2, fmt.Sprintf("%s is interactive: local run unsupported", ref.Code())}
	}
	mode := ""
	if savedMode != nil {
		mode = savedMode(ref.Contest, ref.Index)
	}
	spec, err := buildSpec(cfg, cacheDir, ref, d, runner.Mode(mode))
	if err != nil {
		return nil, &exitError{2, err.Error()}
	}
	if len(spec.Tests) == 0 {
		return nil, &exitError{2, "no tests found"}
	}
	header := fmt.Sprintf("%s  %s  TL %d ms x%g  ML %d MB  %s", ref.Code(), ref.Lang, d.TimeLimitMS, cfg.TimeMultiplier, d.MemoryLimitMB, spec.Mode)
	return &testPrep{ref, spec, header}, nil
}

// testCmd runs `verd test <file>` headless: samples to disk, compile (cached), run, print.
// Exit 0 only if every test is AC.
func testCmd(ctx context.Context, out io.Writer, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string) error {
	prep, err := prepareTest(ctx, cfg, cacheDir, detail, savedMode, path)
	if err != nil {
		return err
	}
	rep := &reporter{out: out}
	rep.header(prep.header)
	for ev := range runner.Run(ctx, prep.spec) {
		rep.event(ev)
	}
	return rep.finish()
}

// reporter prints a Test Run, whether it ran in-process or was streamed from a running TUI.
type reporter struct {
	out               io.Writer
	passed, total, ms int
	mb                float64
	overall           string
}

func (r *reporter) header(line string) { fmt.Fprintln(r.out, line) }

func (r *reporter) event(ev runner.Event) {
	switch ev.Kind {
	case runner.CompileFinished:
		switch {
		case ev.Err != "":
			fmt.Fprintf(r.out, "compile: FAILED\n%s\n", ev.Err)
		case ev.Interpreted:
			fmt.Fprintln(r.out, "compile: none (interpreted)")
		case ev.Cached:
			fmt.Fprintln(r.out, "compile: ok (cached)")
		default:
			fmt.Fprintln(r.out, "compile: ok")
		}
	case runner.TestFinished:
		res := ev.Result
		r.total++
		if res.Verdict == runner.AC {
			r.passed++
		}
		r.ms, r.mb = max(r.ms, res.TimeMS), max(r.mb, res.MemoryMB)
		fmt.Fprintf(r.out, "%-10s %-3s %6d ms %7.1f MB%s\n", res.Name, res.Verdict, res.TimeMS, res.MemoryMB, detailOf(res))
	case runner.Done:
		r.overall = ev.Verdict
	}
}

// finish prints the summary and returns the exit status: nil only if every test was AC.
func (r *reporter) finish() error {
	switch r.overall {
	case runner.CE:
		return &exitError{1, ""}
	case "":
		return &exitError{2, "run ended without a result"}
	}
	fmt.Fprintf(r.out, "%d/%d AC  max %d ms  max %.1f MB  %s\n", r.passed, r.total, r.ms, r.mb, r.overall)
	if r.overall != runner.AC {
		return &exitError{1, ""}
	}
	return nil
}

func detailOf(r runner.Result) string {
	switch {
	case r.Mismatch != nil:
		m := r.Mismatch
		return fmt.Sprintf("   line %d col %d: want %q got %q", m.Line, m.Col, m.Want, m.Got)
	case r.Note != "":
		return "   " + r.Note
	case r.Verdict == runner.RE:
		return fmt.Sprintf("   exit %d %s", r.ExitCode, firstLine(r.Stderr))
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " ..."
	}
	return s
}

// buildSpec materializes Sample Tests next to the Solution and assembles the Runner spec.
// An empty mode means the Problem's default.
func buildSpec(cfg config.Config, cacheDir string, ref workspace.Ref, d *scrape.Detail, mode runner.Mode) (runner.Spec, error) {
	ws := workspace.New(cfg.Workspace)
	if _, err := os.Stat(ref.Path); err != nil {
		return runner.Spec{}, fmt.Errorf("no Solution at %s", ref.Path)
	}
	if err := ws.WriteSamples(ref.Contest, ref.Index, d.Samples); err != nil {
		return runner.Spec{}, err
	}
	tests, err := ws.Tests(ref.Contest, ref.Index)
	if err != nil {
		return runner.Spec{}, err
	}
	if mode == "" {
		mode = runner.Mode(d.DefaultMode())
	}
	return runner.Spec{
		Solution: ref.Path, Lang: cfg.Lang[ref.Lang], CacheDir: filepath.Join(cacheDir, "build"), Tests: tests,
		TimeLimitMS: d.TimeLimitMS, MemoryMB: d.MemoryLimitMB, Multiplier: cfg.TimeMultiplier, Mode: mode, FloatEps: cfg.FloatEps,
	}, nil
}
