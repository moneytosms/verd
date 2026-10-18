package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
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
	t.Log(out)
	for _, want := range []string{"7 ms", "2.5 MB", "first difference: line 1 col 1", "! 0123456789", "(empty)", "1/1 lines differ"} {
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

func TestFullViewRerunsOnlyThatTest(t *testing.T) {
	m, ch := openWithTests(t, []runner.Event{
		{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-1", Verdict: runner.WA}},
		{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-2", Verdict: runner.AC}},
		{Kind: runner.Done, Verdict: runner.WA},
	})
	only, orig := "unset", m.deps.Tests
	m.deps.Tests = func(ctx context.Context, p cf.Problem, d *scrape.Detail, o RunOpts) (<-chan runner.Event, error) {
		only = strings.Join(o.Only, ",")
		return orig(ctx, p, d, o)
	}
	m, cmd := send(m, "t")
	for i := 0; i < 3; i++ {
		m, cmd = step(t, m, cmd)
	}
	m, _ = send(m, "f")
	m, cmd = send(m, "r")
	if only != "sample-1" || !m.run.only {
		t.Fatalf("r should rerun just the selected test, only=%q", only)
	}
	if len(m.run.results) != 2 {
		t.Fatalf("other results should stay: %+v", m.run.results)
	}
	ch <- runner.Event{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-1", Verdict: runner.AC}}
	ch <- runner.Event{Kind: runner.Done, Verdict: runner.AC}
	m, cmd = step(t, m, cmd)
	m, _ = step(t, m, cmd)
	if len(m.run.results) != 2 || m.run.results[0].Verdict != runner.AC || m.run.overall != runner.AC {
		t.Fatalf("result replaced in place, overall recomputed: %+v %q", m.run.results, m.run.overall)
	}
}

func TestFullViewShowsLiveOutput(t *testing.T) {
	m, _ := openWithTests(t, []runner.Event{
		{Kind: runner.Output, Result: runner.Result{Name: "sample-1", Output: "PARTIAL-OUT\n", Stderr: "DBG-LIVE\n"}},
	})
	m, cmd := send(m, "t")
	m, _ = step(t, m, cmd)
	m.cases = []Case{{Name: "sample-1", Input: "1\n", Want: "2\n"}}
	m, _ = send(m, "f")
	out := plain(m)
	for _, want := range []string{"PARTIAL-OUT", "DBG-LIVE", "running..."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestFullViewKeysRebindable(t *testing.T) {
	m, _ := openWithTests(t, []runner.Event{{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-1", Verdict: runner.AC}}})
	km, err := newKeymap(map[string][]string{"full.wrap": {"W"}})
	if err != nil {
		t.Fatal(err)
	}
	m.km = km
	m, cmd := send(m, "t")
	m, _ = step(t, m, cmd)
	m, _ = send(m, "f")
	m, _ = send(m, "w")
	if m.fullWrap {
		t.Fatal("old key must not wrap once rebound")
	}
	m, _ = send(m, "W")
	if !m.fullWrap {
		t.Fatal("rebound key should toggle wrap")
	}
}

func TestCompileLoc(t *testing.T) {
	f, n := compileLoc("In file included:\nmain.cpp:2:1: warning: x\nmain.cpp:7:3: error: bad\n")
	if f != "main.cpp" || n != 7 {
		t.Fatalf("%s %d", f, n)
	}
	if _, n = compileLoc("no location here"); n != 0 {
		t.Fatal("no location")
	}
}

func TestCompileErrorJumpsToEditorLine(t *testing.T) {
	m, _ := openWithTests(t, []runner.Event{
		{Kind: runner.CompileFinished, Err: "main.cpp:3:5: error: bad"},
		{Kind: runner.Done, Verdict: runner.CE},
	})
	got := 0
	m.deps.SolutionPath = func(cf.Problem, string) string { return "/w/1/A/main.cpp" }
	m.deps.EditAt = func(p cf.Problem, lang string, line int) (*exec.Cmd, error) { got = line; return nil, nil }
	m, cmd := send(m, "t")
	for i := 0; i < 2; i++ {
		m, cmd = step(t, m, cmd)
	}
	m, _ = send(m, "d")
	if !strings.Contains(plain(m), "open at first error") {
		t.Fatalf("hint missing:\n%s", plain(m))
	}
	m, _ = send(m, "e")
	if got != 3 || m.run.diff {
		t.Fatalf("line=%d diff=%v", got, m.run.diff)
	}
}

func TestRuntimeErrorShowsInput(t *testing.T) {
	res := runner.Result{Name: "sample-1", Verdict: runner.RE, ExitCode: 1, Input: "INPUT-42\n"}
	m, _ := openWithTests(t, []runner.Event{{Kind: runner.TestFinished, Result: res}})
	m, cmd := send(m, "t")
	m, _ = step(t, m, cmd)
	m, _ = send(m, "d")
	if out := plain(m); !strings.Contains(out, "input:") || !strings.Contains(out, "INPUT-42") {
		t.Fatalf("input missing:\n%s", out)
	}
}

func TestPartialRuns(t *testing.T) {
	m, _ := openWithTests(t, []runner.Event{
		{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-1", Verdict: runner.WA}},
		{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-2", Verdict: runner.AC}},
		{Kind: runner.Done, Verdict: runner.WA},
	})
	var only string
	orig := m.deps.Tests
	m.deps.Tests = func(ctx context.Context, p cf.Problem, d *scrape.Detail, o RunOpts) (<-chan runner.Event, error) {
		only = strings.Join(o.Only, ",")
		return orig(ctx, p, d, o)
	}
	m, cmd := send(m, "t")
	for i := 0; i < 3; i++ {
		m, cmd = step(t, m, cmd)
	}
	m, _ = send(m, "F")
	if only != "sample-1" {
		t.Fatalf("F should rerun the failing test only, got %q", only)
	}
	m.pane = paneTests
	nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = nm.(Model)
	if !m.marks["sample-1"] || m.tsel != 1 {
		t.Fatalf("space should mark the selected test and move down: %v tsel=%d", m.marks, m.tsel)
	}
	m, _ = send(m, "R")
	if only != "sample-1" {
		t.Fatalf("R should run the marked tests, got %q", only)
	}
}

func TestFuzzySnippetsHelpAndSettings(t *testing.T) {
	p := &snippetPicker{items: []Snippet{{Name: "dijkstra", Desc: "shortest paths"}, {Name: "modpow", Desc: "fast power"}, {Name: "dsu", Desc: "union find"}}}
	p.query = "djk"
	if ms := p.matches(); len(ms) != 1 || ms[0].Name != "dijkstra" {
		t.Fatalf("fuzzy name: %+v", ms)
	}
	p.query = "unfi"
	if ms := p.matches(); len(ms) != 1 || ms[0].Name != "dsu" {
		t.Fatalf("fuzzy description: %+v", ms)
	}

	m := New(nil, "", Deps{})
	m.setQ = "thm"
	vis := m.settingIdxs()
	if len(vis) == 0 || settingDefs[vis[0]].key != "theme" {
		t.Fatalf("settings filter: %v", vis)
	}
	m.helpQ = "run tests"
	acts := m.matchActions(m.helpQ)
	if len(acts) == 0 || acts[0].id != "run_tests" {
		t.Fatalf("help search: %+v", acts)
	}
}

func TestKeyPressRoundTripsEveryDefaultKey(t *testing.T) {
	for _, a := range keyActions {
		for _, k := range a.def {
			if msg, ok := keyPress(k); !ok || msg.String() != k {
				t.Errorf("%s: %q -> %q ok=%v", actionID(a), k, msg.String(), ok)
			}
		}
	}
}

func TestPaletteRunsActionAndJumpsToSetting(t *testing.T) {
	m, _ := openWithTests(t, []runner.Event{{Kind: runner.Done, Verdict: runner.AC}})
	nm0, _ := m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	m = nm0.(Model)
	if m.pal == nil {
		t.Fatal("ctrl+k should open the palette")
	}
	for _, r := range "run tests" {
		nm, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = nm.(Model)
	}
	if ms := m.palMatches(); len(ms) == 0 || ms[0].key != "t" {
		t.Fatalf("matches: %+v", ms)
	}
	nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	if m.pal != nil || m.run == nil {
		t.Fatalf("enter should close the palette and press t: pal=%v run=%v", m.pal, m.run)
	}

	m2 := New(nil, "", Deps{})
	m2 = m2.openPalette()
	m2.pal.q = "theme"
	nm, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 = nm.(Model)
	if m2.tab != 4 || settingDefs[m2.setSel].key != "theme" {
		t.Fatalf("setting jump: tab=%d sel=%d", m2.tab, m2.setSel)
	}

	// a yes/no setting flips in place, from the palette
	m3 := New(nil, "", Deps{})
	m3 = m3.openPalette()
	m3.pal.q = "show footer"
	nm, _ = m3.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m3 = nm.(Model)
	if m3.pal != nil || m3.cfgVals["footer"] != "true" && m3.cfgVals["footer"] != "false" {
		t.Fatalf("toggle: pal=%v footer=%q", m3.pal, m3.cfgVals["footer"])
	}
	// zen hides the header and footer for the session
	nm, _ = m3.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	z := nm.(Model)
	if !z.zen || z.hdrOn() || z.ftrOn() {
		t.Fatalf("zen should hide header and footer: %+v", z.zen)
	}
	nm, _ = z.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	if nm.(Model).zen {
		t.Fatal("Z again should leave zen")
	}
}

func TestNoticeFades(t *testing.T) {
	m := New(nil, "", Deps{})
	m.Notice, m.noticeID = "config reloaded", 3
	nm, _ := m.Update(noticeClearMsg{2}) // a stale timer must not clear a newer notice
	if nm.(Model).Notice == "" {
		t.Fatal("stale timer cleared the notice")
	}
	nm, _ = m.Update(noticeClearMsg{3})
	if nm.(Model).Notice != "" {
		t.Fatal("notice should fade")
	}
}

// idle reports that cmd does no work: none, or only the notice fade timer (which blocks for seconds).
func idle(cmd tea.Cmd) bool {
	if cmd == nil {
		return true
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		_, fade := msg.(noticeClearMsg)
		return msg == nil || fade
	case <-time.After(150 * time.Millisecond):
		return true
	}
}
