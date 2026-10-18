package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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

// testOpts are the `verd test` flags: -v prints every test's input, expected, stdout and stderr;
// -t a,b runs only those tests.
type testOpts struct {
	verbose bool
	only    []string
}

// parseTestArgs reads `verd test` arguments: one file plus -v and -t NAME in any order.
func parseTestArgs(args []string) (file string, opts testOpts, ok bool) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-v":
			opts.verbose = true
		case a == "-t" && i+1 < len(args):
			i++
			opts.only = strings.Split(args[i], ",")
		case file == "" && !strings.HasPrefix(a, "-"):
			file = a
		default:
			return "", opts, false
		}
	}
	return file, opts, file != ""
}

// testCmd runs `verd test <file>` headless: samples to disk, compile (cached), run, print.
// Exit 0 only if every test is AC.
func testCmd(ctx context.Context, out io.Writer, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string, opts testOpts) error {
	prep, err := prepareTest(ctx, cfg, cacheDir, detail, savedMode, path)
	if err != nil {
		return err
	}
	for _, name := range opts.only {
		if !slices.ContainsFunc(prep.spec.Tests, func(t workspace.Test) bool { return t.Name == name }) {
			return &exitError{2, fmt.Sprintf("no test %q (see tests/ next to the Solution)", name)}
		}
	}
	prep.spec.Only = opts.only
	rep := &reporter{out: out, verbose: opts.verbose}
	rep.header(prep.header)
	for ev := range runner.Run(ctx, prep.spec) {
		rep.event(ev)
	}
	return rep.finish()
}

// reporter prints a Test Run, whether it ran in-process or was streamed from a running TUI.
type reporter struct {
	out               io.Writer
	verbose           bool
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
		if x := runner.Explain(res); x.Hint != "" {
			fmt.Fprintf(r.out, "  why: %s\n", x.Hint)
			if x.Loc != "" {
				fmt.Fprintf(r.out, "  at:  %s\n", x.Loc)
			}
		}
		if r.verbose {
			r.sections(res)
			break
		}
		if e := strings.TrimRight(res.Stderr, "\n"); e != "" && res.Verdict != runner.AC {
			fmt.Fprintf(r.out, "  stderr:\n    %s\n", strings.ReplaceAll(e, "\n", "\n    "))
		}
		if in := strings.TrimRight(res.Input, "\n"); in != "" && (res.Verdict == runner.RE || res.Verdict == runner.TLE || res.Verdict == runner.MLE) {
			fmt.Fprintf(r.out, "  input:\n    %s\n", strings.ReplaceAll(in, "\n", "\n    "))
		}
	case runner.Done:
		r.overall = ev.Verdict
	}
}

// sections prints a test's input, expected, stdout and stderr, whatever its verdict.
func (r *reporter) sections(res runner.Result) {
	for _, sec := range []struct{ name, text string }{{"input", res.Input}, {"expected", res.Expected}, {"stdout", res.Output}, {"stderr", res.Stderr}} {
		text := strings.TrimRight(sec.text, "\n")
		if text == "" {
			text = "(empty)"
		}
		fmt.Fprintf(r.out, "  %s:\n    %s\n", sec.name, strings.ReplaceAll(text, "\n", "\n    "))
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
	case r.Detail() != "":
		return "   " + r.Detail()
	case r.Note != "":
		return "   " + r.Note
	}
	return ""
}

// buildSpec materializes Sample Tests next to the Solution and assembles the Runner spec.
// An empty mode means the Problem's default.
func buildSpec(cfg config.Config, cacheDir string, ref workspace.Ref, d *scrape.Detail, mode runner.Mode) (runner.Spec, error) {
	ws := workspace.New(cfg.Workspace)
	if _, err := os.Stat(ref.Path); err != nil {
		return runner.Spec{}, fmt.Errorf("no Solution at %s", ref.Path)
	}
	if err := workspace.EnsureInclude(); err != nil {
		return runner.Spec{}, err
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
	// Use language-specific overrides if set, else global config.
	timeMultiplier := cfg.TimeMultiplier
	floatEps := cfg.FloatEps
	memoryMB := float64(d.MemoryLimitMB)
	lang := cfg.Lang[ref.Lang]
	if lang.TimeMultiplier != 0 {
		timeMultiplier = lang.TimeMultiplier
	}
	if lang.FloatEps != 0 {
		floatEps = lang.FloatEps
	}
	if lang.MemoryMultiplier != 0 {
		memoryMB *= lang.MemoryMultiplier
	}
	var checker []string
	if c := filepath.Join(filepath.Dir(ref.Path), "checker."+lang.Ext); fileExists(c) {
		if checker, _, err = runner.Build(context.Background(), lang, c, filepath.Join(cacheDir, "build")); err != nil {
			return runner.Spec{}, fmt.Errorf("checker does not build: %w", err)
		}
	}
	return runner.Spec{
		Checker:  checker,
		Solution: ref.Path, Lang: lang, CacheDir: filepath.Join(cacheDir, "build"), Tests: tests,
		TimeLimitMS: d.TimeLimitMS, MemoryMB: int(memoryMB), Multiplier: timeMultiplier, Mode: mode, FloatEps: floatEps,
	}, nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
