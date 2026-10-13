package tui

import (
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/runner"
)

func TestFullTestView(t *testing.T) {
	wa := runner.Result{Name: "sample-1", Verdict: runner.WA, Expected: "EXPWANT\n", Output: "STDOUTGOT\n", Stderr: "DBGERR line1\nline2\n", Mismatch: &runner.Mismatch{Line: 1, Col: 1, Want: "a", Got: "b"}}
	m, ch := openWithTests(t, []runner.Event{
		{Kind: runner.TestFinished, Result: wa},
		{Kind: runner.Done, Verdict: runner.WA},
	})
	m, cmd := send(m, "t")
	m, cmd = step(t, m, cmd)
	m, _ = step(t, m, cmd)
	close(ch)
	m, _ = send(m, "f")
	if !m.full {
		t.Fatal("f should open the full view")
	}
	out := plain(m)
	for _, want := range []string{"Input", "Expected", "EXPWANT", "Output", "STDOUTGOT", "Stderr", "DBGERR line1"} {
		if !strings.Contains(out, want) {
			t.Errorf("full view missing %q:\n%s", want, out)
		}
	}
	m, _ = send(m, "j")
	if m.fullOff != 1 {
		t.Fatalf("j should scroll, off=%d", m.fullOff)
	}
	m, _ = send(m, "esc")
	if m.full || m.open == nil {
		t.Fatal("esc closes the full view, not the Problem")
	}
}

func TestDiffLines(t *testing.T) {
	want := []string{"1 2", "3.0", "x"}
	got := []string{"1  2", "3.0000001", "y", "extra"}
	bw, bg := diffLines(runner.Tokens, want, got)
	if bw[0] || bg[0] || !bw[2] || !bg[2] || !bg[3] || !bg[1] {
		t.Fatalf("tokens: %v %v", bw, bg)
	}
	bw, bg = diffLines(runner.Float, want, got)
	if bg[1] || bw[1] {
		t.Fatalf("float should accept 3.0000001: %v %v", bw, bg)
	}
	bw, bg = diffLines(runner.Exact, want, got)
	if !bg[0] || !bw[0] {
		t.Fatal("exact should flag the spacing difference")
	}
	if bw, bg = diffLines(runner.None, want, got); bw[2] || bg[2] {
		t.Fatal("none never marks")
	}
}

func TestFullViewPanAndSections(t *testing.T) {
	long := strings.Repeat("0123456789", 30)
	wa := runner.Result{Name: "sample-1", Verdict: runner.WA, TimeMS: 7, MemoryMB: 2.5, Expected: "ok\n", Output: long + "\n", Mismatch: &runner.Mismatch{Line: 1, Col: 1, Want: "ok", Got: "0123"}}
	m, ch := openWithTests(t, []runner.Event{{Kind: runner.TestFinished, Result: wa}, {Kind: runner.Done, Verdict: runner.WA}})
	m, cmd := send(m, "t")
	m, cmd = step(t, m, cmd)
	m, _ = step(t, m, cmd)
	close(ch)
	m, _ = send(m, "f")
	out := plain(m)
	for _, want := range []string{"7 ms", "2.5 MB", "first difference: line 1 col 1", "! 0123456789", "(empty)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	m, _ = send(m, "l")
	if m.fullX != 8 {
		t.Fatalf("l should pan, x=%d", m.fullX)
	}
	m, _ = send(m, "w")
	if !m.fullWrap || m.fullX != 0 {
		t.Fatal("w should switch to wrap")
	}
	m, _ = send(m, "3")
	if m.fullOff == 0 {
		t.Fatal("3 should jump to the Output section")
	}
}
