package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/mux"
)

func TestSuspendWithoutMux(t *testing.T) {
	c := &Controller{Bin: "nvim"}
	cmd, err := c.Open("/w/1/A/main.cpp", 7)
	if err != nil || cmd == nil || strings.Join(cmd.Args, " ") != "nvim +7 /w/1/A/main.cpp" || cmd.Dir != "/w/1/A" {
		t.Fatalf("%v %v", cmd, err)
	}
	if c.Alive() {
		t.Fatal("no pane in suspend mode")
	}
}

func TestOpenPairSuspend(t *testing.T) {
	c := &Controller{Bin: "nvim"}
	cmd, err := c.OpenPair("/w/1/A/tests/custom-1.in", "/w/1/A/tests/custom-1.ans")
	if err != nil || strings.Join(cmd.Args, " ") != "nvim -O /w/1/A/tests/custom-1.in /w/1/A/tests/custom-1.ans" || cmd.Dir != "/w/1/A/tests" {
		t.Fatalf("%v %v", cmd, err)
	}
}

func waitFor(cond func() bool) bool {
	for i := 0; i < 80; i++ {
		if cond() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// tmuxRig is a real tmux session with a Controller that splits inside it.
type tmuxRig struct {
	t    *testing.T
	sock string
	self string
	dir  string
	c    *Controller
}

// tmx runs a tmux command on the rig's socket and returns its trimmed output.
func (r *tmuxRig) tmx(args ...string) string {
	out, err := exec.Command("tmux", append([]string{"-L", r.sock}, args...)...).CombinedOutput()
	if err != nil {
		r.t.Fatalf("tmux %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *tmuxRig) panes() int {
	return len(strings.Fields(r.tmx("list-panes", "-t", "t", "-F", "#{pane_id}")))
}

// remote evaluates expr in the nvim behind the Controller's --listen socket.
func (r *tmuxRig) remote(expr string) string {
	out, _ := exec.Command("nvim", "--server", r.c.sock, "--remote-expr", expr).Output()
	return strings.TrimSpace(string(out))
}

// Real tmux + real nvim: the second Open must focus the existing pane, not split again.
func TestSecondOpenReusesPaneTmux(t *testing.T) {
	for _, bin := range []string{"tmux", "nvim"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	r := &tmuxRig{t: t, sock: "verd-test-editor", dir: t.TempDir()}
	r.tmx("new-session", "-d", "-s", "t", "-x", "120", "-y", "30")
	t.Cleanup(func() { exec.Command("tmux", "-L", r.sock, "kill-server").Run() })
	r.self = r.tmx("display-message", "-p", "-t", "t", "#{pane_id}")
	r.c = &Controller{Mux: mux.NewTmux([]string{"tmux", "-L", r.sock}, r.self), Bin: "nvim", SockDir: t.TempDir()}

	a, b := filepath.Join(r.dir, "a.cpp"), filepath.Join(r.dir, "it's b.cpp")
	os.WriteFile(a, []byte("a\n"), 0o644)
	os.WriteFile(b, []byte("b1\nb2\nb3\n"), 0o644)

	checkFirstOpen(r, a)
	checkSecondOpen(r, b)
	checkReopenAfterQuit(r, a)
	checkOpenPair(r)
}

// checkFirstOpen: pane mode returns no foreground cmd, and the first Open splits.
func checkFirstOpen(r *tmuxRig, a string) {
	if cmd, err := r.c.Open(a, 1); err != nil || cmd != nil {
		r.t.Fatalf("pane mode returns no foreground cmd: %v %v", cmd, err)
	}
	if !r.c.Alive() || r.panes() != 2 {
		r.t.Fatalf("first Open should split: alive=%v panes=%d", r.c.Alive(), r.panes())
	}
	if !waitFor(func() bool { return r.remote("expand('%:t')") == "a.cpp" }) {
		r.t.Fatal("nvim did not come up on its --listen socket with a.cpp")
	}
}

// checkSecondOpen: the second Open reuses the pane and focuses it.
func checkSecondOpen(r *tmuxRig, b string) {
	if cmd, err := r.c.Open(b, 3); err != nil || cmd != nil {
		r.t.Fatalf("second Open: %v %v", cmd, err)
	}
	if r.panes() != 2 {
		r.t.Fatalf("second Open must reuse the pane, got %d panes", r.panes())
	}
	if got := r.remote("expand('%:t') . ':' . line('.')"); got != "it's b.cpp:3" {
		r.t.Fatalf("nvim should now edit b.cpp at line 3, got %q", got)
	}
	if active := r.tmx("display-message", "-p", "-t", "t", "#{pane_id}"); active == r.self {
		r.t.Fatal("second Open should focus the editor pane")
	}
}

// checkReopenAfterQuit: closing nvim is noticed, and the next Open starts a fresh pane.
func checkReopenAfterQuit(r *tmuxRig, a string) {
	exec.Command("nvim", "--server", r.c.sock, "--remote-send", "<C-\\><C-N>:qa!<CR>").Run()
	if !waitFor(func() bool { return !r.c.Alive() }) {
		r.t.Fatal("Alive should turn false after nvim quits")
	}
	if cmd, err := r.c.Open(a, 1); err != nil || cmd != nil || !r.c.Alive() || r.panes() != 2 {
		r.t.Fatalf("reopen after close: %v %v alive=%v panes=%d", cmd, err, r.c.Alive(), r.panes())
	}
}

// checkOpenPair: a Custom Test's two files open side by side in the same pane (wait for the reopened nvim).
func checkOpenPair(r *tmuxRig) {
	if !waitFor(func() bool { return r.remote("1") == "1" }) {
		r.t.Fatal("reopened nvim never answered")
	}
	in, ans := filepath.Join(r.dir, "custom-1.in"), filepath.Join(r.dir, "custom-1.ans")
	os.WriteFile(in, nil, 0o644)
	os.WriteFile(ans, nil, 0o644)
	if cmd, err := r.c.OpenPair(in, ans); err != nil || cmd != nil {
		r.t.Fatalf("OpenPair: %v %v", cmd, err)
	}
	if r.panes() != 2 {
		r.t.Fatalf("OpenPair must reuse the pane, got %d", r.panes())
	}
	if got := r.remote("winnr('$') . ':' . expand('%:t')"); got != "2:custom-1.ans" {
		r.t.Fatalf("want 2 windows with .ans focused after vsplit, got %q", got)
	}
}

func TestEditorCommandLines(t *testing.T) {
	for _, tc := range []struct{ editor, want string }{
		{"vim", "vim +7 /w/main.cpp"},
		{"hx", "hx /w/main.cpp:7"},
		{"nano", "nano +7 /w/main.cpp"},
		{"code -w", "code -w -g /w/main.cpp:7"},
		{"nvim", "nvim +7 /w/main.cpp"},
	} {
		c := &Controller{}
		c.SetEditor(tc.editor)
		cmd, err := c.Open("/w/main.cpp", 7)
		got := ""
		if cmd != nil {
			got = strings.Join(append([]string{filepath.Base(cmd.Args[0])}, cmd.Args[1:]...), " ")
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q (%v) want %q", tc.editor, got, err, tc.want)
		}
	}
}

// fakeMux records splits so a GUI or non-nvim editor can be checked without tmux.
type fakeMux struct {
	opened [][]string
	closed int
	alive  bool
}

func (f *fakeMux) Name() string { return "fake" }
func (f *fakeMux) OpenEditor(_ string, argv []string) (mux.Pane, error) {
	f.opened, f.alive = append(f.opened, argv), true
	return mux.Pane{ID: "p"}, nil
}
func (f *fakeMux) Alive(mux.Pane) bool  { return f.alive }
func (f *fakeMux) Focus(mux.Pane) error { return nil }
func (f *fakeMux) Close(mux.Pane) error { f.closed++; f.alive = false; return nil }

func TestNonNvimEditorsInAMux(t *testing.T) {
	f := &fakeMux{}
	c := &Controller{Mux: f, SockDir: t.TempDir()}
	c.SetEditor("vim")
	if cmd, err := c.Open("/w/a.cpp", 3); cmd != nil || err != nil || strings.Join(f.opened[0], " ") != strings.Join([]string{c.Bin, "+3", "/w/a.cpp"}, " ") {
		t.Fatalf("vim opens in a split with no --listen: %v %v %v", cmd, err, f.opened)
	}
	if _, err := c.Open("/w/b.cpp", 1); err != nil || f.closed != 1 || len(f.opened) != 2 {
		t.Fatalf("a second open replaces the pane: closed=%d opened=%d", f.closed, len(f.opened))
	}
	c.SetEditor("code")
	if cmd, err := c.Open("/w/c.cpp", 2); cmd == nil || err != nil || len(f.opened) != 2 {
		t.Fatalf("a GUI editor never uses a split: %v %v", cmd, err)
	}
}
