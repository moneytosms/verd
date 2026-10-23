package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

func TestPaintLineKeepsBackground(t *testing.T) {
	sgr := bgSGR("#ffffff")
	if sgr != "\x1b[48;2;255;255;255m" {
		t.Fatalf("bgSGR: %q", sgr)
	}
	got := paintLine("a\x1b[1mb\x1b[0mc", sgr, 6)
	want := sgr + "a\x1b[1mb\x1b[0m" + sgr + "c   \x1b[0m"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNightWindow(t *testing.T) {
	if nightNow("7") {
		t.Fatal("a night starting at 07:00 never happens")
	}
	if nightNow("0") != (time.Now().Hour() < 7) {
		t.Fatal("night from 0 is 00:00 to 07:00")
	}
}

func TestPKeyCyclesPalette(t *testing.T) {
	var saved []string
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p>Hello</p></div>`}, nil
		},
		SaveSetting: func(k, v string) error { saved = append(saved, k+"="+v); return nil },
	}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A", Name: "Hi"}}, "", deps), "enter")
	nm, _ := m.Update(cmd())
	m = nm.(Model)

	m, _ = send(m, "P")
	if got := m.setting("reading_palette"); got != "paper" {
		t.Fatalf("first P: palette %q", got)
	}
	if len(saved) != 1 || saved[0] != "reading_palette=paper" {
		t.Fatalf("saved %v", saved)
	}
	// The body rendered for the old palette is replaced at once: bodyW 0 forces the re-render.
	m.body, m.bodyW = []string{"stale"}, m.stmtWidth()
	m, _ = send(m, "P")
	if m.setting("reading_palette") != "sepia" || m.body[0] == "stale" {
		t.Fatalf("second P should re-render: palette %q body %q", m.setting("reading_palette"), m.body)
	}
	for i := 0; i < 6; i++ {
		m, _ = send(m, "P")
	}
	if m.setting("reading_palette") != "app" {
		t.Fatalf("P should wrap to app, got %q", m.setting("reading_palette"))
	}
	if !strings.Contains(plain(m), "Hello") {
		t.Fatal("statement missing after cycling")
	}
}

func TestSplitCapWidth(t *testing.T) {
	m := New([]cf.Problem{{ContestID: 1, Index: "A"}}, "", Deps{})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = nm.(Model)
	m.cfgVals = map[string]string{"reading_split": "cap", "reading_width": "0"}
	if _, _, ok := m.split(); !ok {
		t.Fatal("want split view at this size")
	}
	if w := m.readingOptions().Width; w != 76 {
		t.Fatalf("cap: width %d", w)
	}
	m.cfgVals["reading_split"] = "full"
	if w := m.readingOptions().Width; w != 0 {
		t.Fatalf("full: width %d", w)
	}
}

func TestPaletteBackgroundInSplitView(t *testing.T) {
	deps := Deps{Load: func(cf.Problem, bool) (*scrape.Detail, error) {
		return &scrape.Detail{Statement: `<div class="problem-statement"><p>Hello</p></div>`}, nil
	}}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A", Name: "Hi"}}, "", deps), "enter")
	nm, _ := m.Update(cmd())
	m = nm.(Model)
	nm, _ = m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = nm.(Model)
	m.cfgVals["reading_palette"] = "paper"
	m.bodyW = 0
	m = m.rewrap()
	if out := m.View().Content; !strings.Contains(out, "\x1b[48;2;247;241;227m") {
		t.Fatal("paper background missing from the split view")
	}
	m.cfgVals["reading_palette"] = "app"
	m.bodyW = 0
	m = m.rewrap()
	if out := m.View().Content; strings.Contains(out, "\x1b[48;2;247;241;227m") {
		t.Fatal("app palette should not paint paper")
	}
}
