// Package editor opens Solutions in Neovim: in a multiplexer split when possible, else suspend-and-resume.
package editor

import (
	"fmt"
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
	Mux     mux.Mux // nil: no multiplexer, use suspend
	Bin     string  // nvim executable
	SockDir string  // short directory for nvim's --listen socket

	mu   sync.Mutex
	pane *mux.Pane
	sock string
}

// Open opens path at line. With a multiplexer it returns (nil, nil): the editor lives in a split
// pane. A second Open reuses a live pane (`:edit +N`, then focus). Without one it returns the
// command to run in the foreground (tea.ExecProcess).
func (c *Controller) Open(path string, line int) (*exec.Cmd, error) {
	if c.Mux == nil {
		cmd := exec.Command(c.Bin, fmt.Sprintf("+%d", line), path)
		cmd.Dir = filepath.Dir(path)
		return cmd, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pane != nil && c.Mux.Alive(*c.pane) {
		if err := c.remoteEdit(path, line); err == nil {
			c.Mux.Focus(*c.pane) // best effort: herdr cannot focus by id
			return nil, nil
		}
		// nvim does not answer (hung or mid-exit): fall through and open a fresh pane
		c.Mux.Close(*c.pane)
	}
	return nil, c.openPane(filepath.Dir(path), fmt.Sprintf("+%d", line), path)
}

// openPane starts nvim in a new split with its --listen socket. The caller holds c.mu.
func (c *Controller) openPane(cwd string, args ...string) error {
	if err := os.MkdirAll(c.SockDir, 0o700); err != nil {
		return err
	}
	sock := filepath.Join(c.SockDir, fmt.Sprintf("nvim-%d.sock", os.Getpid()))
	os.Remove(sock) // stale socket from a crashed editor
	p, err := c.Mux.OpenEditor(cwd, append([]string{c.Bin, "--listen", sock}, args...))
	if err != nil {
		return err
	}
	c.pane, c.sock = &p, sock
	return nil
}

// OpenPair opens two files side by side (a Custom Test's .in and .ans), with the same
// pane / suspend behavior as Open.
func (c *Controller) OpenPair(left, right string) (*exec.Cmd, error) {
	if c.Mux == nil {
		cmd := exec.Command(c.Bin, "-O", left, right)
		cmd.Dir = filepath.Dir(left)
		return cmd, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pane != nil && c.Mux.Alive(*c.pane) {
		if err := c.remote(fmt.Sprintf("execute('edit ' . fnameescape('%s') . ' | vsplit ' . fnameescape('%s'))", vimQuote(left), vimQuote(right))); err == nil {
			c.Mux.Focus(*c.pane)
			return nil, nil
		}
		c.Mux.Close(*c.pane)
	}
	return nil, c.openPane(filepath.Dir(left), "-O", left, right)
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
