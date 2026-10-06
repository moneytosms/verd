package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

var helpPages = []string{"This screen", "Everywhere", "Mouse", "Guide"}

// keys lists the current screen's own key bindings.
func (m Model) keys() (screen string, keys [][2]string) {
	switch {
	case m.open != nil:
		return "Problem", [][2]string{{"tab, shift+tab", "move between statement, tests and detail panes"}, {"j/k, pgup/pgdn, g/G", "scroll the pane, or move the test selection"}, {"e", "edit Solution in Neovim"}, {"y", "copy the focused pane (question, tests or selected test)"}, {"T", "manage tests: view, add, edit, copy, delete"}, {"a", "add a Custom Test"}, {"l", "switch language"}, {"s", "submit (copy Solution, open Codeforces, track Verdict)"}, {"t", "run tests"}, {"S", "stress test (esc cancels, w saves counterexample)"}, {"c", "cycle Comparison Mode (tokens, exact, float, none)"}, {"n/p", "select next/previous test"}, {"d", "diff selected failing test"}, {"r", "refetch statement"}, {"o", "open in browser"}, {"esc", "back"}, {"q", "close a modal; quit when none is open"}, {"?", "toggle help"}}
	case m.input != nil:
		return "Prompt", [][2]string{{"enter", "apply"}, {"esc", "cancel"}, {"ctrl+u", "clear"}}
	case m.tab == 0:
		return "Problems", [][2]string{{"j/k, pgup/pgdn", "move"}, {"enter", "open Problem"}, {"f", "filters: rating, status, sort, tags (modal)"}, {":", "filter expression: 800-1200 +dp -graphs unsolved"}, {"X", "clear all filters"}, {"/", "live fuzzy search (tab: all/name/tag/id; #dp matches tags)"}}
	case m.tab == 1 && m.contestOpen != nil:
		return "Contest", [][2]string{{"j/k", "move"}, {"enter", "open Problem"}, {"q, esc", "close"}}
	case m.tab == 1:
		return "Contests", [][2]string{{"j/k, pgup/pgdn", "move"}, {"enter", "list Problems"}}
	case m.tab == 2:
		return "Stats", [][2]string{{"j/k, pgup/pgdn", "scroll"}, {"n/N", "select next/previous attempted Problem"}, {"enter", "open it"}, {"p", "Problem Picker with the weak-topics preset"}}
	case m.tab == 3:
		return "Picker", [][2]string{{"space, r", "re-roll"}, {"enter", "open the Problem"}, {"f", "filters: 800-1200 +dp -graphs"}, {"w", "weak-topics preset"}}
	}
	return "Settings", [][2]string{{"j/k", "move"}, {"←/→, space", "change the value (saved at once)"}, {"enter", "edit a text value; enter saves, esc cancels"}, {"e", "open config.toml in your editor"}}
}

var globalKeys = [][2]string{
	{"1-5, tab", "switch tab: Problems, Contests, Stats, Picker, Settings"},
	{"ctrl+r", "refresh from Codeforces"},
	{"?", "this help"},
	{"esc", "go back, close a modal, cancel a prompt"},
	{"q", "close a modal; quit when none is open"},
	{"x", "dismiss a notification"},
	{"ctrl+c", "quit from anywhere"},
}

var mouseHelp = [][2]string{
	{"click a tab", "switch screen"},
	{"click a row", "select it; click the selected row again to open it"},
	{"click the filter bar", "open the filters"},
	{"click a pane", "focus it (Problem view); click a test to select it"},
	{"click a setting", "select it; click its value to change it"},
	{"click outside a modal", "close it"},
	{"wheel", "scroll the list, pane or modal under the pointer"},
	{"shift + drag", "select text (your terminal handles it while verd uses the mouse)"},
}

var guide = []string{
	"1  Pick     Browse Problems (1). / searches fuzzily, f filters by rating, status and tags.",
	"2  Edit     enter opens a Problem; e opens Neovim next to verd on your Solution.",
	"3  Test     t runs Sample and Custom Tests (or just save: autotest). tab moves to the tests;",
	"            the selected test shows input, expected and your output. d diffs a failure.",
	"4  Cases    a adds a test, T manages them: edit, copy a sample, delete.",
	"5  Stress   S creates gen and brute, then hunts for a counterexample. w saves it as a test.",
	"6  Submit   s sends the Solution. The Submission modal shows the Verdict live.",
	"7  Tune     Settings (5) changes theme, language, editor and submit mode without editing files.",
	"",
	"Docs: docs/keys.md, docs/config.md, docs/testing.md, docs/submit.md",
}

// helpLines renders one help page as lines.
func (m Model) helpLines(page int) []string {
	st := m.styles()
	cap := func(k string) string { return st.Chip.Render(" " + k + " ") }
	var rows [][2]string
	switch page {
	case 0:
		_, rows = m.keys()
	case 1:
		rows = globalKeys
	case 2:
		rows = mouseHelp
	default:
		out := make([]string, len(guide))
		for i, g := range guide {
			if g != "" && g[0] >= '1' && g[0] <= '9' {
				out[i] = st.Accent2.Render(g[:12]) + g[12:]
			} else {
				out[i] = st.Dim.Render(g)
			}
		}
		return out
	}
	var out []string
	kw := 0
	for _, r := range rows {
		kw = max(kw, len([]rune(r[0])))
	}
	for _, r := range rows {
		out = append(out, cap(fit(r[0], kw))+" "+r[1])
	}
	return out
}

func (m Model) helpBox() []string {
	screen, _ := m.keys()
	st := m.styles()
	w, h := m.modalSize(92, 34)
	var tabsRow []string
	for i, p := range helpPages {
		if i == m.helpPage {
			tabsRow = append(tabsRow, st.PillOn.Render(" "+p+" "))
		} else {
			tabsRow = append(tabsRow, st.PillOff.Render(" "+p+" "))
		}
	}
	body := []string{strings.Join(tabsRow, " "), ""}
	lines := m.helpLines(m.helpPage)
	room := max(3, h-2-len(body))
	off := min(m.helpScroll, max(0, len(lines)-room))
	body = append(body, lines[off:min(len(lines), off+room)]...)
	note := "←/→ page  j/k scroll  q close"
	if len(lines) > room {
		note = fmt.Sprintf("%d/%d  ", off+1, len(lines)) + note
	}
	return box("Keys: "+screen, body, note, w, min(h, len(body)+2), true, st)
}

func (m Model) updateHelp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q":
		m.help = false
	case "ctrl+c":
		return m, tea.Quit
	case "right", "l", "tab":
		m.helpPage, m.helpScroll = (m.helpPage+1)%len(helpPages), 0
	case "left", "h", "shift+tab":
		m.helpPage, m.helpScroll = (m.helpPage+len(helpPages)-1)%len(helpPages), 0
	case "down", "j":
		m.helpScroll++
	case "up", "k":
		m.helpScroll = max(0, m.helpScroll-1)
	case "1", "2", "3", "4":
		m.helpPage, m.helpScroll = int(msg.String()[0]-'1'), 0
	}
	return m, nil
}
