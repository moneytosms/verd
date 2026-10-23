package tui

import (
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

// Typing digits into a text setting must not switch tabs.
func TestDigitsTypedIntoSettingStayOnSettings(t *testing.T) {
	m := New(nil, "", Deps{})
	m.tab = 4
	for i, d := range settingDefs {
		if d.key == "wheel_lines" {
			m.setSel = i
		}
	}
	m, _ = send(m, "enter") // start editing the text setting
	if m.setEdit == nil {
		t.Fatal("enter should open the text edit")
	}
	m, _ = send(m, "5")
	if m.tab != 4 || m.setEdit == nil || m.setEdit.text != "5" {
		t.Fatalf("digit switched tab to %d or was not typed: edit=%+v", m.tab, m.setEdit)
	}
}

// default_lang "ask": opening a Problem with no language chosen shows the picker; enter saves the choice.
func TestAskLanguagePickerSavesChoice(t *testing.T) {
	var saved ProblemState
	deps := Deps{Langs: []string{"cpp", "python"}, DefaultLang: "ask",
		SaveState: func(_ cf.Problem, s ProblemState) error { saved = s; return nil }}
	m := New(nil, "", deps)
	m.cfgVals = map[string]string{"default_lang": "ask"}
	p := cf.Problem{ContestID: 1, Index: "A", Name: "A"}
	m.open, m.openTab, m.loading = &p, 0, true
	nm, _ := m.Update(detailMsg{p: p, d: &scrape.Detail{}, w: 40, body: "x", state: ProblemState{}})
	m = nm.(Model)
	if !m.langAsk {
		t.Fatal("picker should open for an unchosen language")
	}
	m, _ = send(m, "j")
	m, _ = send(m, "enter")
	if m.langAsk || saved.Lang != "python" || m.lang() != "python" {
		t.Fatalf("choice not saved: ask=%v saved=%+v lang=%s", m.langAsk, saved, m.lang())
	}
}
