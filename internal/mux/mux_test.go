package mux

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDetect(t *testing.T) {
	tmuxEnv := map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0", "TMUX_PANE": "%3"}
	herdrEnv := map[string]string{"HERDR_ENV": "1", "HERDR_PANE_ID": "w1:p2"}
	both := map[string]string{"TMUX": "x", "TMUX_PANE": "%1", "HERDR_ENV": "1", "HERDR_PANE_ID": "w1:p1"}
	for _, tc := range []struct {
		name string
		mode string
		env  map[string]string
		want string // "" = nil
	}{
		{"tmux", "auto", tmuxEnv, "tmux"},
		{"herdr", "auto", herdrEnv, "herdr"},
		{"tmux wins inside herdr", "auto", both, "tmux"},
		{"neither", "auto", map[string]string{}, ""},
		{"tmux var without pane", "auto", map[string]string{"TMUX": "x"}, ""},
		{"herdr needs ENV=1", "auto", map[string]string{"HERDR_PANE_ID": "w1:p1"}, ""},
		{"empty mode is auto", "", tmuxEnv, "tmux"},
		{"force herdr over tmux", "herdr", both, "herdr"},
		{"force tmux", "tmux", tmuxEnv, "tmux"},
		{"force tmux outside tmux falls back", "tmux", herdrEnv, ""},
		{"force herdr outside herdr falls back", "herdr", tmuxEnv, ""},
		{"suspend disables", "suspend", tmuxEnv, ""},
		{"embedded disables", "embedded", tmuxEnv, ""},
	} {
		got := Detect(tc.mode, envOf(tc.env))
		name := ""
		if got != nil {
			name = got.Name()
		}
		if name != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, name, tc.want)
		}
	}
}

func TestQuote(t *testing.T) {
	if got := shellLine([]string{"nvim", "+3", "/p/it's here/main.cpp"}); got != `'nvim' '+3' '/p/it'\''s here/main.cpp'` {
		t.Fatal(got)
	}
}

type call struct {
	name string
	args []string
}

func fake(out map[string]string, errOn string, calls *[]call) execer {
	return func(name string, args ...string) ([]byte, error) {
		*calls = append(*calls, call{name, args})
		key := strings.Join(args, " ")
		if errOn != "" && strings.Contains(key, errOn) {
			return nil, errors.New("boom")
		}
		for k, v := range out {
			if strings.Contains(key, k) {
				return []byte(v), nil
			}
		}
		return nil, nil
	}
}

func TestTmuxCommands(t *testing.T) {
	var calls []call
	tm := NewTmux([]string{"tmux", "-L", "x"}, "%1")
	tm.run = fake(map[string]string{"split-window": "%7\n", "list-panes": "%1\n%7\n"}, "", &calls)
	p, err := tm.OpenEditor("/w/1/A", []string{"nvim", "--listen", "/s", "/w/1/A/main.cpp"})
	if err != nil || p.ID != "%7" {
		t.Fatalf("%v %v", p, err)
	}
	got := strings.Join(calls[0].args, " ")
	want := "-L x split-window -h -d -P -F #{pane_id} -c /w/1/A -t %1 'nvim' '--listen' '/s' '/w/1/A/main.cpp'"
	if calls[0].name != "tmux" || got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if !tm.Alive(p) || tm.Alive(Pane{"%9"}) {
		t.Fatal("Alive should check list-panes")
	}
	tm.Focus(p)
	tm.Close(p)
	if g := strings.Join(calls[len(calls)-2].args, " "); g != "-L x select-pane -t %7" {
		t.Fatal(g)
	}
	if g := strings.Join(calls[len(calls)-1].args, " "); g != "-L x kill-pane -t %7" {
		t.Fatal(g)
	}
}

