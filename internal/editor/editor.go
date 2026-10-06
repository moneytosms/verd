// Package editor opens Solutions in the user's editor (Neovim by default): in a multiplexer split when
// possible, else suspend-and-resume. Only Neovim gets the reuse-the-open-pane treatment.
package editor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/moneytosms/verd/internal/mux"
)

// Controller owns at most one editor pane. Open is called from the Bubble Tea update loop,
// Alive from its tick; the mutex keeps the pane state consistent between them.
type Controller struct {
	Mux     mux.Mux  // nil: no multiplexer, use suspend
	Bin     string   // editor executable
	Args    []string // extra arguments from the editor setting, e.g. {"-w"} for "code -w"
	SockDir string   // short directory for nvim's --listen socket

	mu   sync.Mutex
	pane *mux.Pane
	sock string
}

// SetEditor switches the editor to a command line such as "nvim", "hx" or "code -w". The first
// word is resolved through PATH when it can be.
func (c *Controller) SetEditor(command string) {
	f := strings.Fields(command)
	if len(f) == 0 {
		f = []string{"nvim"}
	}
	bin := f[0]
	if p, err := exec.LookPath(bin); err == nil {
		bin = p
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Bin, c.Args, c.pane = bin, f[1:], nil
}

// profile is how an editor takes "open this file at this line" and "open these two side by side".
type profile struct {
	gui  bool // opens its own window: never put it in a split pane
	nvim bool // supports the --listen / --remote-expr reuse
	line func(path string, line int) []string
	pair func(l, r string) []string
}

func vi(path string, line int) []string { return []string{fmt.Sprintf("+%d", line), path} }
func two(l, r string) []string          { return []string{l, r} }

func (c *Controller) profile() profile {
	base := strings.TrimSuffix(filepath.Base(c.Bin), filepath.Ext(c.Bin))
	switch base {
	case "nvim":
		return profile{nvim: true, line: vi, pair: func(l, r string) []string { return []string{"-O", l, r} }}
	case "vim", "vi":
		return profile{line: vi, pair: func(l, r string) []string { return []string{"-O", l, r} }}
	case "hx", "helix":
		return profile{line: func(p string, n int) []string { return []string{fmt.Sprintf("%s:%d", p, n)} }, pair: two}
	case "code", "codium", "cursor":
		return profile{gui: true, line: func(p string, n int) []string { return []string{"--reuse-window", "-g", fmt.Sprintf("%s:%d", p, n)} }, pair: func(l, r string) []string { return []string{"--reuse-window", l, r} }}
	case "zed":
		return profile{gui: true, line: func(p string, n int) []string { return []string{"--existing", fmt.Sprintf("%s:%d", p, n)} }, pair: func(l, r string) []string { return []string{"--existing", l, r} }}
	case "subl":
		return profile{gui: true, line: func(p string, n int) []string { return []string{fmt.Sprintf("%s:%d", p, n)} }, pair: two}
	}
	return profile{line: vi, pair: two} // nano, micro, emacs, kak and most others take +N file
}

func (c *Controller) argv(args []string) []string {
	return append(append([]string{c.Bin}, c.Args...), args...)
}

// SetMux switches the multiplexer (a changed `split` setting); a pane of the old one is forgotten.
func (c *Controller) SetMux(m mux.Mux) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Mux, c.pane = m, nil
}

// Open opens path at line. With a multiplexer it returns (nil, nil): the editor lives in a split
// pane. A second Open reuses a live pane (`:edit +N`, then focus). Without one it returns the
// command to run in the foreground (tea.ExecProcess).
func (c *Controller) Open(path string, line int) (*exec.Cmd, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pr := c.profile()
	args := pr.line(path, line)
	if pr.gui {
		return nil, c.launchGUI(filepath.Dir(path), args)
	}
	if c.Mux == nil {
		return c.foreground(filepath.Dir(path), args), nil
	}
	if c.pane != nil && c.Mux.Alive(*c.pane) {
		if pr.nvim {
			if err := c.remoteEdit(path, line); err == nil {
				c.Mux.Focus(*c.pane) // best effort: herdr cannot focus by id
				return nil, nil
			}
		}
		// not nvim, or nvim does not answer (hung or mid-exit): open a fresh pane
		c.Mux.Close(*c.pane)
	}
	return nil, c.openPane(filepath.Dir(path), pr.nvim, args...)
}

// openPane starts nvim in a new split with its --listen socket. The caller holds c.mu.
func (c *Controller) openPane(cwd string, nvim bool, args ...string) error {
	sock := ""
	if nvim {
		if err := os.MkdirAll(c.SockDir, 0o700); err != nil {
			return err
		}
		sock = filepath.Join(c.SockDir, fmt.Sprintf("nvim-%d.sock", os.Getpid()))
		os.Remove(sock) // stale socket from a crashed editor
		args = append([]string{"--listen", sock}, args...)
	}
	p, err := c.Mux.OpenEditor(cwd, c.argv(args))
	if err != nil {
		return err
	}
	c.pane, c.sock = &p, sock
	return nil
}

// foreground is the editor as a command to run in the foreground (suspend, or a GUI editor).
func (c *Controller) foreground(dir string, args []string) *exec.Cmd {
	argv := c.argv(args)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	return cmd
}

// OpenPair opens two files side by side (a Custom Test's .in and .ans), with the same
// pane / suspend behavior as Open.
func (c *Controller) OpenPair(left, right string) (*exec.Cmd, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pr := c.profile()
	args := pr.pair(left, right)
	if pr.gui {
		return nil, c.launchGUI(filepath.Dir(left), args)
	}
	if c.Mux == nil {
		return c.foreground(filepath.Dir(left), args), nil
	}
	if c.pane != nil && c.Mux.Alive(*c.pane) {
		if pr.nvim {
			if err := c.remote(fmt.Sprintf("execute('edit ' . fnameescape('%s') . ' | vsplit ' . fnameescape('%s'))", vimQuote(left), vimQuote(right))); err == nil {
				c.Mux.Focus(*c.pane)
				return nil, nil
			}
		}
		c.Mux.Close(*c.pane)
	}
	return nil, c.openPane(filepath.Dir(left), pr.nvim, args...)
}

// Alive reports whether a split-pane editor is still open.
func (c *Controller) Alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pane != nil && c.Mux != nil && c.Mux.Alive(*c.pane)
}

// remoteEdit tells the running nvim to edit path at line. :edit (not --remote +N, which
// nvim 0.12 mangles) via an expression, so the editor's current mode does not matter.
func (c *Controller) remoteEdit(path string, line int) error {
	return c.remote(fmt.Sprintf("execute('edit +%d ' . fnameescape('%s'))", line, vimQuote(path)))
}

func vimQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func (c *Controller) remote(expr string) error {
	out, err := exec.Command(c.Bin, "--server", c.sock, "--remote-expr", expr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nvim --remote-expr: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GUI launchers return quickly; keep their output and lifecycle outside the terminal UI.
func (c *Controller) launchGUI(dir string, args []string) error {
	cmd := c.foreground(dir, args)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
