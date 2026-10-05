package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/stress"
	"github.com/moneytosms/verd/internal/workspace"
)

// prepareStress resolves path to a Problem and builds its stress spec. Errors are *exitError code 2.
func prepareStress(ctx context.Context, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string, iter int, limit time.Duration) (workspace.Ref, stress.Spec, error) {
	ref, err := workspace.New(cfg.Workspace).Resolve(path, cfg.Lang)
	if err != nil {
		return ref, stress.Spec{}, &exitError{2, err.Error()}
	}
	d, err := detail(ctx, ref.Contest, ref.Index)
	if err != nil {
		return ref, stress.Spec{}, &exitError{2, err.Error()}
	}
	if d.Interactive {
		return ref, stress.Spec{}, &exitError{2, "interactive Problem: stress unsupported"}
	}
	mode := ""
	if savedMode != nil {
		mode = savedMode(ref.Contest, ref.Index)
	}
	spec, err := buildStress(cfg, cacheDir, ref, d, runner.Mode(mode), iter, limit)
	if err != nil {
		return ref, spec, &exitError{2, err.Error()}
	}
	return ref, spec, nil
}

// stressCmd runs `verd stress <file>`: gen + brute helpers are created from Templates if missing.
// Exit 0 only if no counterexample was found within the limits.
func stressCmd(ctx context.Context, out io.Writer, cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, path string, iter int, limit time.Duration) error {
	_, spec, err := prepareStress(ctx, cfg, cacheDir, detail, savedMode, path, iter, limit)
	if err != nil {
		return err
	}
	var r stress.Result
	for e := range stress.Run(ctx, spec) {
		if e.Done {
			r = e.Result
		}
	}
	return printStress(out, r)
}

// buildStress creates the gen and brute helpers from Templates if missing and assembles the spec.
// mode "" means the Problem's default Comparison Mode.
func buildStress(cfg config.Config, cacheDir string, ref workspace.Ref, d *scrape.Detail, mode runner.Mode, iter int, limit time.Duration) (stress.Spec, error) {
	ws := workspace.New(cfg.Workspace)
	l := cfg.Lang[ref.Lang]
	tdir := filepath.Join(config.Dir(), "templates")
	var progs [2]stress.Program
	for i, kind := range []string{"gen.", "brute."} {
		tmpl, _ := workspace.LoadKindTemplate(tdir, kind, ref.Lang, l)
		p, err := ws.EnsureKind(ref.Contest, ref.Index, kind, l, tmpl)
		if err != nil {
			return stress.Spec{}, err
		}
		progs[i] = stress.Program{Path: p, Lang: l}
	}
	if mode == "" {
		mode = runner.Mode(d.DefaultMode())
	}
	return stress.Spec{Gen: progs[0], Solution: stress.Program{Path: ref.Path, Lang: l}, Brute: progs[1],
		CacheDir: filepath.Join(cacheDir, "build"), Mode: mode, FloatEps: cfg.FloatEps,
		TimeLimitMS: int(float64(d.TimeLimitMS) * cfg.TimeMultiplier), MaxIter: iter, Timeout: limit}, nil
}

// printStress reports a finished stress run. Only "no counterexample" returns nil.
func printStress(out io.Writer, r stress.Result) error {
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

// stressHelpers are where a Solution's gen and brute live and whether either is still missing.
func stressHelpers(cfg config.Config, ref workspace.Ref) (gen, brute string, missing bool) {
	l := cfg.Lang[ref.Lang]
	dir := workspace.New(cfg.Workspace).Dir(ref.Contest, ref.Index)
	gen, brute = filepath.Join(dir, "gen."+l.Ext), filepath.Join(dir, "brute."+l.Ext)
	for _, p := range []string{gen, brute} {
		if _, err := os.Stat(p); err != nil {
			missing = true
		}
	}
	return
}
