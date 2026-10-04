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
	if err := os.MkdirAll(c.SockDir, 0o700); err != nil {
		return nil, err
	}
	sock := filepath.Join(c.SockDir, fmt.Sprintf("nvim-%d.sock", os.Getpid()))
	os.Remove(sock) // stale socket from a crashed editor
	argv := []string{c.Bin, "--listen", sock, fmt.Sprintf("+%d", line), path}
	p, err := c.Mux.OpenEditor(filepath.Dir(path), argv)
	if err != nil {
		return nil, err
	}
	c.pane, c.sock = &p, sock
	return nil, nil
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
	expr := fmt.Sprintf("execute('edit +%d ' . fnameescape('%s'))", line, strings.ReplaceAll(path, "'", "''"))
	out, err := exec.Command(c.Bin, "--server", c.sock, "--remote-expr", expr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nvim --remote-expr: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
