package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/embed"
)

const (
	defaultRatio    = 0.4
	defaultFocusKey = "ctrl+\\"
	minPane         = 20 // narrower than this and the pane is not worth drawing
)

// embedPane is the embedded editor, drawn as a right-hand column.
type embedPane struct {
	term  *embed.Term
	focus bool // keys go to the editor
}

// layout splits the window: verd on the left (ratio of the width), a 1-column divider, the pane on the right.
func (m Model) layout() (left, right int) {
	r := m.deps.EmbedRatio
	if r <= 0 || r >= 1 {
		r = defaultRatio
	}
	left = int(float64(m.width) * r)
	right = m.width - left - 1
	if right < minPane || left < minPane {
		left, right = m.width, 0 // too narrow: verd only
	}
	return left, right
}

func (m Model) focusKey() string {
	if m.deps.FocusKey != "" {
		return m.deps.FocusKey
	}
	return defaultFocusKey
}

// resizeEmbed makes the editor match its column.
func (m Model) resizeEmbed() {
	if m.embed.term == nil {
		return
	}
	if _, right := m.layout(); right > 0 {
		m.embed.term.Resize(right, max(1, m.height))
	}
}

// onEmbedMsg handles the adapter's messages; handled=false means msg is not one of them.
func (m Model) onEmbedMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case embed.OpenedMsg:
		m.embed = embedPane{term: msg.Term, focus: true}
		m.resizeEmbed()
		m.editorOpen = true
		return m, m.relayout(), true
	case embed.RedrawMsg:
		if m.embed.term != nil && !m.embed.term.Alive() { // the editor exited
			m.embed = embedPane{}
			m.editorOpen = false
			return m, m.relayout(), true
		}
		return m, nil, true // the View is rebuilt from the emulator
	case embed.FocusMsg:
		if m.embed.term != nil {
			m.embed.focus = true
		}
		return m, nil, true
	}
	return m, nil, false
}

// relayout re-renders the open statement for the new column width.
func (m Model) relayout() tea.Cmd {
	if m.open != nil && m.detail != nil {
		return m.load(*m.open, false)
	}
	return nil
}

// routeEmbedInput sends keys, paste and mouse to the editor while it has focus. handled=true means
// the message is consumed. The focus key always comes back to verd.
func (m Model) routeEmbedInput(msg tea.Msg) (Model, tea.Cmd, bool) {
	t := m.embed.term
	if t == nil {
		return m, nil, false
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == m.focusKey() {
			m.embed.focus = !m.embed.focus
			if m.embed.focus {
				t.Focus()
			} else {
				t.Blur()
			}
			return m, nil, true
		}
		if m.embed.focus {
			t.SendKey(msg)
			return m, nil, true
		}
	case tea.PasteMsg:
		if m.embed.focus {
			t.Paste(msg.Content)
			return m, nil, true
		}
	case tea.MouseMsg:
		left, right := m.layout()
		mouse := msg.Mouse()
		if right == 0 {
			return m, nil, false
		}
		inPane := mouse.X > left && mouse.X < left+1+right && mouse.Y < m.height
		if _, click := msg.(tea.MouseClickMsg); click {
			m.embed.focus = inPane // clicking a column gives it the keyboard
		}
		if inPane {
			rel := mouse
			rel.X -= left + 1
			t.SendMouse(translateMouse(msg, rel))
			return m, nil, true
		}
	}
	return m, nil, false
}

// translateMouse rebuilds msg with pane-relative coordinates.
func translateMouse(msg tea.MouseMsg, rel tea.Mouse) tea.MouseMsg {
	switch msg.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(rel)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(rel)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(rel)
	}
	return tea.MouseMotionMsg(rel)
}

// View draws verd, with the embedded editor as a right column when there is one.
func (m Model) View() tea.View {
	left, right := m.layout()
	if m.embed.term == nil || right == 0 {
		v := tea.NewView(m.screen())
		v.AltScreen = true
		return v
	}
	lm := m
	lm.width = left
	verd := strings.Split(lm.screen(), "\n")
	pane := strings.Split(m.embed.term.Render(), "\n")
	divider := m.styles().Dim.Render("│")
	if m.embed.focus {
		divider = m.styles().Accent.Render("┃")
	}
	var b strings.Builder
	for i := 0; i < max(1, m.height); i++ {
		l, r := "", ""
		if i < len(verd) {
			l = verd[i]
		}
		if i < len(pane) {
			r = xansi.Truncate(pane[i], right, "")
		}
		l = xansi.Truncate(l, left, "")
		b.WriteString(l + strings.Repeat(" ", max(0, left-xansi.StringWidth(l))) + divider + r)
		if i < max(1, m.height)-1 {
			b.WriteByte('\n')
		}
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if m.embed.focus {
		x, y := m.embed.term.Cursor()
		v.Cursor = tea.NewCursor(left+1+x, y)
	}
	return v
}

// embedBadge describes where the keyboard is, for the footer.
func (m Model) embedBadge() string {
	if m.embed.term == nil {
		return ""
	}
	if m.embed.focus {
		return fmt.Sprintf("[editor focused, %s returns here]", m.focusKey())
	}
	return fmt.Sprintf("[%s focuses the editor]", m.focusKey())
}
