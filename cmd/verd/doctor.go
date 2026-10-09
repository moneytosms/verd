package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Check: config file loads
	cfg, err := loadConfig(path)
	if err != nil {
		results = append(results, checkResult{"fail", "config", err.Error()})
	} else {
		results = append(results, checkResult{"ok", "config", "loads"})

		// Check: handle is set
		if cfg.Handle == "" {
			results = append(results, checkResult{"fail", "handle", "not set"})
		} else {
			results = append(results, checkResult{"ok", "handle", cfg.Handle})
		}

		// Check: workspace exists or is creatable
		ws := cfg.Workspace
		if ws == "" || ws == "." {
			ws = "."
		} else if strings.HasPrefix(ws, "~/") {
			home, _ := os.UserHomeDir()
			ws = filepath.Join(home, ws[2:])
		}

		if info, err := c.Stat(ws); err == nil {
			if !info.IsDir() {
				results = append(results, checkResult{"fail", "workspace", "not a directory"})
			} else {
				results = append(results, checkResult{"ok", "workspace", ws})
			}
		} else {
			// Try to create it
			if err := os.MkdirAll(ws, 0o755); err != nil {
				results = append(results, checkResult{"fail", "workspace", fmt.Sprintf("cannot create: %v", err)})
			} else {
				results = append(results, checkResult{"ok", "workspace", "creatable at " + ws})
			}
		}

		// Check: workspace not under /mnt/ on WSL
		if c.Getenv("WSL_DISTRO_NAME") != "" && strings.HasPrefix(ws, "/mnt/") {
			results = append(results, checkResult{"warn", "workspace", "under /mnt/ on WSL: slow I/O (use /tmp or $HOME)"})
		}

		// Check: each language's compiler/run is on PATH
		for langName, lang := range cfg.Lang {
			execName := ""
			if len(lang.Compile) > 0 {
				execName = lang.Compile[0]
			} else if len(lang.Run) > 0 {
				execName = lang.Run[0]
			}
			if execName != "" {
				if _, err := c.LookPath(execName); err != nil {
					results = append(results, checkResult{"fail", langName, execName + " not on PATH"})
				} else {
					results = append(results, checkResult{"ok", langName, execName})
				}
			}
		}

		// Check: editor command on PATH
		if cfg.Editor != "" {
			editorCmd := strings.Fields(cfg.Editor)[0]
			if _, err := c.LookPath(editorCmd); err != nil {
				results = append(results, checkResult{"fail", "editor", editorCmd + " not on PATH"})
			} else {
				results = append(results, checkResult{"ok", "editor", editorCmd})

				// Check: nvim version if editor is nvim
				if editorCmd == "nvim" {
					out, err := c.RunCmd("nvim", "--version")
					if err != nil {
						results = append(results, checkResult{"fail", "nvim version", "could not check"})
					} else {
						// Parse version from "nvim X.Y.Z ..." or "NVIM X.Y.Z ..."
						ver := strings.Fields(out)
						if len(ver) > 1 {
							versionStr := ver[1]
							if isVersionAtLeast(versionStr, "0.9") {
								results = append(results, checkResult{"ok", "nvim version", versionStr})
							} else {
								results = append(results, checkResult{"warn", "nvim version", fmt.Sprintf("%s (< 0.9)", versionStr)})
							}
						}
					}
				}
			}
		}
	}

	// Check: tmux or herdr presence (info only, no fail/warn)
	if _, err := c.LookPath("tmux"); err == nil {
		results = append(results, checkResult{"ok", "tmux", "available"})
	} else if _, err := c.LookPath("herdr"); err == nil {
		results = append(results, checkResult{"ok", "herdr", "available"})
	}

	// Check: terminal color support
	colorTerm := c.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		results = append(results, checkResult{"ok", "terminal colors", "truecolor"})
	} else if colorTerm != "" {
		results = append(results, checkResult{"warn", "terminal colors", colorTerm + " (named themes may look off)"})
	} else {
		results = append(results, checkResult{"warn", "terminal colors", "unknown (use truecolor terminal or 'terminal' theme)"})
	}

	// Check: $TERM not "dumb"
	term := c.Getenv("TERM")
	if term == "dumb" {
		results = append(results, checkResult{"fail", "TERM", "is 'dumb': verd cannot run here"})
	} else if term != "" {
		results = append(results, checkResult{"ok", "TERM", term})
	}

	// Print results
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
