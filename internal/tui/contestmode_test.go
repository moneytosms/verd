package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/session"
)

func alt(m Model, r rune) Model {
	nm, _ := m.Update(tea.KeyPressMsg{Code: r, Mod: tea.ModAlt})
	return nm.(Model)
}

// contestModel is a running practice session over a three-problem contest, 160x40.
func contestModel(t *testing.T, now *time.Time) (Model, *session.Stats, *string) {
	ps := []cf.Problem{{ContestID: 7, Index: "A", Name: "Alpha"}, {ContestID: 7, Index: "B", Name: "Beta"}, {ContestID: 7, Index: "C", Name: "Gamma"}, {ContestID: 9, Index: "A", Name: "Other"}}
	cs := []cf.Contest{{ID: 7, Name: "Round 7", Phase: "FINISHED", Start: 1, Duration: 7200}}
	saved, report := &session.Stats{}, new(string)
	deps := Deps{
		Now: func() time.Time { return *now },
		StartSession: func(c cf.Contest, mode string) (session.Session, error) {
			return session.Session{ContestID: c.ID, Name: c.Name, Mode: mode, Start: now.Unix()}, nil
		},
		EndSession:       func() error { return nil },
		SaveSessionStats: func(st session.Stats) error { *saved = st; return nil },
		SaveReport:       func(_ session.Session, md string) (string, error) { *report = md; return "/w/7/practice.md", nil },
	}
	m := New(ps, "", deps).WithData(Data{Problems: ps, Contests: cs})
	m.width, m.height = 160, 40
	m, _ = send(m, "2")
	m, _ = send(m, "enter")
	m, _ = send(m, "p")
	m, _ = send(m, "p")
	if m.sess == nil {
		t.Fatal("practice did not start")
	}
	return m, saved, report
}

// checkContestStart checks the contest screen opens on problem A with tabs hidden.
func checkContestStart(t *testing.T, m Model) {
	t.Helper()
	if !m.viewing() || m.open.Index != "A" || m.tab != 0 {
		t.Fatalf("starting opens problem A: %+v tab %d", m.open, m.tab)
	}
	mustHave(t, plain(m), "contest", "Round 7", "▸A", " B", " C")
	mustLack(t, plain(m), "Problems", "Settings")
}

// expectOpen fails unless problem index is open.
func expectOpen(t *testing.T, m Model, index string) {
	t.Helper()
	if m.open.Index != index {
		t.Fatalf("open %s, want %s", m.open.Index, index)
	}
}

// cycleProblems walks alt+n, alt+3 and alt+p around the contest, keeping A's scroll.
func cycleProblems(t *testing.T, m Model) Model {
	t.Helper()
	m.scroll = 3
	m = alt(m, 'n')
	expectOpen(t, m, "B")
	m = alt(m, '3')
	expectOpen(t, m, "C")
	m = alt(m, 'n') // wraps around
	expectOpen(t, m, "A")
	if m.cmRestoreScroll().scroll != 3 {
		t.Fatalf("back to A with its scroll: %d", m.cmRestoreScroll().scroll)
	}
	m = alt(m, 'p')
	expectOpen(t, m, "C")
	if m, _ = send(m, "esc"); !m.viewing() {
		t.Fatal("esc must not leave contest mode")
	}
	return m
}

// idleOnC runs five seconds of session ticks.
func idleOnC(m Model, now *time.Time) Model {
	for i := 0; i < 5; i++ {
		*now = now.Add(time.Second)
		nm, _ := m.Update(sessTickMsg{})
		m = nm.(Model)
	}
	return m
}

