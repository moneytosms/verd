package tui

import (
	tea "charm.land/bubbletea/v2"
)

// updateAskShown reports whether the update question is open: a newer release is offered and
// nothing else needs the keys (no other modal, no text entry, no embedded editor).
func (m Model) updateAskShown() bool {
	return m.updAsk != "" && m.deps.Update != nil && m.otherModalBox() == nil && m.input == nil && !m.typing() && !m.editorOpen
}

// updateAsk answers the update question: y updates now, n (or esc) leaves a bottom hint.
func (m Model) updateAsk(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y":
		tag := m.updAsk
		m.updAsk = ""
		m.toast = &toast{text: "updating verd to " + tag + "…"}
		return m, m.runUpdate()
	case "n", "esc":
		m.updHint, m.updAsk = m.updAsk, ""
	}
	return m, nil
}

// updateBox is the update question: the offered release and the two answers.
func (m Model) updateBox() []string {
	st := m.styles()
	w, _ := m.modalSize(60, 10)
	body := []string{st.Accent.Render("verd " + m.updAsk + " is available"), "",
		st.Dim.Render("Update now? The release notes show the next time verd starts.")}
	return box("Update verd", body, "y update now · n later", w, len(body)+2, true, st, m.borderStyle())
}
