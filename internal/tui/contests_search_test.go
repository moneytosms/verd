package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

// contestIDs lists the IDs of the shown contests.
func contestIDs(m Model) []int {
	var out []int
	for _, i := range m.contestIdxs() {
		out = append(out, m.contests[i].ID)
	}
	return out
}

func TestContestProgressCount(t *testing.T) {
	cs := []cf.Contest{{ID: 1900, Name: "Round A", Phase: "FINISHED", Start: 2}, {ID: 1901, Name: "Round B", Phase: "FINISHED", Start: 1}}
	ps := []cf.Problem{{ContestID: 1900, Index: "A", Name: "X"}, {ContestID: 1900, Index: "B", Name: "Y"}}
	m := New(ps, "", Deps{}).WithStatuses(map[string]store.Status{"1900A": store.StatusSolved}).WithContests(cs)
	m, _ = send(m, "2")
	if out := plain(m); !strings.Contains(out, "1/2") || !strings.Contains(out, "-") {
		t.Fatalf("want 1/2 and - for uncached contest:\n%s", out)
	}
}

func TestContestChipFilters(t *testing.T) {
	cs := []cf.Contest{
		{ID: 10, Name: "Up Round", Phase: "BEFORE", Start: 2_000_000},
		{ID: 11, Name: "Live Round", Phase: "CODING", Start: 1_500_000},
		{ID: 12, Name: "Old Round", Phase: "FINISHED", Start: 1_000},
	}
	ps := []cf.Problem{{ContestID: 12, Index: "A", Name: "Old"}}
	m := New(ps, "", Deps{}).WithStatuses(map[string]store.Status{"12A": store.StatusSolved}).WithContests(cs)
	m, _ = send(m, "2")
	for _, c := range []struct {
		key  string
		want []int
	}{
		{"u", []int{10}},
		{"u", []int{11, 10, 12}}, // same chip again goes back to all
		{"p", []int{12}},
		{"r", []int{11}},
		{"s", []int{12}},
		{"a", []int{11, 10, 12}},
	} {
		m, _ = send(m, c.key)
		if got := contestIDs(m); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("key %s: want %v, got %v", c.key, c.want, got)
		}
	}
	m, _ = send(m, "p")
	if out := plain(m); !strings.Contains(out, "chip: past") {
		t.Fatalf("footer should show the chip:\n%s", out)
	}
}

func TestContestSortLeastKeepsSections(t *testing.T) {
	cs := []cf.Contest{
		{ID: 10, Name: "Up A", Phase: "BEFORE", Start: 2_000_000},
		{ID: 11, Name: "Up B", Phase: "BEFORE", Start: 3_000_000},
		{ID: 12, Name: "Past", Phase: "FINISHED", Start: 1_000},
	}
	ps := []cf.Problem{{ContestID: 10, Index: "A", Name: "x"}, {ContestID: 10, Index: "B", Name: "y"}, {ContestID: 11, Index: "A", Name: "z"}, {ContestID: 12, Index: "A", Name: "w"}}
	m := New(ps, "", Deps{}).WithStatuses(map[string]store.Status{"10A": store.StatusSolved}).WithContests(cs)
	m, _ = send(m, "2")
	m, _ = send(m, "o")
	if got := contestIDs(m); fmt.Sprint(got) != "[11 10 12]" {
		t.Fatalf("least solved inside sections, upcoming first: got %v", got)
	}
	if out := plain(m); !strings.Contains(out, "sort: least solved") {
		t.Fatalf("footer should show the sort:\n%s", out)
	}
}

func TestContestSearchMatchesProblemName(t *testing.T) {
	cs := []cf.Contest{{ID: 1900, Name: "Educational Round 160", Phase: "FINISHED", Start: 2}, {ID: 1901, Name: "Good Bye 2023", Phase: "FINISHED", Start: 1}}
	ps := []cf.Problem{{ContestID: 1900, Index: "E", Name: "DSU on Tree"}, {ContestID: 1901, Index: "A", Name: "Cut Triangle"}}
	m := New(ps, "", Deps{}).WithContests(cs)
	m, _ = send(m, "2")
	m, _ = send(m, "/")
	for _, r := range "dsu" {
		m, _ = send(m, string(r))
	}
	if got := contestIDs(m); fmt.Sprint(got) != "[1900]" {
		t.Fatalf("want the contest with the DSU problem, got %v", got)
	}
	if out := plain(m); !strings.Contains(out, "E  DSU on Tree") {
		t.Fatalf("matched problem should show at the row end:\n%s", out)
	}
}

func TestContestSearchFiltersAndKeepsCursorVisible(t *testing.T) {
	deps := Deps{Now: func() time.Time { return time.Unix(1_000_000, 0) }}
	cs := []cf.Contest{
		{ID: 1900, Name: "Codeforces Round 917 (Div. 2)", Phase: "FINISHED", Start: 900_000},
		{ID: 1901, Name: "Educational Round 160", Phase: "FINISHED", Start: 800_000},
		{ID: 1902, Name: "Good Bye 2023", Phase: "FINISHED", Start: 700_000},
	}
	m := New(nil, "", deps).WithContests(cs)
	m, _ = send(m, "2")
	m, _ = send(m, "/")
	for _, r := range "edu" {
		m, _ = send(m, string(r))
	}
	if got := m.contestIdxs(); len(got) != 1 || m.contests[got[0]].ID != 1901 {
		t.Fatalf("want only Educational Round, got %v", got)
	}
	if m.contests[m.contestCursor].ID != 1901 {
		t.Fatalf("cursor should follow the match, on %d", m.contests[m.contestCursor].ID)
	}
	if out := plain(m); strings.Contains(out, "Good Bye") {
		t.Fatalf("filtered contest still shown:\n%s", out)
	}
	m, _ = send(m, "enter") // keep the filter
	m, _ = send(m, "esc")   // first esc clears it
	if len(m.contestIdxs()) != 3 || m.conQ != "" {
		t.Fatalf("esc should clear the filter, have %q", m.conQ)
	}
}

func TestContestSearchNoMatchAndNavigation(t *testing.T) {
	cs := []cf.Contest{{ID: 1, Name: "Alpha", Phase: "FINISHED", Start: 2}, {ID: 2, Name: "Beta", Phase: "FINISHED", Start: 1}}
	m := New(nil, "", Deps{}).WithContests(cs)
	m, _ = send(m, "2")
	m, _ = send(m, "/")
	m, _ = send(m, "z")
	if !strings.Contains(plain(m), "no contest matches") {
		t.Fatalf("want empty state:\n%s", plain(m))
	}
	m, _ = send(m, "enter")
	if m.contestOpen != nil {
		t.Fatal("enter must not open a hidden contest")
	}
	m, _ = send(m, "esc") // clears the query, so j moves over the full list
	m, _ = send(m, "j")
	if m.contestCursor != 1 {
		t.Fatalf("j should move through the full list, cursor %d", m.contestCursor)
	}
}

func TestContestsListSurvivesShortTerminal(t *testing.T) {
	cs := []cf.Contest{{ID: 1, Name: "Alpha", Phase: "FINISHED", Start: 2}}
	m := New(nil, "", Deps{}).WithContests(cs)
	m.height = 4
	m, _ = send(m, "2")
	_ = plain(m)
	m, _ = send(m, "j")
	_ = plain(m)
}