func TestContestModeNavigatesAndReports(t *testing.T) {
	now := time.Unix(10_000, 0)
	m, saved, report := contestModel(t, &now)
	checkContestStart(t, m)
	m = cycleProblems(t, m)

	m = idleOnC(m, &now) // five seconds on C
	m = m.cmCountRun("AC")
	if c := (*saved)["C"]; c == nil || c.Seconds != 5 || c.Runs != 1 || c.Green != 1 || c.FirstOpen == 0 {
		t.Fatalf("stats for C: %+v", c)
	}

	m = alt(m, 'x')
	if !m.sessSum || !strings.Contains(plain(m), "test runs") {
		t.Fatalf("alt+x opens the summary:\n%s", plain(m))
	}
	m, _ = send(m, "e")
	if m.sess != nil || m.cmOn() {
		t.Fatal("e ends the session and contest mode")
	}
	mustHave(t, *report, "| C Gamma | - | 0 | 0:05 |")
	mustHave(t, m.Notice, "/w/7/practice.md")
	mustHave(t, plain(m), "Settings")
}

func TestContestRail(t *testing.T) {
	now := time.Unix(10_000, 0)
	m, _, _ := contestModel(t, &now)
	if m.railCols(160) != railFullW || m.railCols(120) != railCompactW || m.railCols(50) != 0 {
		t.Fatalf("auto rail: %d %d %d", m.railCols(160), m.railCols(120), m.railCols(50))
	}
	m = alt(m, 'b')
	if m.railCols(160) != railCompactW {
		t.Fatal("alt+b: full to compact")
	}
	m = alt(m, 'b')
	if m.railCols(160) != 0 || strings.Contains(plain(m), "▸A") {
		t.Fatal("alt+b: compact to hidden")
	}
	m = alt(m, 'b')
	if m.railCols(160) != railFullW {
		t.Fatal("alt+b: hidden to full")
	}
	if m.zen = true; m.railCols(160) != 0 {
		t.Fatal("zen hides the rail")
	}
	m.zen = false

	// a click on B's row jumps there (full rail: name, mode, score, rule, then the problems)
	nm, _ := m.Update(tea.MouseClickMsg{X: 2, Y: 5, Button: tea.MouseLeft})
	if m = nm.(Model); m.open.Index != "B" {
		t.Fatalf("rail click: %s", m.open.Index)
	}
}

func TestStandingsPanel(t *testing.T) {
	now := time.Unix(10_000, 0)
	m, _, _ := contestModel(t, &now)
	m.sess.Mode = session.Live
	row := func(rank int, h string) cf.StandingRow { return cf.StandingRow{Rank: rank, Handle: h} }
	me := row(41, "me")
	m.standing = &cf.Standing{Row: &me, Total: 900, Top: []cf.StandingRow{row(1, "tourist"), row(2, "jiangly"), row(3, "ksun")},
		Near: []cf.StandingRow{row(39, "x"), row(40, "y"), me, row(42, "z")}}
	out := strings.Join(m.railLines(railFullW, 40), "\n")
	for _, want := range []string{"rank 41/900", "1 tourist", "3 ksun", "⋯", "39 x", "41 me", "42 z"} {
		if !strings.Contains(out, want) {
			t.Fatalf("standings panel missing %q:\n%s", want, out)
		}
	}
}

func TestSummaryEscKeepsOneTickChain(t *testing.T) {
	now := time.Unix(10_000, 0)
	m, _, _ := contestModel(t, &now)
	m = alt(m, 'x')
	if !m.sessSum {
		t.Fatal("alt+x opens the summary")
	}
	// the tick chain never stopped under the summary: esc must not start a second one
	if m, cmd := send(m, "esc"); m.sessSum || cmd != nil {
		t.Fatalf("esc: summary %v, cmd %v", m.sessSum, cmd)
	}
}

func TestOutsideProblemStaysOnceSessionShown(t *testing.T) {
	now := time.Unix(10_000, 0)
	m, _, _ := contestModel(t, &now) // session shows A
	other := cf.Problem{ContestID: 9, Index: "A", Name: "Other"}
	m.open = &other // e.g. a Test Run from the CLI on another contest's Problem
	if m, _ = m.cmEnsure(); m.open.ContestID != 9 {
		t.Fatalf("a Problem opened from outside was replaced by %s", m.open.Name)
	}
}