func TestHerdrCommands(t *testing.T) {
	var calls []call
	h := NewHerdr([]string{"herdr"}, "")
	h.run = fake(map[string]string{"pane split": `{"result":{"pane":{"pane_id":"w1:p4"}}}`}, "", &calls)
	p, err := h.OpenEditor("/w", []string{"nvim", "/w/main.py"})
	if err != nil || p.ID != "w1:p4" {
		t.Fatalf("%v %v", p, err)
	}
	if g := strings.Join(calls[0].args, " "); g != "pane split --direction right --no-focus --cwd /w --current" {
		t.Fatal(g)
	}
	if g := strings.Join(calls[1].args, " "); g != "pane run w1:p4 exec 'nvim' '/w/main.py'" {
		t.Fatalf("must exec so the pane closes with the editor: %s", g)
	}
	calls = nil
	if err := h.Focus(p); err != nil {
		t.Fatal(err)
	}
	if g := strings.Join(calls[0].args, " "); g != "pane focus --direction right --current" {
		t.Fatal(g)
	}
	// run failure closes the half-made pane
	calls = nil
	h.run = fake(map[string]string{"pane split": `{"result":{"pane":{"pane_id":"w1:p5"}}}`}, "pane run", &calls)
	if _, err := h.OpenEditor("/w", []string{"nvim"}); err == nil {
		t.Fatal("want error")
	}
	if g := strings.Join(calls[len(calls)-1].args, " "); g != "pane close w1:p5" {
		t.Fatalf("leaked pane: %s", g)
	}
	// garbage output
	h.run = fake(map[string]string{"pane split": "nope"}, "", &calls)
	if _, err := h.OpenEditor("/w", []string{"nvim"}); err == nil {
		t.Fatal("garbage must error")
	}
}

func waitFor(cond func() bool) bool {
	for i := 0; i < 50; i++ {
		if cond() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// A throwaway tmux server (`-L`) proves the real commands work.
func TestTmuxReal(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	sock := "verd-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	tmx := func(args ...string) string {
		out, err := exec.Command("tmux", append([]string{"-L", sock}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	tmx("new-session", "-d", "-s", "t", "-x", "100", "-y", "30")
	t.Cleanup(func() { exec.Command("tmux", "-L", sock, "kill-server").Run() })
	self := tmx("display-message", "-p", "-t", "t", "#{pane_id}")

	m := NewTmux([]string{"tmux", "-L", sock}, self)
	p, err := m.OpenEditor(os.TempDir(), []string{"sleep", "30"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.ID, "%") || !m.Alive(p) {
		t.Fatalf("pane %v alive=%v", p, m.Alive(p))
	}
	if n := len(strings.Fields(tmx("list-panes", "-t", "t", "-F", "#{pane_id}"))); n != 2 {
		t.Fatalf("want a split (2 panes), got %d", n)
	}
	if active := tmx("display-message", "-p", "-t", "t", "#{pane_id}"); active != self {
		t.Fatal("split must not steal focus")
	}
	if err := m.Focus(p); err != nil || tmx("display-message", "-p", "-t", "t", "#{pane_id}") != p.ID {
		t.Fatalf("focus: %v", err)
	}
	if err := m.Close(p); err != nil || !waitFor(func() bool { return !m.Alive(p) }) {
		t.Fatal("close should end the pane")
	}
	// pane process exiting closes the pane too
	p2, _ := m.OpenEditor(os.TempDir(), []string{"true"})
	if !waitFor(func() bool { return !m.Alive(p2) }) {
		t.Fatal("exited editor must read as not alive")
	}
}

// A throwaway herdr session (own headless server) proves the real commands work.
func TestHerdrReal(t *testing.T) {
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not installed")
	}
	session := "verdtest" + strings.ToLower(strings.NewReplacer("/", "", "_", "").Replace(t.Name())) + "x"
	hd := func(args ...string) ([]byte, error) {
		return exec.Command("herdr", append([]string{"--session", session}, args...)...).Output()
	}
	srv := exec.Command("herdr", "--session", session, "server")
	if err := srv.Start(); err != nil {
		t.Skipf("cannot start herdr server: %v", err)
	}
	t.Cleanup(func() { hd("server", "stop"); srv.Wait() })
	if !waitFor(func() bool { _, err := hd("pane", "list"); return err == nil }) {
		t.Skip("herdr server did not come up")
	}
	hd("workspace", "create", "--cwd", os.TempDir(), "--no-focus") // a fresh headless server has no pane yet
	var out []byte
	waitFor(func() bool { out, _ = hd("pane", "list"); return strings.Contains(string(out), `"pane_id":"`) })
	i := strings.Index(string(out), `"pane_id":"`)
	if i < 0 {
		t.Skip("no herdr pane to split")
	}
	self := strings.SplitN(string(out)[i+len(`"pane_id":"`):], `"`, 2)[0]

	m := NewHerdr([]string{"herdr", "--session", session}, self)
	p, err := m.OpenEditor(os.TempDir(), []string{"sleep", "30"})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Alive(p) {
		t.Fatalf("pane %v should be alive", p)
	}
	if err := m.Close(p); err != nil || !waitFor(func() bool { return !m.Alive(p) }) {
		t.Fatal("close should end the pane")
	}
	// exec: the editor exiting removes the pane
	p2, err := m.OpenEditor(os.TempDir(), []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(func() bool { return !m.Alive(p2) }) {
		t.Fatal("exited editor must read as not alive")
	}
}
