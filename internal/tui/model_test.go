package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
)

func TestViewAndQuit(t *testing.T) {
	m := New([]cf.Problem{{ContestID: 1900, Index: "A", Name: "Cut the Triangle", Rating: 800, Tags: []string{"math"}, SolvedCount: 9}}, "", Deps{})
	out := m.View().Content
	for _, want := range []string{"Problems", "Contests", "1900A", "Cut the Triangle", "800", "math"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q cmd should produce QuitMsg")
	}
}

func TestControlCharsStripped(t *testing.T) {
	m := New([]cf.Problem{{ContestID: 1, Index: "A", Name: "evil\x1b]0;pwn\x07name", Tags: []string{"a\x1b[2Jb"}}}, "x\x1by", Deps{})
	if out := m.View().Content; strings.ContainsAny(out, "\x1b\x07") {
		t.Fatalf("control chars leaked: %q", out)
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func send(m Model, key string) (Model, tea.Cmd) {
	k := tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	switch key {
	case "enter":
		k = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		k = tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	nm, cmd := m.Update(k)
	return nm.(Model), cmd
}

func TestProblemViewFlow(t *testing.T) {
	p := cf.Problem{ContestID: 1, Index: "A", Name: "Theatre Square"}
	calls := 0
	deps := Deps{Load: func(p cf.Problem, force bool) (*scrape.Detail, error) {
		calls++
		return &scrape.Detail{
			Statement:   `<div class="problem-statement"><div><p>Hello $$$n \le 5$$$</p></div></div>`,
			TimeLimitMS: 1000, MemoryLimitMB: 256, Interactive: true,
			Samples: []scrape.Sample{{Input: "1 2\n", Output: "3\n"}},
		}, nil
	}}
	m, cmd := send(New([]cf.Problem{p}, "", deps), "enter")
	if m.open == nil || cmd == nil {
		t.Fatal("enter should open and load")
	}
	nm, _ := m.Update(cmd())
	m = nm.(Model)
	out := ansi.ReplaceAllString(m.View().Content, "")
	for _, want := range []string{"Theatre Square", "time 1 s", "256 MB", "interactive", "n ≤ 5", "Sample 1 input:", "1 2", "esc back"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	m, _ = send(m, "esc")
	if m.open != nil || !strings.Contains(m.View().Content, "enter open") {
		t.Fatal("esc should return to list")
	}
	if calls != 1 {
		t.Fatalf("calls %d", calls)
	}
}

func TestChallengeMessage(t *testing.T) {
	deps := Deps{Load: func(cf.Problem, bool) (*scrape.Detail, error) { return nil, cf.ErrChallenge }}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A"}}, "", deps), "enter")
	nm, _ := m.Update(cmd())
	if out := nm.(Model).View().Content; !strings.Contains(out, "Cloudflare") || !strings.Contains(out, "o to open") {
		t.Fatalf("no challenge message:\n%s", out)
	}
}

func TestOpenBrowser(t *testing.T) {
	var got string
	deps := Deps{Load: func(cf.Problem, bool) (*scrape.Detail, error) { return nil, cf.ErrChallenge }, OpenURL: func(u string) error { got = u; return nil }}
	m, _ := send(New([]cf.Problem{{ContestID: 7, Index: "C"}}, "", deps), "enter")
	_, cmd := send(m, "o")
	cmd()
	if got != "https://codeforces.com/problemset/problem/7/C" {
		t.Fatalf("got %q", got)
	}
}

func typeText(m Model, text string) Model {
	for _, r := range text {
		nm, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = nm.(Model)
	}
	return m
}

func TestFilterSearchAndMarks(t *testing.T) {
	ps := []cf.Problem{
		{ContestID: 3, Index: "A", Name: "Alpha", Rating: 800, Tags: []string{"dp"}},
		{ContestID: 2, Index: "B", Name: "Beta", Rating: 1500, Tags: []string{"dp", "math"}},
		{ContestID: 1, Index: "C", Name: "Gamma", Rating: 800, Tags: []string{"math"}},
	}
	m := New(ps, "", Deps{}).WithStatuses(map[string]store.Status{"3A": store.StatusSolved, "2B": store.StatusAttempted})
	out := m.View().Content
	if !strings.Contains(out, "✓ 3A") || !strings.Contains(out, "✗ 2B") || !strings.Contains(out, "3 problems") {
		t.Fatalf("marks/count wrong:\n%s", out)
	}
	// filter: +dp unsolved  -> only 2B
	m, _ = send(m, "f")
	m = typeText(m, "+dp unsolved")
	m, _ = send(m, "enter")
	if len(m.visible) != 1 || m.visible[0].Index != "B" || !strings.Contains(m.View().Content, "1/3 problems") {
		t.Fatalf("filter: %+v\n%s", m.visible, m.View().Content)
	}
	// search combines with the filter
	m, _ = send(m, "/")
	m = typeText(m, "gamma")
	m, _ = send(m, "enter")
	if len(m.visible) != 0 {
		t.Fatalf("search should AND with filter: %+v", m.visible)
	}
	// clear search, bad filter keeps prompt open with an error
	m, _ = send(m, "/")
	nm, _ := m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = nm.(Model)
	m, _ = send(m, "enter")
	m, _ = send(m, "f")
	nm, _ = m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = typeText(nm.(Model), "wat")
	m, _ = send(m, "enter")
	if m.input == nil || !strings.Contains(m.View().Content, "unknown filter") {
		t.Fatal("bad filter should keep prompt with error")
	}
	// enter opens the filtered row (2B), not row 0 of the full list (3A)
	m, _ = send(m, "esc")
	m, cmd := send(m, "enter")
	if m.open == nil || m.open.Index != "B" || cmd == nil {
		t.Fatalf("want 2B opened, got %+v", m.open)
	}
}
