package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moneytosms/verd/internal/config"
)

// Checks is an injectable set of functions for diagnostics, used by tests to fake I/O.
type Checks struct {
	LookPath func(file string) (string, error)
	Getenv   func(key string) string
	RunCmd   func(name string, args ...string) (string, error)
	Stat     func(path string) (os.FileInfo, error)
}

// DefaultChecks returns the real implementations.
func DefaultChecks() Checks {
	return Checks{
		LookPath: exec.LookPath,
		Getenv:   os.Getenv,
		RunCmd: func(name string, args ...string) (string, error) {
			out, err := exec.Command(name, args...).CombinedOutput()
			return string(out), err
		},
		Stat: os.Stat,
	}
}

type checkResult struct {
	level  string // "ok", "warn", "fail"
	name   string
	detail string
}

// doctorCmd runs diagnostics on the verd installation.
func doctorCmd(out io.Writer, path string, c Checks) error {
	var results []checkResult
	cfg, err := loadConfig(path)
	if err != nil {
		results = append(results, checkResult{"fail", "config", err.Error()})
	} else {
		results = append(results, configChecks(cfg, c)...)
	}
	results = append(results, multiplexerChecks(c)...)
	results = append(results, colorCheck(c))
	results = append(results, termChecks(c)...)
	return printChecks(out, results)
}

// configChecks runs the checks that need a loadable config.
func configChecks(cfg config.Config, c Checks) []checkResult {
	res := []checkResult{{"ok", "config", "loads"}, handleCheck(cfg)}
	res = append(res, workspaceChecks(cfg, c)...)
	res = append(res, langChecks(cfg, c)...)
	return append(res, editorChecks(cfg, c)...)
}

// handleCheck: the handle must be set.
func handleCheck(cfg config.Config) checkResult {
	if cfg.Handle == "" {
		return checkResult{"fail", "handle", "not set"}
	}
	return checkResult{"ok", "handle", cfg.Handle}
}

// workspaceChecks: the workspace exists or can be created, and is not under /mnt/ on WSL.
func workspaceChecks(cfg config.Config, c Checks) []checkResult {
	ws := workspacePath(cfg.Workspace)
	res := []checkResult{workspaceResult(ws, c)}
	if c.Getenv("WSL_DISTRO_NAME") != "" && strings.HasPrefix(ws, "/mnt/") {
		res = append(res, checkResult{"warn", "workspace", "under /mnt/ on WSL: slow I/O (use /tmp or $HOME)"})
	}
	return res
}

// workspacePath expands "" and "~/" in the configured workspace.
func workspacePath(ws string) string {
	switch {
	case ws == "" || ws == ".":
		return "."
	case strings.HasPrefix(ws, "~/"):
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ws[2:])
	}
	return ws
}

// workspaceResult: an existing directory is ok, an existing file fails, a missing path is created.
func workspaceResult(ws string, c Checks) checkResult {
	if info, err := c.Stat(ws); err == nil {
		if !info.IsDir() {
			return checkResult{"fail", "workspace", "not a directory"}
		}
		return checkResult{"ok", "workspace", ws}
	}
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return checkResult{"fail", "workspace", fmt.Sprintf("cannot create: %v", err)}
	}
	return checkResult{"ok", "workspace", "creatable at " + ws}
}

// langChecks: each language's compiler or run command is on PATH.
func langChecks(cfg config.Config, c Checks) []checkResult {
	var res []checkResult
	for langName, lang := range cfg.Lang {
		execName := ""
		if len(lang.Compile) > 0 {
			execName = lang.Compile[0]
		} else if len(lang.Run) > 0 {
			execName = lang.Run[0]
		}
		if execName == "" {
			continue
		}
		if _, err := c.LookPath(execName); err != nil {
			res = append(res, checkResult{"fail", langName, execName + " not on PATH"})
		} else {
			res = append(res, checkResult{"ok", langName, execName})
		}
	}
	return res
}

// editorChecks: the editor command is on PATH, and nvim is new enough.
func editorChecks(cfg config.Config, c Checks) []checkResult {
	if cfg.Editor == "" {
		return nil
	}
	editorCmd := strings.Fields(cfg.Editor)[0]
	if _, err := c.LookPath(editorCmd); err != nil {
		return []checkResult{{"fail", "editor", editorCmd + " not on PATH"}}
	}
	res := []checkResult{{"ok", "editor", editorCmd}}
	if editorCmd == "nvim" {
		res = append(res, nvimVersionChecks(c)...)
	}
	return res
}

// nvimVersionChecks parses "nvim X.Y.Z ..." or "NVIM X.Y.Z ..." and wants 0.9 or newer.
func nvimVersionChecks(c Checks) []checkResult {
	text, err := c.RunCmd("nvim", "--version")
	if err != nil {
		return []checkResult{{"fail", "nvim version", "could not check"}}
	}
	ver := strings.Fields(text)
	if len(ver) <= 1 {
		return nil
	}
	versionStr := ver[1]
	if isVersionAtLeast(versionStr, "0.9") {
		return []checkResult{{"ok", "nvim version", versionStr}}
	}
	return []checkResult{{"warn", "nvim version", fmt.Sprintf("%s (< 0.9)", versionStr)}}
}

// multiplexerChecks: tmux or herdr presence (info only, no fail/warn).
func multiplexerChecks(c Checks) []checkResult {
	if _, err := c.LookPath("tmux"); err == nil {
		return []checkResult{{"ok", "tmux", "available"}}
	}
	if _, err := c.LookPath("herdr"); err == nil {
		return []checkResult{{"ok", "herdr", "available"}}
	}
	return nil
}

// colorCheck: terminal color support, from COLORTERM.
func colorCheck(c Checks) checkResult {
	colorTerm := c.Getenv("COLORTERM")
	switch {
	case colorTerm == "truecolor" || colorTerm == "24bit":
		return checkResult{"ok", "terminal colors", "truecolor"}
	case colorTerm != "":
		return checkResult{"warn", "terminal colors", colorTerm + " (named themes may look off)"}
	}
	return checkResult{"warn", "terminal colors", "unknown (use truecolor terminal or 'terminal' theme)"}
}

// termChecks: $TERM must not be "dumb".
func termChecks(c Checks) []checkResult {
	term := c.Getenv("TERM")
	switch {
	case term == "dumb":
		return []checkResult{{"fail", "TERM", "is 'dumb': verd cannot run here"}}
	case term != "":
		return []checkResult{{"ok", "TERM", term}}
	}
	return nil
}

// printChecks prints each result and returns exitError 1 if any failed.
func printChecks(out io.Writer, results []checkResult) error {
	hasFail := false
	for _, r := range results {
		fmt.Fprintf(out, "%s  %-20s %s\n", r.level, r.name, r.detail)
		if r.level == "fail" {
			hasFail = true
		}
	}
	if hasFail {
		return &exitError{1, ""}
	}
	return nil
}

// isVersionAtLeast checks if version >= minVersion. Both are "X.Y.Z" style.
func isVersionAtLeast(version, minVersion string) bool {
	vParts := strings.Split(strings.Split(version, "-")[0], ".")
	minParts := strings.Split(minVersion, ".")

	for i := 0; i < len(minParts) && i < len(vParts); i++ {
		var vNum, minNum int
		fmt.Sscanf(vParts[i], "%d", &vNum)
		fmt.Sscanf(minParts[i], "%d", &minNum)
		if vNum > minNum {
			return true
		}
		if vNum < minNum {
			return false
		}
	}
	return true
}
