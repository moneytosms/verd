package tui

import (
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/editorial"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/session"
)

func editorialDeps(res *editorial.Result, opened *string) Deps {
	return Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p>hi</p></div>`, TimeLimitMS: 1000}, nil
		},
		Editorial: func(cf.Problem) (*editorial.Result, error) { return res, nil },
		OpenURL:   func(u string) error { *opened = u; return nil },
	}
}

func openEditorialFor(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := send(m, "enter")
	m, _ = step(t, m, cmd)
	m, cmd = send(m, "H")
	if m.edv == nil || !m.edv.loading || !strings.Contains(plain(m), "loading the editorial") {
		t.Fatalf("H opens the editorial modal:\n%s", plain(m))
	}
	m, _ = step(t, m, cmd)
	return m
}

func TestEditorialRevealsStrictlyInOrder(t *testing.T) {
	res := &editorial.Result{
		Status: editorial.OK, Title: "Round 911 Editorial", URL: "https://codeforces.com/blog/entry/122677",
		Blocks: []editorial.Block{
			{HTML: "<p>Idea: think about water.</p>"},
			{Title: "Hint 1", HTML: "<p>FIRSTHINT text</p>", Spoiler: true},
			{Title: "Solution", HTML: "<p>THESOLUTION text</p>", Spoiler: true},
		},
		Links: []editorial.Material{{Kind: editorial.Video, Label: "Video Tutorial", URL: "https://codeforces.com/blog/entry/9"}},
	}
	var opened string
	m := New([]cf.Problem{{ContestID: 1900, Index: "A", Name: "N"}}, "", editorialDeps(res, &opened))
	m = openEditorialFor(t, m)

	out := plain(m)
	for _, want := range []string{"Idea: think about water", "▸ Hint 1", "to reveal", "▸ Solution (locked)", "0/2 hints", "More materials", "Video Tutorial"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "FIRSTHINT") || strings.Contains(out, "THESOLUTION") {
		t.Fatalf("spoilers must stay closed:\n%s", out)
	}
	m, _ = send(m, "enter")
	out = plain(m)
	if !strings.Contains(out, "FIRSTHINT") || strings.Contains(out, "THESOLUTION") || !strings.Contains(out, "▸ Solution") || !strings.Contains(out, "1/2 hints") {
		t.Fatalf("one at a time:\n%s", out)
	}
	m, _ = send(m, "enter")
	out = plain(m)
	if !strings.Contains(out, "THESOLUTION") || !strings.Contains(out, "2/2 hints") {
		t.Fatalf("second reveal:\n%s", out)
	}
	m, _ = send(m, "enter") // nothing left to open
	if m.edv.revealed != 2 {
		t.Fatalf("revealed %d", m.edv.revealed)
	}
	m, cmd := send(m, "o")
	if cmd == nil {
		t.Fatal("o opens the post")
	}
	cmd()
	if opened != res.URL {
		t.Fatalf("opened %q", opened)
	}
	m, _ = send(m, "esc")
	if m.edv != nil || m.open == nil {
		t.Fatal("esc closes the editorial only")
	}
}

func TestEditorialNoneAndHiddenInSession(t *testing.T) {
	none := &editorial.Result{Status: editorial.None, Links: []editorial.Material{{Kind: editorial.Stream, Label: "Discussion stream", URL: "https://codeforces.com/blog/entry/2"}}}
	var opened string
	ps := []cf.Problem{{ContestID: 1900, Index: "A", Name: "N"}}
	m := openEditorialFor(t, New(ps, "", editorialDeps(none, &opened)))
	out := plain(m)
	if !strings.Contains(out, "No editorial is published") || !strings.Contains(out, "Discussion stream") {
		t.Fatalf("no editorial, links only:\n%s", out)
	}
	ru := &editorial.Result{Status: editorial.NotEnglish}
	m = openEditorialFor(t, New(ps, "", editorialDeps(ru, &opened)))
	if !strings.Contains(plain(m), "Only a Russian editorial exists") {
		t.Fatalf("Russian only:\n%s", plain(m))
	}

	// a running session on that contest hides it
	d := editorialDeps(none, &opened)
	m = New(ps, "", d).WithData(Data{Problems: ps, Session: &session.Session{ContestID: 1900, Mode: session.Virtual, Start: 1, End: 1 << 40}})
	m, cmd := send(m, "enter")
	m, _ = step(t, m, cmd)
	m, cmd = send(m, "H")
	if m.edv != nil || cmd != nil || !strings.Contains(m.Notice, "hidden during the session") {
		t.Fatalf("editorial must be hidden in a session: %q", m.Notice)
	}

	// CSES has none
	cs := []cf.Problem{{ContestID: 1068, Index: cf.SourceCSES, Name: "x"}}
	m = New(cs, "", editorialDeps(none, &opened))
	m, cmd = send(m, "enter")
	m, _ = step(t, m, cmd)
	m, _ = send(m, "H")
	if m.edv != nil || !strings.Contains(m.Notice, "only Codeforces") {
		t.Fatalf("CSES: %q", m.Notice)
	}
}
