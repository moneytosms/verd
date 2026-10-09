package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

var helpPages = []string{"This screen", "Everywhere", "Mouse", "Guide"}

// actionRows lists the current keys and descriptions of the actions in the given groups.
func (m Model) actionRows(groups ...string) (rows [][2]string) {
	for _, g := range groups {
		for _, a := range keyActions {
			if a.ctx == g {
				rows = append(rows, [2]string{strings.Join(m.km.keys(a), ", "), a.desc})
			}
		}
	}
	return rows
}

// keys lists the current screen's own key bindings (live from the keymap, so rebinding shows here).
func (m Model) keys() (screen string, keys [][2]string) {
	ctx := m.screenCtx()
	if m.input != nil {
		return "Prompt", [][2]string{{"enter", "apply"}, {"esc", "cancel"}, {"ctrl+u", "clear"}}
	}
	return map[string]string{"problem": "Problem", "problems": "Problems", "contest": "Contest", "contests": "Contests", "stats": "Stats", "picker": "Picker", "settings": "Settings"}[ctx], m.actionRows(ctx)
}

// globalKeys are the shortcuts that work on every screen.
func (m Model) globalKeys() [][2]string {
	return append(m.actionRows("tabs", "app", "common"), [2]string{"ctrl+c", "quit from anywhere (fixed)"})
}

var mouseHelp = [][2]string{
	{"click a tab", "switch screen"},
	{"click a row", "select it; click the selected row again to open it"},
	{"click the filter bar", "open the filters"},
	{"click a pane", "focus it (Problem view); click a test to select it"},
	{"click a setting", "select it; click its value to change it"},
	{"click outside a modal", "close it"},
	{"wheel", "scroll the list, pane or modal under the pointer"},
	{"drag in a pane", "select whole lines of the statement or test detail and copy them (Problem view)"},
	{"shift + drag", "select any text with your terminal instead (verd keeps the mouse otherwise)"},
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
		rows = m.globalKeys()
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
	note := m.kx("{help.page_prev}/{help.page_next} page  {help.scroll_down}/{help.scroll_up} scroll  {help.close} close")
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
