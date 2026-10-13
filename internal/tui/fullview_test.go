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
