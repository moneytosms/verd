package embed

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func plain(s string) string { return sgr.ReplaceAllString(s, "") }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func start(t *testing.T, w, h int, script string) *Term {
	t.Helper()
	term, err := Start(t.TempDir(), []string{"sh", "-c", script}, w, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)
	return term
}

// A program using SGR colors, cursor addressing and the alternate screen renders as expected.
func TestRendersSGRCursorAndAltScreen(t *testing.T) {
	term := start(t, 30, 6, `printf 'main\033[?1049h\033[2J\033[1;1H\033[31mred\033[0m \033[1mbold\033[0m\033[3;5Hxy\033[5;1H\033[48;2;1;2;3mbg\033[0m'; sleep 5`)
	waitFor(t, "alt screen content", func() bool { return strings.Contains(plain(term.Render()), "xy") })
	raw := term.Render()
	lines := strings.Split(plain(raw), "\n")
	if len(lines) != 6 {
		t.Fatalf("height: %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[0], "red bold") {
		t.Fatalf("row 1: %q", lines[0])
	}
	if lines[2][4:6] != "xy" { // CUP row 3, col 5
		t.Fatalf("cursor addressing: row 3 = %q", lines[2])
	}
	if strings.Contains(plain(raw), "main") {
		t.Fatal("alt screen must hide the main screen's text")
	}
	if !strings.Contains(raw, "\x1b[") || !regexp.MustCompile(`\x1b\[[0-9;]*31m|38;5;1m|31;`).MatchString(raw) {
		t.Fatalf("red SGR missing from render: %q", raw)
	}
	if !strings.Contains(raw, "48;2;1;2;3") {
		t.Fatalf("truecolor background missing: %q", raw)
	}
	if x, y := term.Cursor(); x != 2 || y != 4 {
		t.Fatalf("cursor after 'bg' on row 5 should be (2,4), got (%d,%d)", x, y)
	}
}

// Resize reaches the child: `stty size` inside the PTY reports the new dimensions.
func TestResizeVisibleToChild(t *testing.T) {
	term := start(t, 40, 10, `stty size; trap 'stty size' WINCH; while :; do sleep 0.05; done`)
	waitFor(t, "initial size", func() bool { return strings.Contains(plain(term.Render()), "10 40") })
	term.Resize(55, 17)
	waitFor(t, "resized size", func() bool { return strings.Contains(plain(term.Render()), "17 55") })
	lines := strings.Split(plain(term.Render()), "\n")
	if len(lines) != 17 {
		t.Fatalf("emulator height after resize: %d", len(lines))
	}
}

func TestExitDetectedAndNotified(t *testing.T) {
	notified := make(chan struct{}, 8)
	term, err := Start(t.TempDir(), []string{"sh", "-c", "echo bye"}, 20, 3, func() {
		select {
		case notified <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	if !term.Wait(3 * time.Second) {
		t.Fatal("exit not detected")
	}
	if term.Alive() {
		t.Fatal("Alive must be false after exit")
	}
	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("exit should notify so the UI can react")
	}
	if !strings.Contains(plain(term.Render()), "bye") {
		t.Fatal("output before exit must remain visible")
	}
	term.Close() // idempotent
	if _, err := Start(t.TempDir(), nil, 10, 3, nil); err == nil {
		t.Fatal("empty command must error")
	}
}

// capture runs a raw-mode program that reports, as hex, every byte it receives until input goes
// quiet, after setup (terminal mode switches) and send (the events under test) ran.
func capture(t *testing.T, setup string, send func(*Term)) string {
	t.Helper()
	script := setup + `exec python3 -c "
import os, select, sys, tty
tty.setraw(0)
sys.stdout.write('READY'); sys.stdout.flush()
buf = b''
while select.select([0], [], [], 0.15)[0]:
    d = os.read(0, 4096)
    if not d: break
    buf += d
sys.stdout.write(chr(10) + 'GOT:' + buf.hex()); sys.stdout.flush()
"`
	term := start(t, 240, 4, script)
	waitFor(t, "child ready", func() bool { return strings.Contains(plain(term.Render()), "READY") })
	send(term)
	var got string
	waitFor(t, "captured bytes", func() bool {
		s := plain(term.Render())
		if i := strings.Index(s, "GOT:"); i >= 0 {
			got = strings.TrimSpace(strings.Split(s[i+4:], "\n")[0])
			return true
		}
		return false
	})
	return got
}

func hexOf(s string) string { return fmt.Sprintf("%x", s) }

// What the program receives for each key: the encoding table.
func TestKeyEncoding(t *testing.T) {
	press := func(code rune, mod tea.KeyMod, text string) tea.KeyPressMsg {
		return tea.KeyPressMsg{Code: code, Mod: mod, Text: text}
	}
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		{"letter", press('a', 0, "a"), "a"},
		{"shifted letter", press('a', tea.ModShift, "A"), "A"},
		{"unicode", press('é', 0, "é"), "é"},
		{"space", press(tea.KeySpace, 0, " "), " "},
		{"enter", press(tea.KeyEnter, 0, ""), "\r"},
		{"tab", press(tea.KeyTab, 0, ""), "\t"},
		{"shift+tab", press(tea.KeyTab, tea.ModShift, ""), "\x1b[Z"},
		{"backspace", press(tea.KeyBackspace, 0, ""), "\x7f"},
		{"escape", press(tea.KeyEscape, 0, ""), "\x1b"},
		{"ctrl+c", press('c', tea.ModCtrl, ""), "\x03"},
		{"ctrl+w", press('w', tea.ModCtrl, ""), "\x17"},
		{"ctrl+\\", press('\\', tea.ModCtrl, ""), "\x1c"},
		{"alt+x", press('x', tea.ModAlt, ""), "\x1bx"},
		{"up", press(tea.KeyUp, 0, ""), "\x1b[A"},
		{"down", press(tea.KeyDown, 0, ""), "\x1b[B"},
		{"right", press(tea.KeyRight, 0, ""), "\x1b[C"},
		{"left", press(tea.KeyLeft, 0, ""), "\x1b[D"},
		{"home", press(tea.KeyHome, 0, ""), "\x1b[H"},
		{"delete", press(tea.KeyDelete, 0, ""), "\x1b[3~"},
		{"page up", press(tea.KeyPgUp, 0, ""), "\x1b[5~"},
		{"f1", press(tea.KeyF1, 0, ""), "\x1bOP"},
		{"f5", press(tea.KeyF5, 0, ""), "\x1b[15~"},
		{"ctrl+up", press(tea.KeyUp, tea.ModCtrl, ""), "\x1b[1;5A"},
		{"shift+left", press(tea.KeyLeft, tea.ModShift, ""), "\x1b[1;2D"},
		{"alt+ctrl+right", press(tea.KeyRight, tea.ModAlt|tea.ModCtrl, ""), "\x1b[1;7C"},
		{"ctrl+delete", press(tea.KeyDelete, tea.ModCtrl, ""), "\x1b[3;5~"},
	} {
		got := capture(t, "", func(term *Term) { term.SendKey(tc.key) })
		if got != hexOf(tc.want) {
			t.Errorf("%s: got hex %s want %q (%s)", tc.name, got, tc.want, hexOf(tc.want))
		}
	}
}

// Mouse and paste reach a program that asked for them, as xterm SGR reports and bracketed paste.
func TestMouseAndPasteEncoding(t *testing.T) {
	setup := `printf '\033[?1000h\033[?1006h\033[?2004h'; `
	got := capture(t, setup, func(term *Term) {
		term.SendMouse(tea.MouseClickMsg{X: 4, Y: 2, Button: tea.MouseLeft})
		term.SendMouse(tea.MouseReleaseMsg{X: 4, Y: 2, Button: tea.MouseLeft})
		term.SendMouse(tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelDown})
		term.Paste("hi")
	})
	want := hexOf("\x1b[<0;5;3M" + "\x1b[<0;5;3m" + "\x1b[<65;2;2M" + "\x1b[200~hi\x1b[201~")
	if got != want {
		raw, _ := hex.DecodeString(got)
		t.Fatalf("got %q\nwant %q", raw, "\x1b[<0;5;3M\x1b[<0;5;3m\x1b[<65;2;2M\x1b[200~hi\x1b[201~")
	}
	// without the modes enabled, mouse is not reported and paste is not bracketed
	got = capture(t, "", func(term *Term) {
		term.SendMouse(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
		term.Paste("hi")
	})
	if got != hexOf("hi") {
		raw, _ := hex.DecodeString(got)
		t.Fatalf("mouse/paste without modes: got %q", raw)
	}
}

// Smoke test with the real thing: Neovim starts, shows its UI, and quits on :q.
func TestNeovimSmoke(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim not installed")
	}
	term, err := Start(t.TempDir(), []string{"nvim", "--clean", "-n", "-c", "set nonumber"}, 80, 24, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	waitFor(t, "nvim UI", func() bool {
		s := plain(term.Render())
		return strings.Contains(s, "~") && strings.Contains(s, "0,0-1")
	})
	// type into it through the key encoder: i h i <esc>
	for _, k := range []tea.KeyPressMsg{{Code: 'i', Text: "i"}, {Code: 'h', Text: "h"}, {Code: 'i', Text: "i"}, {Code: tea.KeyEscape}} {
		term.SendKey(k)
	}
	waitFor(t, "typed text", func() bool { return strings.Contains(plain(term.Render()), "hi") })
	for _, r := range ":q!" {
		term.SendKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	term.SendKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !term.Wait(5 * time.Second) {
		t.Fatalf("nvim should exit after :q!; screen:\n%s", plain(term.Render()))
	}
}
