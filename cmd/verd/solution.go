package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editor"
	"github.com/moneytosms/verd/internal/workspace"
)

func langKeys(cfg config.Config) []string {
	keys := make([]string, 0, len(cfg.Lang))
	for k := range cfg.Lang {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func langOf(cfg config.Config, lang string) (config.Lang, error) {
	l, ok := cfg.Lang[lang]
	if !ok {
		return config.Lang{}, fmt.Errorf("language %q is not configured", lang)
	}
	return l, nil
}

// solutionRef is where a Problem's Solution in lang lives (it may not exist yet).
func solutionRef(cfg config.Config, p cf.Problem, lang string) (workspace.Ref, error) {
	l, err := langOf(cfg, lang)
	if err != nil {
		return workspace.Ref{}, err
	}
	dir := workspace.New(cfg.Workspace).Dir(p.ContestID, p.Index)
	return workspace.Ref{Contest: p.ContestID, Index: p.Index, Lang: lang, Path: filepath.Join(dir, "main."+l.Ext)}, nil
}

// ensureSolution creates the Solution from the user's Template if absent; it never overwrites.
// line is where the editor cursor should start.
func ensureSolution(cfg config.Config, p cf.Problem, lang string) (path string, line int, err error) {
	l, err := langOf(cfg, lang)
	if err != nil {
		return "", 0, err
	}
	tmpl, err := workspace.LoadTemplate(filepath.Join(config.Dir(), "templates"), lang, l)
	if err != nil {
		return "", 0, err
	}
	vars := workspace.NewVars(p.ContestID, p.Index, p.Name, cfg.Handle, time.Now())
	path, line, _, err = workspace.New(cfg.Workspace).Ensure(p.ContestID, p.Index, lang, l, tmpl, vars)
	return path, line, err
}

func requireEditor(ctrl *editor.Controller) error {
	if _, err := exec.LookPath(ctrl.Bin); err != nil {
		return fmt.Errorf("editor %q not found in PATH (set `editor` in the config or Settings)", ctrl.Bin)
	}
	return nil
}

// editOpen opens the Solution in the editor: in a split pane (nil command) or, without a
// multiplexer, as a foreground command to suspend for.
func editOpen(cfg config.Config, ctrl *editor.Controller, p cf.Problem, lang string) (*exec.Cmd, error) {
	if err := requireEditor(ctrl); err != nil {
		return nil, err
	}
	path, line, err := ensureSolution(cfg, p, lang)
	if err != nil {
		return nil, err
	}
	return ctrl.Open(path, line)
}

// addCustom creates the next Custom Test files and opens .in and .ans side by side.
func addCustom(cfg config.Config, ctrl *editor.Controller, p cf.Problem) (*exec.Cmd, error) {
	if err := requireEditor(ctrl); err != nil {
		return nil, err
	}
	_, in, ans, err := workspace.New(cfg.Workspace).NextCustom(p.ContestID, p.Index)
	if err != nil {
		return nil, err
	}
	return ctrl.OpenPair(in, ans)
}
