// Package mux opens and controls an editor pane in a terminal multiplexer (tmux, herdr).
package mux

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Pane identifies a pane: "%3" (tmux) or "w1:p2" (herdr).
type Pane struct{ ID string }

type Mux interface {
	Name() string
	// OpenEditor splits the verd pane to the right without taking focus and runs argv there.
	OpenEditor(cwd string, argv []string) (Pane, error)
	// Alive is false once the pane's process exited or the pane is gone.
	Alive(p Pane) bool
	// Focus moves keyboard focus to the pane. Not every multiplexer can (see herdr).
	Focus(p Pane) error
	Close(p Pane) error
}

// ErrFocusUnsupported is returned by adapters whose CLI cannot focus a pane by id.
var ErrFocusUnsupported = errors.New("focusing a pane by id is not supported")

// Detect picks the multiplexer verd runs inside. mode is the `split` config value:
// "auto" detects (tmux before herdr, since herdr strips TMUX from its panes but tmux inherits
// HERDR_*); "tmux"/"herdr" force that adapter; anything else (suspend, embedded) returns nil.
func Detect(mode string, env func(string) string) Mux {
	switch mode {
	case "tmux":
		return NewTmux([]string{"tmux"}, env("TMUX_PANE"))
	case "herdr":
		return NewHerdr([]string{"herdr"}, env("HERDR_PANE_ID"))
	case "auto", "":
		if env("TMUX") != "" && env("TMUX_PANE") != "" {
			return NewTmux([]string{"tmux"}, env("TMUX_PANE"))
		}
		if env("HERDR_ENV") == "1" && env("HERDR_PANE_ID") != "" {
			return NewHerdr([]string{"herdr"}, env("HERDR_PANE_ID"))
		}
	}
	return nil
}

// Quote shell-quotes one argument. Both multiplexers hand a string to a shell.
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func shellLine(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		q[i] = Quote(a)
	}
	return strings.Join(q, " ")
}

// execer runs a command and returns its stdout. Swapped out in unit tests.
type execer func(name string, args ...string) ([]byte, error)

func realExec(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("%s %s: %w: %s", name, args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// Tmux drives panes with the tmux CLI.
type Tmux struct {
	prefix []string // e.g. {"tmux"} or {"tmux", "-L", "verd-test"}
	target string   // the pane to split (verd's own pane)
	run    execer
}

func NewTmux(prefix []string, target string) *Tmux {
	return &Tmux{prefix: prefix, target: target, run: realExec}
}

func (t *Tmux) Name() string { return "tmux" }

func (t *Tmux) cmd(args ...string) ([]byte, error) {
	return t.run(t.prefix[0], append(append([]string{}, t.prefix[1:]...), args...)...)
}

func (t *Tmux) OpenEditor(cwd string, argv []string) (Pane, error) {
	args := []string{"split-window", "-h", "-d", "-P", "-F", "#{pane_id}", "-c", cwd}
	if t.target != "" {
		args = append(args, "-t", t.target)
	}
	out, err := t.cmd(append(args, shellLine(argv))...)
	if err != nil {
		return Pane{}, err
	}
	return Pane{ID: strings.TrimSpace(string(out))}, nil
}

func (t *Tmux) Alive(p Pane) bool {
	out, err := t.cmd("list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		return false
	}
	for _, id := range strings.Fields(string(out)) {
		if id == p.ID {
			return true
		}
	}
	return false
}

func (t *Tmux) Focus(p Pane) error { _, err := t.cmd("select-pane", "-t", p.ID); return err }

func (t *Tmux) Close(p Pane) error { _, err := t.cmd("kill-pane", "-t", p.ID); return err }

// Herdr drives panes with the herdr CLI.
type Herdr struct {
	prefix []string // e.g. {"herdr"} or {"herdr", "--session", "x"}
	target string   // pane to split; empty = --current
	run    execer
}

func NewHerdr(prefix []string, target string) *Herdr {
	return &Herdr{prefix: prefix, target: target, run: realExec}
}

func (h *Herdr) Name() string { return "herdr" }

func (h *Herdr) cmd(args ...string) ([]byte, error) {
	return h.run(h.prefix[0], append(append([]string{}, h.prefix[1:]...), args...)...)
}

// OpenEditor splits, then runs argv with `exec` so the pane closes when the editor quits
// (herdr's split has no command argument; the new pane starts a shell).
func (h *Herdr) OpenEditor(cwd string, argv []string) (Pane, error) {
	args := []string{"pane", "split", "--direction", "right", "--no-focus", "--cwd", cwd}
	if h.target != "" {
		args = append(args, "--pane", h.target)
	} else {
		args = append(args, "--current")
	}
	out, err := h.cmd(args...)
	if err != nil {
		return Pane{}, err
	}
	var r struct {
		Result struct {
			Pane struct {
				ID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil || r.Result.Pane.ID == "" {
		return Pane{}, fmt.Errorf("herdr pane split: unexpected output %q", bytes.TrimSpace(out))
	}
	p := Pane{ID: r.Result.Pane.ID}
	if _, err := h.cmd("pane", "run", p.ID, "exec "+shellLine(argv)); err != nil {
		h.Close(p)
		return Pane{}, err
	}
	return p, nil
}

func (h *Herdr) Alive(p Pane) bool { _, err := h.cmd("pane", "get", p.ID); return err == nil }

// Focus: the herdr CLI can only focus by direction, never by pane id.
func (h *Herdr) Focus(Pane) error { return ErrFocusUnsupported }

func (h *Herdr) Close(p Pane) error { _, err := h.cmd("pane", "close", p.ID); return err }
