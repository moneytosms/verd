package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/stress"
	"github.com/moneytosms/verd/internal/workspace"
)

// stressCmd runs `verd stress <file>`: gen + brute helpers are created from Templates if missing.
// Exit 0 only if no counterexample was found within the limits.
func stressCmd(ctx context.Context, out io.Writer, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string, iter int, limit time.Duration) error {
	ws := workspace.New(cfg.Workspace)
	ref, err := ws.Resolve(path, cfg.Lang)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	d, err := detail(ctx, ref.Contest, ref.Index)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	if d.Interactive {
		return &exitError{2, "interactive Problem: stress unsupported"}
	}
	l := cfg.Lang[ref.Lang]
	tdir := filepath.Join(config.Dir(), "templates")
	var progs [2]stress.Program
	for i, kind := range []string{"gen.", "brute."} {
		tmpl, _ := workspace.LoadKindTemplate(tdir, kind, ref.Lang, l)
		p, err := ws.EnsureKind(ref.Contest, ref.Index, kind, l, tmpl)
		if err != nil {
			return &exitError{2, err.Error()}
		}
		progs[i] = stress.Program{Path: p, Lang: l}
	}
	mode := runner.Mode(d.DefaultMode())
	if savedMode != nil {
		if m := savedMode(ref.Contest, ref.Index); m != "" {
			mode = runner.Mode(m)
		}
	}
	spec := stress.Spec{Gen: progs[0], Solution: stress.Program{Path: ref.Path, Lang: l}, Brute: progs[1],
		CacheDir: filepath.Join(cacheDir, "build"), Mode: mode, FloatEps: cfg.FloatEps,
		TimeLimitMS: int(float64(d.TimeLimitMS) * cfg.TimeMultiplier), MaxIter: iter, Timeout: limit}
	var r stress.Result
	for e := range stress.Run(ctx, spec) {
		if e.Done {
			r = e.Result
		}
	}
	switch r.Kind {
	case stress.Limit:
		fmt.Fprintf(out, "no counterexample in %d iterations\n", r.Iterations)
		return nil
	case stress.Cancelled:
		return &exitError{1, "cancelled"}
	case stress.Error:
		return &exitError{2, r.Err}
	case stress.Mismatch:
		fmt.Fprintf(out, "MISMATCH at seed %d (iteration %d)\ninput:\n%s\nbrute:\n%s\nsolution:\n%s", r.Seed, r.Iterations, r.Input, r.Want, r.Got)
	default:
		fmt.Fprintf(out, "%s at seed %d (iteration %d)\ninput:\n%s%s\n", r.Kind, r.Seed, r.Iterations, r.Input, r.Err)
	}
	return &exitError{1, ""}
}
