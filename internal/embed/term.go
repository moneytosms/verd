// Package embed hosts a real program (Neovim) in a PTY and renders it with a terminal emulator,
// so Bubble Tea can draw it as a pane. The emulator (charmbracelet/x/vt) has no Kitty keyboard
// support: keys are encoded the classic xterm way, so a few chords degrade (see docs).
package embed

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Term is one program running in a PTY, mirrored into an emulator.
type Term struct {
	em   *vt.SafeEmulator
	ptmx *os.File
	cmd  *exec.Cmd
	done chan struct{} // closed once the program has exited and been reaped

	closeOnce sync.Once
	notify    func()
	dirty     atomic.Bool
	timerOn   atomic.Bool
	stopped   atomic.Bool
}

// Start runs argv in cwd inside a w x h PTY. notify is called (at most about 60 times a second)
// whenever the screen may have changed or the program exited.
func Start(cwd string, argv []string, w, h int, notify func()) (*Term, error) {
	if len(argv) == 0 {
		return nil, errors.New("embed: empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
	if err != nil {
		return nil, err
	}
	t := &Term{em: vt.NewSafeEmulator(w, h), ptmx: ptmx, cmd: cmd, done: make(chan struct{}), notify: notify}
	go t.pump()
	go t.feedBack()
	return t, nil
}

// pump copies the program's output into the emulator until it exits.
func (t *Term) pump() {
	buf := make([]byte, 32<<10)
	for {
		n, err := t.ptmx.Read(buf)
		if n > 0 {
			t.em.Write(buf[:n])
			t.touch()
		}
		if err != nil {
			break
		}
	}
	t.cmd.Wait()
	close(t.done)
	// Wake feedBack so it can notice the exit. (The emulator's own Close races with its Read.)
	t.stopped.Store(true)
	t.em.SendText("\x00")
	t.ping()
}

// feedBack sends whatever the emulator produces (encoded keys, answers to terminal queries) to the program.
func (t *Term) feedBack() {
	buf := make([]byte, 4096)
	for {
		n, err := t.em.Read(buf)
		if t.stopped.Load() || err != nil {
			return
		}
		if n > 0 {
			t.ptmx.Write(buf[:n])
		}
	}
}

// touch marks the screen dirty and schedules one notification.
func (t *Term) touch() {
	t.dirty.Store(true)
	if t.timerOn.CompareAndSwap(false, true) {
		time.AfterFunc(16*time.Millisecond, func() {
			t.timerOn.Store(false)
			if t.dirty.Swap(false) {
				t.ping()
			}
		})
	}
}

func (t *Term) ping() {
	if t.notify != nil {
		t.notify()
	}
}

// Alive reports whether the program is still running.
func (t *Term) Alive() bool {
	select {
	case <-t.done:
		return false
	default:
		return true
	}
}

// Wait blocks until the program has exited.
func (t *Term) Wait(timeout time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Close kills the program (if running) and releases the PTY.
func (t *Term) Close() {
	t.closeOnce.Do(func() {
		if t.Alive() && t.cmd.Process != nil {
			t.cmd.Process.Kill()
		}
		t.ptmx.Close()
	})
}

// Resize changes the PTY and emulator size; the program gets SIGWINCH.
func (t *Term) Resize(w, h int) {
	if w < 1 || h < 1 {
		return
	}
	pty.Setsize(t.ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
	t.em.Resize(w, h)
	t.touch()
}

// Render returns the screen as a styled string, h lines of at most w cells.
func (t *Term) Render() string { return t.em.Render() }

// Cursor is the cursor position within the screen.
func (t *Term) Cursor() (x, y int) {
	p := t.em.CursorPosition()
	return p.X, p.Y
}

func (t *Term) Focus() { t.em.Focus() }
func (t *Term) Blur()  { t.em.Blur() }

// SendKey encodes a key press for the program the classic xterm way.
func (t *Term) SendKey(k tea.KeyPressMsg) {
	if seq, ok := modifiedKey(k); ok {
		t.em.SendText(seq) // the emulator does not encode modified cursor/editing keys
		return
	}
	if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		t.em.SendText(k.Text) // printable text, including shifted and non-ASCII
		return
	}
	t.em.SendKey(uv.KeyPressEvent(k))
}

// modifiedKey encodes cursor and editing keys with modifiers as CSI 1;m X / CSI n;m ~ (xterm).
func modifiedKey(k tea.KeyPressMsg) (string, bool) {
	m := 1
	if k.Mod&tea.ModShift != 0 {
		m += 1
	}
	if k.Mod&tea.ModAlt != 0 {
		m += 2
	}
	if k.Mod&tea.ModCtrl != 0 {
		m += 4
	}
	if k.Mod&tea.ModMeta != 0 {
		m += 8
	}
	if m == 1 {
		return "", false
	}
	final := map[rune]string{tea.KeyUp: "A", tea.KeyDown: "B", tea.KeyRight: "C", tea.KeyLeft: "D", tea.KeyHome: "H", tea.KeyEnd: "F"}
	if f, ok := final[k.Code]; ok {
		return fmt.Sprintf("\x1b[1;%d%s", m, f), true
	}
	tilde := map[rune]int{tea.KeyInsert: 2, tea.KeyDelete: 3, tea.KeyPgUp: 5, tea.KeyPgDown: 6}
	if n, ok := tilde[k.Code]; ok {
		return fmt.Sprintf("\x1b[%d;%d~", n, m), true
	}
	return "", false
}

// SendMouse encodes a mouse event (coordinates already relative to the pane) for the program.
// The program only receives it if it enabled mouse reporting.
func (t *Term) SendMouse(m tea.MouseMsg) {
	mm := m.Mouse()
	u := uv.Mouse{X: mm.X, Y: mm.Y, Button: mm.Button, Mod: mm.Mod}
	switch m.(type) {
	case tea.MouseClickMsg:
		t.em.SendMouse(uv.MouseClickEvent(u))
	case tea.MouseReleaseMsg:
		t.em.SendMouse(uv.MouseReleaseEvent(u))
	case tea.MouseWheelMsg:
		t.em.SendMouse(uv.MouseWheelEvent(u))
	case tea.MouseMotionMsg:
		t.em.SendMouse(uv.MouseMotionEvent(u))
	}
}

// Paste sends text, wrapped in bracketed-paste markers if the program asked for them.
func (t *Term) Paste(text string) { t.em.Paste(text) }
