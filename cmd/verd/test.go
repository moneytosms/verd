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

// testCmd runs `verd test <file>` headless: samples to disk, compile (cached), run, print.
// Exit 0 only if every test is AC.
func testCmd(ctx context.Context, out io.Writer, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string) error {
	ws := workspace.New(cfg.Workspace)
	ref, err := ws.Resolve(path, cfg.Lang)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	d, err := detail(ctx, ref.Contest, ref.Index)
	if err != nil {
		return &exitError{2, fmt.Sprintf("loading %d%s: %v", ref.Contest, ref.Index, err)}
	}
	if d.Interactive {
		return &exitError{2, fmt.Sprintf("%d%s is interactive: local run unsupported", ref.Contest, ref.Index)}
	}
	mode := ""
	if savedMode != nil {
		mode = savedMode(ref.Contest, ref.Index)
	}
	spec, err := buildSpec(cfg, cacheDir, ref, d, runner.Mode(mode))
	if err != nil {
		return &exitError{2, err.Error()}
	}
	if len(spec.Tests) == 0 {
		return &exitError{2, "no tests found"}
	}
	fmt.Fprintf(out, "%d%s  %s  TL %d ms x%g  ML %d MB  %s\n", ref.Contest, ref.Index, ref.Lang, d.TimeLimitMS, cfg.TimeMultiplier, d.MemoryLimitMB, spec.Mode)

	var passed, total, maxMS int
	var maxMB float64
	overall := runner.AC
	for ev := range runner.Run(ctx, spec) {
		switch ev.Kind {
		case runner.CompileFinished:
			switch {
			case ev.Err != "":
				fmt.Fprintf(out, "compile: FAILED\n%s\n", ev.Err)
			case len(spec.Lang.Compile) == 0:
				fmt.Fprintln(out, "compile: none (interpreted)")
			case ev.Cached:
				fmt.Fprintln(out, "compile: ok (cached)")
			default:
				fmt.Fprintln(out, "compile: ok")
			}
		case runner.TestFinished:
			r := ev.Result
			total++
			if r.Verdict == runner.AC {
				passed++
			}
			maxMS, maxMB = max(maxMS, r.TimeMS), max(maxMB, r.MemoryMB)
			fmt.Fprintf(out, "%-10s %-3s %6d ms %7.1f MB%s\n", r.Name, r.Verdict, r.TimeMS, r.MemoryMB, detailOf(r))
		case runner.Done:
			overall = ev.Verdict
		}
	}
	if overall == runner.CE {
		return &exitError{1, ""}
	}
	fmt.Fprintf(out, "%d/%d AC  max %d ms  max %.1f MB  %s\n", passed, total, maxMS, maxMB, overall)
	if overall != runner.AC {
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
