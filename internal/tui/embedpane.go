package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/embed"
)

const (
	defaultRatio = 0.4
	minPane      = 20 // narrower than this and the pane is not worth drawing
)

// embedPane is the embedded editor, drawn as a column (right by default, left with embed_side = left).
type embedPane struct {
	term  *embed.Term
	focus bool // keys go to the editor
	zoom  bool // editor fullscreen, verd hidden
}

// paneLeft reports whether the editor column is on the left.
func (m Model) paneLeft() bool { return m.deps.EmbedSide == "left" }

// layout splits the window into verd's width and the editor's width (verd gets the ratio), with a
// 1-column divider between them. Which side each sits on is paneLeft.
func (m Model) layout() (left, right int) {
	if m.embed.zoom && m.embed.term != nil {
		return 0, m.width
	}
	r := m.deps.EmbedRatio
	if r <= 0 || r >= 1 {
		r = defaultRatio
	}
	left = int(float64(m.width) * r) // verd's width
	right = m.width - left - 1       // the editor's width
	if right < minPane || left < minPane {
		left, right = m.width, 0 // too narrow: verd only
	}
	return left, right
}

// edKeys are the keys bound to an editor action (editor.focus, focus_left, focus_right, zoom).
// embed_focus_key is the older spelling of editor.focus and applies when [keys.editor] doesn't.
func (m Model) edKeys(id string) []string {
	if u := m.km.user["editor."+id]; len(u) > 0 {
		return u
	}
	if id == "focus" && m.deps.FocusKey != "" {
		return []string{m.deps.FocusKey}
	}
	a, _ := findAction("editor." + id)
	return a.def
}

func (m Model) focusKey() string { return m.edKeys("focus")[0] }

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
		if slices.Contains(m.edKeys("zoom"), msg.String()) {
			m.embed.zoom = !m.embed.zoom
			m.embed.focus = m.embed.zoom // hiding verd hands keys to the editor
			if m.embed.focus {
				t.Focus()
			} else {
				t.Blur()
			}
			m.resizeEmbed()
			return m, m.relayout(), true
		}
		want, dir := m.embed.focus, false
		switch key := msg.String(); {
		case slices.Contains(m.edKeys("focus"), key):
			want, dir = !m.embed.focus, true
		case slices.Contains(m.edKeys("focus_left"), key):
			want, dir = m.paneLeft(), true // the editor is in the left column
		case slices.Contains(m.edKeys("focus_right"), key):
			want, dir = !m.paneLeft(), true
		}
		if dir {
			if m.embed.zoom {
				return m, nil, true // verd is hidden; alt+z first
			}
			m.embed.focus = want
			if want {
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
		x0 := left + 1 // editor column's first x
		if m.paneLeft() {
			x0 = 0
		}
		inPane := m.embed.zoom || mouse.X >= x0 && mouse.X < x0+right && mouse.Y < m.height
		if _, click := msg.(tea.MouseClickMsg); click {
			m.embed.focus = inPane // clicking a column gives it the keyboard
		}
		if inPane {
			rel := mouse
			if !m.embed.zoom {
				rel.X -= x0
			}
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
	mouseMode := tea.MouseModeCellMotion
	if !m.deps.Mouse {
		mouseMode = tea.MouseModeNone
	}
	if m.embed.term == nil || right == 0 {
		v := tea.NewView(m.screen())
		v.AltScreen = true
		v.MouseMode = mouseMode
		return v
	}
	if left == 0 { // zoomed: editor only
		v := tea.NewView(m.embed.term.Render())
		v.AltScreen = true
		v.MouseMode = mouseMode
		x, y := m.embed.term.Cursor()
		v.Cursor = tea.NewCursor(x, y)
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
		l += strings.Repeat(" ", max(0, left-xansi.StringWidth(l)))
		r += strings.Repeat(" ", max(0, right-xansi.StringWidth(r)))
		if m.paneLeft() {
			b.WriteString(r + divider + l)
		} else {
			b.WriteString(l + divider + r)
		}
		if i < max(1, m.height)-1 {
			b.WriteByte('\n')
		}
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = mouseMode
	if m.embed.focus {
		x, y := m.embed.term.Cursor()
		if m.paneLeft() {
			v.Cursor = tea.NewCursor(x, y)
		} else {
			v.Cursor = tea.NewCursor(left+1+x, y)
		}
	}
	return v
}

// embedBadge describes where the keyboard is, for the footer.
func (m Model) embedBadge() string {
	if m.embed.term == nil {
		return ""
	}
	if m.embed.focus {
		return fmt.Sprintf("[editor focused, %s returns here, %s zooms]", m.focusKey(), m.edKeys("zoom")[0])
	}
	return fmt.Sprintf("[%s focuses the editor]", m.focusKey())
}

// shiftMouse moves a mouse event into verd's own coordinates when the editor column is on the left.
func (m Model) shiftMouse(msg tea.Msg) tea.Msg {
	mm, ok := msg.(tea.MouseMsg)
	if !ok || m.embed.term == nil || m.embed.zoom || !m.paneLeft() {
		return msg
	}
	if _, right := m.layout(); right > 0 {
		rel := mm.Mouse()
		rel.X -= right + 1
		return translateMouse(mm, rel)
	}
	return msg
}
