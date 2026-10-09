package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
)

func click(m Model, x, y int) (Model, tea.Cmd) {
	nm, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return nm.(Model), cmd
}

func wheel(m Model, x, y int, up bool) Model {
	b := tea.MouseWheelDown
	if up {
		b = tea.MouseWheelUp
	}
	nm, _ := m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: b})
	return nm.(Model)
}

func listModel() Model {
	var ps []cf.Problem
	for i := 0; i < 40; i++ {
		ps = append(ps, cf.Problem{ContestID: 100 + i, Index: "A", Name: "P" + strings.Repeat("x", i%5), Rating: 800 + 100*(i%10)})
	}
	return New(ps, "", Deps{Mouse: true, WheelLines: 3, MouseSelect: true})
}

func TestMouseTabsListAndWheel(t *testing.T) {
	m := listModel()
	r := tabRanges()
	m, _ = click(m, r[2][0]+2, 0)
	if m.tab != 2 {
		t.Fatalf("clicking the Stats pill switches tab: %d", m.tab)
	}
	m, _ = click(m, r[0][0]+2, 0)
	if m.tab != 0 {
		t.Fatal("back to Problems")
	}
	// rows start under header, rule, chips and column header
	m, _ = click(m, 10, 4+3)
	if m.cursor != 3 || m.open != nil {
		t.Fatalf("first click selects: cursor=%d", m.cursor)
	}
	m, cmd := click(m, 10, 4+3)
	if m.open == nil || m.open.ContestID != 103 || cmd == nil {
		t.Fatalf("second click opens: %+v", m.open)
	}
	m, _ = send(m, "esc")
	m = wheel(m, 5, 8, false)
	if m.cursor != 6 {
		t.Fatalf("wheel moves the cursor by 3: %d", m.cursor)
	}
	m = wheel(m, 5, 8, true)
	if m.cursor != 3 {
		t.Fatalf("wheel up: %d", m.cursor)
	}
}

func TestMouseChipsBarOpensFiltersAndOutsideClickCloses(t *testing.T) {
	m := listModel()
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = nm.(Model)
	m, _ = click(m, 20, 2)
	if m.fm == nil {
		t.Fatal("clicking the chips bar opens the filters")
	}
	m, _ = click(m, 0, 0) // outside the modal
	if m.fm != nil {
		t.Fatal("clicking outside closes the modal")
	}
	m, _ = send(m, "?")
	if !m.help {
		t.Fatal("? opens help")
	}
	m, _ = click(m, 0, 39)
	if m.help {
		t.Fatal("clicking outside closes help")
	}
}

func TestMouseSettingsCyclesThemeAndSaves(t *testing.T) {
	var saved []string
	deps := Deps{
		Settings:    map[string]string{"theme": "terminal"},
		SaveSetting: func(k, v string) error { saved = append(saved, k+"="+v); return nil },
	}
	m := New(nil, "", deps)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = nm.(Model)
	m, _ = send(m, "5")
	// first settings row (Theme) is at content row 3 (border 2, heading 3, row 4)
	m, _ = click(m, 6, contentTop+1+1)
	if settingDefs[m.setSel].key != "theme" {
		t.Fatalf("click selects the row: %s", settingDefs[m.setSel].key)
	}
	m, _ = click(m, 30, contentTop+1+1)
	if len(saved) != 1 || !strings.HasPrefix(saved[0], "theme=") || m.setting("theme") == "terminal" {
		t.Fatalf("clicking the value cycles the theme: %v", saved)
	}
	if m.theme.Name != m.setting("theme") {
		t.Fatal("the theme applies live")
	}
}

func TestMouseSplitViewSelectsTest(t *testing.T) {
	m := splitModel(t, Deps{Customs: func(cf.Problem) ([]Case, error) {
		return []Case{{Name: "custom-1", Input: "6\n", Want: "NO\n"}}, nil
	}})
	g, ok := m.splitGeom()
	if !ok {
		t.Fatal("split expected")
	}
	// second test row: tests box starts after the info box; border row then the rows
	m, _ = click(m, g.lw+4, contentTop+g.infoH+1+1)
	if m.pane != paneTests || m.tsel != 1 {
		t.Fatalf("click selects the test: pane=%d tsel=%d", m.pane, m.tsel)
	}
	m, _ = click(m, 3, contentTop+3)
	if m.pane != paneStatement {
		t.Fatal("click on the statement focuses it")
	}
}

func TestMouseViewEnablesMouse(t *testing.T) {
	if listModel().View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("mouse must be on")
	}
}

func TestSettingsDirectAsksForSession(t *testing.T) {
	var cookie, ua string
	deps := Deps{
		Settings:    map[string]string{"submit_mode": "browser"},
		SaveSetting: func(k, v string) error { return nil },
		HasCreds:    func() bool { return false },
		SaveCreds:   func(c, u string) (string, error) { cookie, ua = c, u; return "keyring", nil },
	}
	m := New(nil, "", deps)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = send(nm.(Model), "5")
	m, _ = send(m, "G") // submit mode is the last row
	m, _ = send(m, "right")
	if m.setEdit == nil || m.setEdit.cred != 1 {
		t.Fatalf("choosing direct opens the paste prompt: %+v", m.setEdit)
	}
	nm, _ = m.Update(tea.PasteMsg{Content: "a=b; c=d"})
	m = nm.(Model)
	if strings.Contains(plain(m), "a=b") {
		t.Fatal("cookie must be masked")
	}
	m, _ = send(m, "enter")
	nm, _ = m.Update(tea.PasteMsg{Content: "Mozilla/5.0"})
	m, _ = send(nm.(Model), "enter")
	if cookie != "a=b; c=d" || ua != "Mozilla/5.0" || m.setEdit != nil || !strings.Contains(m.setNote, "keyring") {
		t.Fatalf("cookie=%q ua=%q edit=%v note=%q", cookie, ua, m.setEdit, m.setNote)
	}
}
