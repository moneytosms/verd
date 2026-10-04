package tui

import (
	"context"
	"errors"
	"fmt"
	"github.com/moneytosms/verd/internal/runner"
	"image/color"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
)

func TestViewAndQuit(t *testing.T) {
	m := New([]cf.Problem{{ContestID: 1900, Index: "A", Name: "Cut the Triangle", Rating: 800, Tags: []string{"math"}, SolvedCount: 9}}, "", Deps{})
	out := plain(m)
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
	if out := plain(m); strings.ContainsAny(out, "\x1b\x07") {
		t.Fatalf("control chars leaked: %q", out)
	}
}

// plain renders the model without SGR styling so tests can match on text.
func plain(m Model) string { return ansi.ReplaceAllString(m.View().Content, "") }

// initMsgs runs Init's commands and returns their messages (Init batches several).
func initMsgs(m Model) []tea.Msg {
	var out []tea.Msg
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				run(c)
			}
			return
		}
		out = append(out, msg)
	}
	run(m.Init())
	return out
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
	out := plain(m)
	for _, want := range []string{"Theatre Square", "time 1 s", "256 MB", "interactive", "n ≤ 5", "Sample 1 input:", "1 2", "esc back"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	m, _ = send(m, "esc")
	if m.open != nil || !strings.Contains(plain(m), "enter open") {
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
	if out := plain(nm.(Model)); !strings.Contains(out, "Cloudflare") || !strings.Contains(out, "o to open") {
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
	out := plain(m)
	if !strings.Contains(out, "✓ 3A") || !strings.Contains(out, "✗ 2B") || !strings.Contains(out, "3 problems") {
		t.Fatalf("marks/count wrong:\n%s", out)
	}
	// filter: +dp unsolved  -> only 2B
	m, _ = send(m, "f")
	m = typeText(m, "+dp unsolved")
	m, _ = send(m, "enter")
	if len(m.visible) != 1 || m.visible[0].Index != "B" || !strings.Contains(plain(m), "1/3 problems") {
		t.Fatalf("filter: %+v\n%s", m.visible, plain(m))
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
	if m.input == nil || !strings.Contains(plain(m), "unknown filter") {
		t.Fatal("bad filter should keep prompt with error")
	}
	// enter opens the filtered row (2B), not row 0 of the full list (3A)
	m, _ = send(m, "esc")
	m, cmd := send(m, "enter")
	if m.open == nil || m.open.Index != "B" || cmd == nil {
		t.Fatalf("want 2B opened, got %+v", m.open)
	}
}

func TestContestsTab(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cs := []cf.Contest{
		{ID: 10, Name: "Old Round", Phase: "FINISHED", Start: 500_000},
		{ID: 30, Name: "Later Round", Phase: "BEFORE", Start: now.Add(50 * time.Hour).Unix()},
		{ID: 20, Name: "Soon Round", Phase: "BEFORE", Start: now.Add(3*time.Hour + 20*time.Minute).Unix()},
		{ID: 5, Name: "Newer Past", Phase: "FINISHED", Start: 900_000},
	}
	ps := []cf.Problem{
		{ContestID: 10, Index: "B", Name: "Second"},
		{ContestID: 10, Index: "A", Name: "First"},
		{ContestID: 5, Index: "A", Name: "Other"},
	}
	deps := Deps{Now: func() time.Time { return now }, Load: func(cf.Problem, bool) (*scrape.Detail, error) { return nil, cf.ErrChallenge }}
	m := New(ps, "", deps).WithStatuses(map[string]store.Status{"10A": store.StatusSolved, "10B": store.StatusAttempted}).WithContests(cs)
	m, _ = send(m, "2")
	out := plain(m)
	iSoon, iLater, iNew, iOld := strings.Index(out, "Soon Round"), strings.Index(out, "Later Round"), strings.Index(out, "Newer Past"), strings.Index(out, "Old Round")
	if !(iSoon >= 0 && iSoon < iLater && iLater < iNew && iNew < iOld) {
		t.Fatalf("order wrong (upcoming asc, then past desc):\n%s", out)
	}
	for _, want := range []string{"Upcoming", "Past", "in 3h 20m", "in 2d 2h", "1970-01-11"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	// open Old Round (4th row), see Problems with status, in index order
	for range 3 {
		m, _ = send(m, "j")
	}
	m, _ = send(m, "enter")
	out = plain(m)
	if !(strings.Index(out, "✓ A") > 0 && strings.Index(out, "✓ A") < strings.Index(out, "✗ B")) || strings.Contains(out, "Other") {
		t.Fatalf("contest problems wrong:\n%s", out)
	}
	// open a Problem, esc returns to the contest's problems, esc again to the list
	m, cmd := send(m, "enter")
	if m.open == nil || m.open.Index != "A" || cmd == nil {
		t.Fatalf("want 10A open, got %+v", m.open)
	}
	m, _ = send(m, "esc")
	if m.contestOpen == nil || !strings.Contains(plain(m), "First") {
		t.Fatal("esc from Problem should return to contest Problems")
	}
	m, _ = send(m, "esc")
	if m.contestOpen != nil || !strings.Contains(plain(m), "Upcoming") {
		t.Fatal("esc should return to contest list")
	}
}

func TestBackgroundRefreshDoesNotBlockInput(t *testing.T) {
	release := make(chan struct{})
	deps := Deps{Refresh: []func() (Data, error){func() (Data, error) {
		<-release
		return Data{Problems: []cf.Problem{{ContestID: 9, Index: "Z", Name: "Fresh"}}}, nil
	}}}
	cached := []cf.Problem{{ContestID: 1, Index: "A", Name: "Cached1"}, {ContestID: 2, Index: "A", Name: "Cached2"}}
	m := New(cached, "", deps)
	cmd := m.Init() // must return immediately; the blocking work lives in the batched cmds
	if cmd == nil || !strings.Contains(plain(m), "[syncing...]") || !strings.Contains(plain(m), "Cached1") {
		t.Fatal("should render cache and show syncing before refresh completes")
	}
	batch := cmd().(tea.BatchMsg)
	done := make(chan tea.Msg, len(batch))
	for _, c := range batch {
		go func() { done <- c() }()
	}
	m, _ = send(m, "j") // input handled while refresh is still blocked
	if m.cursor != 1 {
		t.Fatal("input blocked during refresh")
	}
	close(release)
	for range batch {
		nm, _ := m.Update(<-done)
		m = nm.(Model)
	}
	out := plain(m)
	if !strings.Contains(out, "Fresh") || strings.Contains(out, "syncing") {
		t.Fatalf("fresh data not applied:\n%s", out)
	}
}

func TestOfflineBadgeAndMessages(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cached := []cf.Problem{{ContestID: 1, Index: "A", Name: "Cached1"}}
	loads := 0
	deps := Deps{
		Now:     func() time.Time { return now },
		Refresh: []func() (Data, error){func() (Data, error) { return Data{}, nil }},
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			loads++
			return nil, fmt.Errorf("x: %w", cf.ErrNetwork)
		},
	}
	m := New(cached, "", deps)
	nm, _ := m.Update(refreshedMsg{Data{Problems: cached, SyncedAt: now.Add(-3 * time.Hour)}, fmt.Errorf("sync: %w", cf.ErrNetwork), 0})
	m = nm.(Model)
	out := plain(m)
	if !strings.Contains(out, "Cached1") || !strings.Contains(out, "[offline, synced 3h ago]") {
		t.Fatalf("offline render:\n%s", out)
	}
	nm, _ = m.Update(refreshedMsg{Data{Problems: cached}, fmt.Errorf("sync: %w", cf.ErrNetwork), 0})
	if !strings.Contains(plain(nm.(Model)), "[offline, never synced]") {
		t.Fatal("never-synced badge missing")
	}
	// uncached statement while offline
	m, cmd := send(m, "enter")
	nm, _ = m.Update(cmd())
	m = nm.(Model)
	if out := plain(m); !strings.Contains(out, "statement not cached") {
		t.Fatalf("want not-cached message:\n%s", out)
	}
	// refetch is network-only: explicit message, no attempt
	before := loads
	m, cmd = send(m, "r")
	if cmd != nil || loads != before || !strings.Contains(plain(m), "offline: refetch needs network") {
		t.Fatalf("refetch offline should explain, not try (loads %d->%d)", before, loads)
	}
}

func TestNonNetworkSyncErrorIsNotOffline(t *testing.T) {
	m := New(nil, "", Deps{Refresh: []func() (Data, error){func() (Data, error) { return Data{}, nil }}})
	nm, _ := m.Update(refreshedMsg{Data{}, &cf.APIError{Comment: "handle not found"}, 0})
	out := plain(nm.(Model))
	if strings.Contains(out, "offline") || !strings.Contains(out, "handle not found") {
		t.Fatalf("api error should be a notice, not offline:\n%s", out)
	}
}

func TestRefreshStagesRunInOrderAndStopOffline(t *testing.T) {
	var ran []string
	stage := func(name string, err error) func() (Data, error) {
		return func() (Data, error) { ran = append(ran, name); return Data{}, err }
	}
	run := func(m Model) Model {
		msgs := initMsgs(m)
		for len(msgs) > 0 {
			nm, cmd := m.Update(msgs[0])
			m, msgs = nm.(Model), msgs[1:]
			if cmd != nil {
				msgs = append(msgs, cmd())
			}
		}
		return m
	}
	m := run(New(nil, "", Deps{Refresh: []func() (Data, error){stage("core", nil), stage("subs", nil)}}))
	if strings.Join(ran, ",") != "core,subs" || m.syncing {
		t.Fatalf("ran %v syncing=%v", ran, m.syncing)
	}
	ran = nil
	m = run(New(nil, "", Deps{Refresh: []func() (Data, error){stage("core", fmt.Errorf("x: %w", cf.ErrNetwork)), stage("subs", nil)}}))
	if strings.Join(ran, ",") != "core" || !m.offline || m.syncing {
		t.Fatalf("offline should skip later stages: ran %v", ran)
	}
}

func TestHelpOverlayListsCurrentScreenKeys(t *testing.T) {
	m := New([]cf.Problem{{ContestID: 1, Index: "A", Name: "X"}}, "", Deps{})
	m, _ = send(m, "?")
	out := plain(m)
	for _, want := range []string{"Keys: Problems", "filter", "search by ID or name", "open Problem"} {
		if !strings.Contains(out, want) {
			t.Errorf("list help missing %q:\n%s", want, out)
		}
	}
	m, _ = send(m, "esc")
	if strings.Contains(plain(m), "Keys:") {
		t.Fatal("esc should close help")
	}
	m, _ = send(m, "2")
	m, _ = send(m, "?")
	if out := plain(m); !strings.Contains(out, "Keys: Contests") || strings.Contains(out, "search by ID") {
		t.Errorf("contests help wrong:\n%s", out)
	}
	m, _ = send(m, "?")
	// inside a Problem
	m, _ = send(m, "1")
	m, _ = send(m, "enter")
	m, _ = send(m, "?")
	if out := plain(m); !strings.Contains(out, "Keys: Problem") || !strings.Contains(out, "refetch statement") {
		t.Errorf("problem help wrong:\n%s", out)
	}
	// typing '?' in the search prompt is text, not help
	m, _ = send(m, "?")
	m, _ = send(m, "esc")
	m, _ = send(m, "/")
	m = typeText(m, "?")
	if m.input == nil || m.input.text != "?" || m.help {
		t.Fatal("? inside a prompt must be typed text")
	}
}

func TestStatusLineAndTheme(t *testing.T) {
	m := New(nil, "", Deps{}).WithData(Data{Handle: "tourist", Rating: 3800})
	if out := plain(m); !strings.Contains(out, "tourist (3800)") {
		t.Fatalf("status line missing handle/rating:\n%s", out)
	}
	if out := plain(New(nil, "", Deps{}).WithData(Data{Handle: "newbie"})); !strings.Contains(out, "newbie") || strings.Contains(out, "(0)") {
		t.Fatalf("unrated user should show handle only:\n%s", out)
	}
	// theme switch changes rendered colors; unknown theme falls back with a notice
	term := New(nil, "", Deps{}).WithData(Data{Handle: "h"})
	drac := term.WithTheme("dracula")
	if term.View().Content == drac.View().Content {
		t.Fatal("theme must change rendered output")
	}
	if !strings.Contains(plain(term.WithTheme("nope")), "unknown theme") {
		t.Fatal("unknown theme should notice")
	}
	// light background picks light variants
	nm, _ := drac.Update(tea.BackgroundColorMsg{Color: color.White})
	if nm.(Model).dark || nm.(Model).View().Content == drac.View().Content {
		t.Fatal("light background must change styling")
	}
}

func TestEditKey(t *testing.T) {
	var got cf.Problem
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"></div>`}, nil
		},
		Edit: func(p cf.Problem) (*exec.Cmd, error) { got = p; return exec.Command("true"), nil },
	}
	m, _ := send(New([]cf.Problem{{ContestID: 7, Index: "C", Name: "N"}}, "", deps), "enter")
	m, cmd := send(m, "e")
	if got.ContestID != 7 || got.Index != "C" || cmd == nil {
		t.Fatalf("e should prepare the editor for 7C and return an exec cmd: %+v", got)
	}
	// editor failure surfaces in the Problem view
	nm, _ := m.Update(editorDoneMsg{errors.New("exit status 1")})
	if !strings.Contains(plain(nm.(Model)), "editor: exit status 1") {
		t.Fatal("editor error not shown")
	}
	// Edit failing (e.g. nvim missing) shows a message and runs nothing
	deps.Edit = func(cf.Problem) (*exec.Cmd, error) { return nil, errors.New("nvim not found in PATH") }
	m, _ = send(New([]cf.Problem{{ContestID: 7, Index: "C"}}, "", deps), "enter")
	m, cmd = send(m, "e")
	if cmd != nil || !strings.Contains(plain(m), "nvim not found in PATH") {
		t.Fatal("edit error should be shown, no exec")
	}
}

// drain feeds a command's message (and its follow-up commands) back into the model.
func step(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	nm, next := m.Update(cmd())
	return nm.(Model), next
}

func openWithTests(t *testing.T, events []runner.Event) (Model, chan runner.Event) {
	t.Helper()
	ch := make(chan runner.Event, len(events)+1)
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p>hi</p></div>`, TimeLimitMS: 1000, MemoryLimitMB: 256}, nil
		},
		Tests: func(ctx context.Context, p cf.Problem, d *scrape.Detail) (<-chan runner.Event, error) { return ch, nil },
	}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A", Name: "N"}}, "", deps), "enter")
	m, _ = step(t, m, cmd)
	for _, ev := range events {
		ch <- ev
	}
	return m, ch
}

func TestTestsPanelStreams(t *testing.T) {
	wa := runner.Result{Name: "sample-2", Verdict: runner.WA, TimeMS: 5, MemoryMB: 3, Expected: "1\n10\n", Output: "1\n11\n", Mismatch: &runner.Mismatch{Line: 2, Col: 1, Want: "10", Got: "11"}}
	m, ch := openWithTests(t, []runner.Event{
		{Kind: runner.CompileStarted},
		{Kind: runner.CompileFinished, Cached: true},
		{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-1", Verdict: runner.AC, TimeMS: 8, MemoryMB: 31.9}},
		{Kind: runner.TestFinished, Result: wa},
		{Kind: runner.Done, Verdict: runner.WA},
	})
	if !strings.Contains(plain(m), "press t to run") {
		t.Fatal("idle panel should hint t")
	}
	m, cmd := send(m, "t")
	if !strings.Contains(plain(m), "running...") {
		t.Fatal("run should show running")
	}
	// panel updates per streamed event
	m, cmd = step(t, m, cmd) // CompileStarted
	if !strings.Contains(plain(m), "compile: compiling...") {
		t.Fatalf("compile start not shown:\n%s", plain(m))
	}
	m, cmd = step(t, m, cmd) // CompileFinished
	m, cmd = step(t, m, cmd) // sample-1
	out := plain(m)
	if !strings.Contains(out, "compile: ok (cached)") || !strings.Contains(out, "sample-1") || !strings.Contains(out, "31.9 MB") || strings.Contains(out, "sample-2") {
		t.Fatalf("after first test only sample-1 should show:\n%s", out)
	}
	m, cmd = step(t, m, cmd) // sample-2 WA
	if out := plain(m); !strings.Contains(out, "sample-2") || !strings.Contains(out, `line 2 col 1: want "10" got "11"`) {
		t.Fatalf("WA row missing:\n%s", out)
	}
	m, cmd = step(t, m, cmd) // Done
	close(ch)
	m, cmd = step(t, m, cmd) // closed
	if cmd != nil {
		t.Fatal("stream end should stop listening")
	}
	if out := plain(m); !strings.Contains(out, "1/2 AC") || strings.Contains(out, "running...") {
		t.Fatalf("summary missing:\n%s", out)
	}

	// diff overlay on the failing test: select it, d
	m, _ = send(m, "n")
	m, _ = send(m, "d")
	out = plain(m)
	for _, want := range []string{"Diff: sample-2", "first mismatch at line 2 col 1", "expected", "actual", "10", "11", "showing 2/2 lines"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
	// mismatching line is highlighted (styled), matching line is not
	raw := m.View().Content
	if !strings.Contains(raw, "\x1b[") || strings.Count(raw, "10") < 1 {
		t.Fatal("no styling in diff")
	}
	m, _ = send(m, "esc")
	if m.run == nil || m.run.diff || m.open == nil {
		t.Fatal("esc closes the diff, not the Problem")
	}
	// d on a passing test does nothing
	m, _ = send(m, "p")
	if m, _ = send(m, "d"); m.run.diff {
		t.Fatal("no diff for AC")
	}
	// leaving the Problem drops the run
	m, _ = send(m, "esc")
	if m.run != nil {
		t.Fatal("run should be dropped on esc")
	}
}

func TestDiffTruncatesLargeOutput(t *testing.T) {
	var want, got []string
	for i := 0; i < 500; i++ {
		want = append(want, "line")
		got = append(got, "line")
	}
	got[400] = "BAD"
	res := runner.Result{Name: "sample-1", Verdict: runner.WA, Expected: strings.Join(want, "\n"), Output: strings.Join(got, "\n"), Mismatch: &runner.Mismatch{Line: 401, Col: 1, Want: "line", Got: "BAD"}}
	m, ch := openWithTests(t, []runner.Event{{Kind: runner.TestFinished, Result: res}})
	m, cmd := send(m, "t")
	m, _ = step(t, m, cmd)
	_ = ch
	m, _ = send(m, "d")
	out := plain(m)
	if !strings.Contains(out, "of 500 lines") && !strings.Contains(out, "/500 lines") {
		t.Fatalf("truncation hint missing:\n%s", out)
	}
	if !strings.Contains(out, "BAD") {
		t.Fatalf("diff should start near the first mismatch:\n%s", out)
	}
	if strings.Count(out, "\n") > 40 {
		t.Fatalf("diff not truncated: %d lines", strings.Count(out, "\n"))
	}
}

func TestStaleRunEventsIgnored(t *testing.T) {
	m, _ := openWithTests(t, nil)
	m, _ = send(m, "t")
	old := m.run.id
	m, _ = send(m, "t") // restart: new run id
	nm, cmd := m.Update(testEventMsg{id: old, ev: runner.Event{Kind: runner.TestFinished, Result: runner.Result{Name: "old"}}})
	if cmd != nil || strings.Contains(plain(nm.(Model)), "old") {
		t.Fatal("events from a replaced run must be dropped")
	}
}

func TestInteractiveRefusesTests(t *testing.T) {
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"></div>`, Interactive: true}, nil
		},
		Tests: func(context.Context, cf.Problem, *scrape.Detail) (<-chan runner.Event, error) {
			t.Fatal("must not run")
			return nil, nil
		},
	}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A"}}, "", deps), "enter")
	m, _ = step(t, m, cmd)
	m, _ = send(m, "t")
	if !strings.Contains(plain(m), "local run unsupported") {
		t.Fatal("interactive should be refused with a message")
	}
}
