package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/relnotes"
)

// whatsNewMsg carries the release notes to show on this launch, when there are any.
type whatsNewMsg struct{ rels []relnotes.Release }

// newsLines is every shown release's notes as screen lines, fitted to width w.
func (m Model) newsLines(w int) []string {
	st := m.styles()
	var out []string
	for _, r := range m.news {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, st.Accent.Render(r.Tag))
		for _, l := range relnotes.Lines(r.Body) {
			switch {
			case l.Heading:
				out = append(out, st.Accent2.Render(l.Text))
			case l.Text == "":
				out = append(out, "")
			default:
				out = append(out, strings.Split(xansi.Wrap(l.Text, w, ""), "\n")...)
			}
		}
	}
	return out
}

// updateNews scrolls the What's new modal; enter, esc or q closes it.
func (m Model) updateNews(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	last := max(0, len(m.newsLines(m.newsWidth()))-1)
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q", "enter":
		m.news = nil
	case "down", "j":
		m.newsOff = min(m.newsOff+1, last)
	case "up", "k":
		m.newsOff = max(0, m.newsOff-1)
	case "pgdown", "space":
		m.newsOff = min(m.newsOff+10, last)
	case "pgup":
		m.newsOff = max(0, m.newsOff-10)
	case "home", "g":
		m.newsOff = 0
	case "end", "G":
		m.newsOff = last
	}
	return m, nil
}

// newsWidth is the text width inside the What's new modal.
func (m Model) newsWidth() int {
	w, _ := m.modalSize(90, 30)
	return w - 6
}

// newsBox is the What's new modal: the notes of every release since the last launch, scrollable.
func (m Model) newsBox() []string {
	st := m.styles()
	w, h := m.modalSize(90, 30)
	lines := m.newsLines(w - 6)
	visible := max(1, h-4)
	off := min(m.newsOff, max(0, len(lines)-visible))
	view := lines[off:min(len(lines), off+visible)]
	note := "↑/↓ scroll · enter close"
	return box("What's new", view, note, w, min(h, len(view)+2), true, st, m.borderStyle())
}
