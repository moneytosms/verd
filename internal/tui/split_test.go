package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

func splitModel(t *testing.T, deps Deps) Model {
	t.Helper()
	deps.Load = func(cf.Problem, bool) (*scrape.Detail, error) {
		return &scrape.Detail{
			Statement:   `<div class="problem-statement"><p>Divide the watermelon.</p></div>`,
			TimeLimitMS: 1000, MemoryLimitMB: 64,
			Samples: []scrape.Sample{{Input: "8\n", Output: "YES\n"}},
		}, nil
	}
	p := cf.Problem{ContestID: 4, Index: "A", Name: "Watermelon", Rating: 800, Tags: []string{"brute force", "math"}}
	m := New([]cf.Problem{p}, "", deps)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m, cmd := send(nm.(Model), "enter")
	m, _ = step(t, m, cmd)
	return m
}

func TestSplitViewShowsStatementTagsAndEveryTest(t *testing.T) {
	m := splitModel(t, Deps{Customs: func(cf.Problem) ([]Case, error) {
		return []Case{{Name: "custom-1", Input: "6\n", Want: "YES\n"}}, nil
	}})
	out := plain(m)
	for _, want := range []string{"4A  Watermelon", "Divide the watermelon", "rating 800", "brute force", "math", "limits 1 s", "sample-1", "custom-1", "Input", "Expected", "8"} {
		if !strings.Contains(out, want) {
			t.Errorf("split view missing %q:\n%s", want, out)
		}
	}
	for i, l := range strings.Split(out, "\n") {
		if w := len([]rune(l)); w > 140 {
			t.Fatalf("line %d is %d columns wide", i, w)
		}
	}
}

func TestPaneFocusMovesTestSelection(t *testing.T) {
	m := splitModel(t, Deps{Customs: func(cf.Problem) ([]Case, error) {
		return []Case{{Name: "custom-1", Input: "6\n", Want: "NO\n"}}, nil
	}})
	m, _ = send(m, "j") // statement pane: scrolls, selection stays
	if m.tsel != 0 {
		t.Fatal("j in the statement pane must not move the test selection")
	}
	m, _ = send(m, "tab")
	m, _ = send(m, "j")
	if m.tsel != 1 || !strings.Contains(plain(m), "custom-1") || !strings.Contains(plain(m), "NO") {
		t.Fatalf("j in the tests pane selects the next test: tsel=%d\n%s", m.tsel, plain(m))
	}
}

func TestTestManagerDeletesCustomButNotSamples(t *testing.T) {
	customs := []Case{{Name: "custom-1", Input: "6\n", Want: "NO\n"}}
	var deleted []string
	m := splitModel(t, Deps{
		Customs: func(cf.Problem) ([]Case, error) { return customs, nil },
		DeleteCase: func(_ cf.Problem, name string) error {
			deleted = append(deleted, name)
			customs = nil
			return nil
		},
	})
	m, _ = send(m, "T")
	m, _ = send(m, "d") // sample-1 is selected first
	if m.tm.confirmDel || len(deleted) != 0 || !strings.Contains(plain(m), "samples cannot be deleted") {
		t.Fatal("samples are not deletable")
	}
	m, _ = send(m, "j")
	m, _ = send(m, "d")
	if !strings.Contains(plain(m), "delete custom-1? y/n") {
		t.Fatalf("delete must ask first:\n%s", plain(m))
	}
	m, _ = send(m, "y")
	if len(deleted) != 1 || deleted[0] != "custom-1" || len(m.cases) != 1 {
		t.Fatalf("deleted=%v cases=%v", deleted, m.cases)
	}
	m, _ = send(m, "q")
	if m.tm != nil || m.open == nil {
		t.Fatal("q closes the manager and stays on the Problem")
	}
}

