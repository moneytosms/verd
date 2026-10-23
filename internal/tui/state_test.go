package tui

import (
	"testing"

	"github.com/moneytosms/verd/internal/cf"
)

func TestStateRoundTrip(t *testing.T) {
	ps := []cf.Problem{{ContestID: 1, Index: "A", Name: "Easy", Rating: 800}, {ContestID: 2, Index: "B", Name: "Hard", Rating: 1600}}
	m := New(ps, "", Deps{})
	m.filter, m.filterExpr, m.sortBy = Filter{MinRating: 1000}, "1000-", "-rating"
	m = m.refilter()
	m, _ = send(m, "enter") // opens 2B, the only match
	m.tab = 1

	r := New(ps, "", Deps{}).WithState(m.State())
	if r.tab != 1 || r.filterExpr != "1000-" || r.sortBy != "-rating" || len(r.visible) != 1 {
		t.Fatalf("tab %d expr %q sort %q visible %d", r.tab, r.filterExpr, r.sortBy, len(r.visible))
	}
	if r.resume == nil || r.resume.Index != "B" {
		t.Fatalf("resume = %v", r.resume)
	}
	nm, _ := r.Update(resumeMsg{*r.resume})
	r = nm.(Model)
	if r.open == nil || r.open.Index != "B" || r.tab != 1 || r.openTab != 0 {
		t.Fatalf("open %v tab %d openTab %d", r.open, r.tab, r.openTab)
	}

	gone := New(ps[:1], "", Deps{}).WithState(m.State()) // a Problem no longer cached is skipped
	if gone.resume != nil {
		t.Fatal("resume should skip a missing Problem")
	}
}
