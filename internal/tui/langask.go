package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// typing reports whether keys go into a text field: a search, or a setting being edited.
func (m Model) typing() bool { return m.conFind || m.setFind || m.setEdit != nil }

// chooseLang makes lang the Problem's language: its template is created, and the choice is saved.
func (m Model) chooseLang(lang string) Model {
	if m.deps.Ensure != nil {
		if err := m.deps.Ensure(*m.open, lang); err != nil {
			m.errMsg = "language " + lang + ": " + err.Error()
			return m
		}
	}
	m.pstate.Lang = lang
	m.errMsg = ""
	if m.deps.SaveState != nil {
		if err := m.deps.SaveState(*m.open, m.pstate); err != nil {
			m.errMsg = "saving language: " + err.Error()
		}
	}
	return m
}

// updateLangAsk handles the language picker opened by default_lang "ask".
func (m Model) updateLangAsk(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.langSel = max(0, m.langSel-1)
	case "down", "j":
		m.langSel = min(len(m.deps.Langs)-1, m.langSel+1)
	case "enter":
		m.langAsk = false
		m = m.chooseLang(m.deps.Langs[m.langSel])
	case "esc":
		m.langAsk = false // not saved: asked again the next time this Problem opens
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// langAskBox is the picker: one row per configured language.
func (m Model) langAskBox() []string {
	st := m.styles()
	w, _ := m.modalSize(48, 24)
	body := []string{st.Dim.Render("Solve this Problem in:"), ""}
	for i, l := range m.deps.Langs {
		cur := " "
		if i == m.langSel {
			cur = st.Accent.Render(">")
		}
		body = append(body, fmt.Sprintf("%s %s", cur, clean(l)))
	}
	return box("Language", body, m.kx("{contest.down}/{contest.up} move  enter choose  esc later"), w, len(body)+2, true, st, m.borderStyle())
}