func TestEditorCursorEditing(t *testing.T) {
	e := newEditor("custom-1", "ab\ncd\n", "x\n")
	e.row, e.col = 0, 2
	e.key(tea.KeyPressMsg{Code: tea.KeyBackspace})
	e.key(tea.KeyPressMsg{Code: tea.KeyEnter})
	e.key(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if got := e.text(0); got != "a\nz\ncd\n" {
		t.Fatalf("input text %q", got)
	}
	e.row, e.col = 1, 0
	e.key(tea.KeyPressMsg{Code: tea.KeyBackspace}) // joins "z" onto "a"
	if got := e.text(0); got != "az\ncd\n" {
		t.Fatalf("join: %q", got)
	}
	e.insert("p\nq") // paste with a newline
	if got := e.text(0); got != "ap\nqz\ncd\n" {
		t.Fatalf("paste: %q", got)
	}
}

func TestContestProblemsOpenInModalClosedByQ(t *testing.T) {
	ps := []cf.Problem{{ContestID: 7, Index: "A", Name: "First"}}
	m := New(ps, "", Deps{}).WithContests([]cf.Contest{{ID: 7, Name: "Round 7", Phase: "FINISHED", Start: 1}}).WithData(Data{Problems: ps, Contests: []cf.Contest{{ID: 7, Name: "Round 7", Phase: "FINISHED", Start: 1}}})
	m, _ = send(m, "2")
	m, _ = send(m, "enter")
	out := plain(m)
	if !strings.Contains(out, "First") || !strings.Contains(out, "q close") || !strings.Contains(out, "Round 7") {
		t.Fatalf("contest modal:\n%s", out)
	}
	if m, cmd := send(m, "q"); m.contestOpen != nil || cmd != nil {
		t.Fatal("q closes the modal without quitting")
	}
}

func TestLiveSearchFiltersWhileTypingAndEscUndoes(t *testing.T) {
	ps := []cf.Problem{
		{ContestID: 1, Index: "A", Name: "Watermelon"},
		{ContestID: 2, Index: "A", Name: "Way Too Long Words"},
		{ContestID: 3, Index: "A", Name: "Team"},
	}
	m := New(ps, "", Deps{})
	m, _ = send(m, "/")
	for _, k := range []string{"w", "m", "l"} {
		m, _ = send(m, k)
	}
	if len(m.visible) != 1 || m.visible[0].Name != "Watermelon" {
		t.Fatalf("typing must filter live (fuzzy): %v", m.visible)
	}
	m, _ = send(m, "esc")
	if len(m.visible) != 3 || m.filter.Search != "" {
		t.Fatalf("esc undoes the search: %v", m.visible)
	}
	m, _ = send(m, "/")
	m, _ = send(m, "t")
	m, _ = send(m, "enter")
	if m.input != nil || m.filter.Search != "t" {
		t.Fatal("enter keeps the search")
	}
}

func TestSplitViewHasNoTabs(t *testing.T) {
	m := splitModel(t, Deps{})
	if strings.Contains(m.View().Content, "\t") {
		t.Fatal("tabs break column alignment")
	}
}

func TestFilterModalRatingStatusTagsAndSort(t *testing.T) {
	ps := []cf.Problem{
		{ContestID: 3, Index: "A", Name: "Alpha", Rating: 800, Tags: []string{"dp"}},
		{ContestID: 2, Index: "B", Name: "Beta", Rating: 1500, Tags: []string{"dp", "math"}},
		{ContestID: 1, Index: "C", Name: "Gamma", Rating: 1000, Tags: []string{"math"}},
	}
	m := New(ps, "", Deps{})
	m, _ = send(m, "f")
	if m.fm == nil {
		t.Fatal("f opens the filter modal")
	}
	// rating min: type 9 0 0 -> 900
	for _, k := range []string{"9", "0", "0"} {
		m, _ = send(m, k)
	}
	if m.filter.MinRating != 900 || len(m.visible) != 2 {
		t.Fatalf("min rating: %d %v", m.filter.MinRating, m.visible)
	}
	// tags: tab x4 to the tag list, type "math", space includes it
	for range 4 {
		m, _ = send(m, "tab")
	}
	for _, k := range []string{"m", "a", "t", "h"} {
		m, _ = send(m, k)
	}
	m, _ = send(m, " ")
	if len(m.filter.Include) != 1 || m.filter.Include[0] != "math" || len(m.visible) != 2 {
		t.Fatalf("tag include: %+v %v", m.filter.Include, m.visible)
	}
	m, _ = send(m, " ") // include -> exclude
	if len(m.filter.Exclude) != 1 || len(m.visible) != 0 {
		t.Fatalf("tag exclude: %+v %v", m.filter.Exclude, m.visible)
	}
	if !strings.Contains(plain(m), "Filters") {
		t.Fatal("modal should be drawn")
	}
	m, _ = send(m, "esc")
	if m.fm != nil || !strings.Contains(plain(m), "rating ≥ 900") || !strings.Contains(plain(m), "math") {
		t.Fatalf("chips should show the active filters:\n%s", plain(m))
	}
	m, _ = send(m, "X")
	if len(m.visible) != 3 || m.filter.Active() {
		t.Fatal("X clears every filter")
	}
}

func TestSortOrdersTheList(t *testing.T) {
	ps := []cf.Problem{
		{ContestID: 3, Index: "A", Name: "Alpha", Rating: 1500},
		{ContestID: 2, Index: "B", Name: "Beta", Rating: 800},
		{ContestID: 1, Index: "C", Name: "Gamma"},
	}
	m := New(ps, "", Deps{})
	m.sortBy = "rating"
	m = m.refilter()
	if m.visible[0].Name != "Beta" || m.visible[2].Name != "Gamma" {
		t.Fatalf("easiest first, unrated last: %v", m.visible)
	}
	m.sortBy = "-rating"
	m = m.refilter()
	if m.visible[0].Name != "Alpha" || m.visible[2].Name != "Gamma" {
		t.Fatalf("hardest first, unrated last: %v", m.visible)
	}
}
