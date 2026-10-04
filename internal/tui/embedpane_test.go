package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/embed"
)

func startTerm(t *testing.T, script string) *embed.Term {
	t.Helper()
	term, err := embed.Start(t.TempDir(), []string{"sh", "-c", script}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)
	return term
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func embedModel(t *testing.T, term *embed.Term, w, h int) Model {
	m := New([]cf.Problem{{ContestID: 1, Index: "A", Name: "Theatre"}}, "", Deps{})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	nm, _ = nm.Update(embed.OpenedMsg{Term: term})
	return nm.(Model)
}

func TestEmbeddedLayoutIsTwoColumns(t *testing.T) {
	term := startTerm(t, `printf 'EDITOR-HERE'; sleep 5`)
	m := embedModel(t, term, 100, 12)
	waitUntil(t, "editor output", func() bool { return strings.Contains(plain(m), "EDITOR-HERE") })
	left, right := m.layout()
	if left != 40 || right != 59 {
		t.Fatalf("layout 100 cols at 0.4: left=%d right=%d", left, right)
	}
	lines := strings.Split(plain(m), "\n")
	if len(lines) != 12 {
		t.Fatalf("view should fill the window height: %d lines", len(lines))
	}
	for i, l := range lines {
		if w := utf8.RuneCountInString(l); w > 100 {
			t.Errorf("line %d is %d columns: %q", i, w, l)
		}
	}
	if !strings.Contains(lines[0], "Problems") || !strings.Contains(lines[0], "┃") || !strings.Contains(lines[0], "EDITOR-HERE") {
		t.Fatalf("verd on the left, divider, editor on the right:\n%s", plain(m))
	}
	if !strings.Contains(plain(m), "[editor focused, ctrl+\\ returns here]") {
		t.Fatalf("focus badge missing:\n%s", plain(m))
	}
	// the cursor is placed inside the pane while it has focus
	if v := m.View(); v.Cursor == nil || v.Cursor.X < left+1 {
		t.Fatalf("cursor should be in the editor column: %+v", v.Cursor)
	}
}

func TestEmbeddedFocusRoutesKeys(t *testing.T) {
	term := startTerm(t, `exec cat`) // the tty echoes typed characters
	m := embedModel(t, term, 120, 12)
	press := func(m Model, msg tea.Msg) (Model, tea.Cmd) {
		nm, cmd := m.Update(msg)
		return nm.(Model), cmd
	}
	// focused: keys (even q and ctrl+c) go to the editor, not to verd
	m, cmd := press(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Fatal("q must not quit verd while the editor has focus")
	}
	m, _ = press(m, tea.KeyPressMsg{Code: 'z', Text: "z"})
	waitUntil(t, "typed chars in the pane", func() bool { return strings.Contains(plain(m), "qz") })
	m, _ = press(m, tea.PasteMsg{Content: "PASTED"})
	waitUntil(t, "paste in the pane", func() bool { return strings.Contains(plain(m), "PASTED") })

	// ctrl+\ hands the keyboard back to verd: now q quits
	m, _ = press(m, tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})
	if m.embed.focus || !strings.Contains(plain(m), "[ctrl+\\ focuses the editor]") {
		t.Fatal("focus key should return to verd")
	}
	if v := m.View(); v.Cursor != nil {
		t.Fatal("no editor cursor while verd has focus")
	}
	_, cmd = press(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q should quit verd once it has focus")
	}
	// and the same key goes back to the editor
	m, _ = press(m, tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})
	if !m.embed.focus {
		t.Fatal("focus key toggles back")
	}
	// a custom focus key
	m.deps.FocusKey = "ctrl+g"
	m, _ = press(m, tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if m.embed.focus {
		t.Fatal("custom focus key should toggle")
	}
}

func TestEmbeddedResizePropagates(t *testing.T) {
	term := startTerm(t, `stty size; trap 'stty size' WINCH; while :; do sleep 0.05; done`)
	m := embedModel(t, term, 100, 20)
	_, right := m.layout()
	want := "20 " + itoa(right)
	waitUntil(t, "child sees the pane size", func() bool { return strings.Contains(plain(m), want) })
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m = nm.(Model)
	_, right = m.layout()
	want = "30 " + itoa(right)
	waitUntil(t, "child sees the new size", func() bool { return strings.Contains(plain(m), want) })
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestEmbeddedExitRestoresFullLayout(t *testing.T) {
	term := startTerm(t, `echo done`)
	m := embedModel(t, term, 100, 12)
	if !term.Wait(3 * time.Second) {
		t.Fatal("child should exit")
	}
	nm, _ := m.Update(embed.RedrawMsg{})
	m = nm.(Model)
	if m.embed.term != nil || m.editorOpen {
		t.Fatal("pane exit must drop the embedded editor")
	}
	if out := plain(m); strings.Contains(out, "┃") || strings.Contains(out, "│") || strings.Contains(out, "focused") {
		t.Fatalf("layout should be verd only again:\n%s", out)
	}
	// a redraw while still alive changes nothing
	term2 := startTerm(t, `sleep 5`)
	m = embedModel(t, term2, 100, 12)
	nm, _ = m.Update(embed.RedrawMsg{})
	if nm.(Model).embed.term == nil {
		t.Fatal("alive pane must stay")
	}
}

func TestEmbeddedNarrowWindowFallsBackToVerdOnly(t *testing.T) {
	term := startTerm(t, `sleep 5`)
	m := embedModel(t, term, 50, 12) // 0.4*50 = 20 left, 29 right: ok
	if _, right := m.layout(); right == 0 {
		t.Fatal("50 columns still fits")
	}
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12}) // left 16 < min
	m = nm.(Model)
	if _, right := m.layout(); right != 0 {
		t.Fatal("40 columns is too narrow for two panes")
	}
	if strings.Contains(plain(m), "┃") {
		t.Fatal("no divider when only verd is drawn")
	}
	// keys still go to the editor if it has focus? No pane drawn: verd must stay usable
	nm, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	_ = nm
	if cmd == nil {
		// focus defaults to the editor; the user can press the focus key; documented behavior
		nm, _ = m.Update(tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})
		_, cmd = nm.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
		if cmd == nil {
			t.Fatal("verd should be quittable after the focus key")
		}
	}
}

func TestEmbeddedMouseFocusAndForwarding(t *testing.T) {
	// the child turns on SGR mouse reporting and echoes the bytes it gets, made visible by cat -v
	term := startTerm(t, `printf '\033[?1000h\033[?1006h'; exec cat -v`)
	m := embedModel(t, term, 100, 12)
	time.Sleep(200 * time.Millisecond)
	left, _ := m.layout()
	// click on verd's side: focus moves to verd, nothing reaches the editor
	nm, _ := m.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	m = nm.(Model)
	if m.embed.focus {
		t.Fatal("clicking verd should take focus")
	}
	// click inside the pane at column left+1+4, row 2: the editor sees x=5,y=3 (1-based) and gets focus
	nm, _ = m.Update(tea.MouseClickMsg{X: left + 1 + 4, Y: 2, Button: tea.MouseLeft})
	m = nm.(Model)
	if !m.embed.focus {
		t.Fatal("clicking the pane should focus it")
	}
	waitUntil(t, "mouse report", func() bool { return strings.Contains(plain(m), "^[[<0;5;3M") })
	if m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("the view must request mouse events while a pane is shown")
	}
}
