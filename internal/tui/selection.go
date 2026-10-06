package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// selection is a mouse drag over whole lines of the statement or detail pane; on release the
// lines go to the clipboard (OSC 52). Terminal-native selection needs shift+drag instead.
type selection struct {
	pane, a, b int  // b is where the pointer is; a is where the drag began
	on, moved  bool // on: highlight shown; moved: the pointer left the first line
}

// stmtBody is every line of the statement pane, including the loading and error notes.
func (m Model) stmtBody() []string {
	body := m.stmtLines()
	if m.loading {
		body = append(body, "", "loading...")
	}
	if m.errMsg != "" {
		body = append(body, "", m.styles().Bad.Render(clean(m.errMsg)))
	}
	return body
}

// detailBody is every line of the detail pane (the stress panel while a stress run owns it).
func (m Model) detailBody(w int) []string {
	if m.strs != nil {
		return m.stressPanel()
	}
	dl, _ := m.detailLines(w)
	return dl
}

// selAt maps a screen cell to a line of the statement or detail pane, clamping to the pane
// so a drag past its edge keeps extending the selection.
func (m Model) selAt(x, y int, pane int) (idx int, ok bool) {
	g, gok := m.splitGeom()
	if !gok {
		return 0, false
	}
	var body []string
	var top, inner, scroll int
	switch pane {
	case paneStatement:
		body, top, inner, scroll = m.stmtBody(), contentTop+1, g.h-2, m.scroll
	case paneDetail:
		if g.detH == 0 {
			return 0, false
		}
		body, top, inner, scroll = m.detailBody(g.rw-4), contentTop+g.infoH+g.testsH+1, g.detH-2, m.detScroll
	default:
		return 0, false
	}
	if len(body) == 0 {
		return 0, false
	}
	off := min(scroll, max(0, len(body)-inner))
	return max(0, min(off+max(0, min(y-top, inner-1)), len(body)-1)), true
}

// paneAt is the selectable pane under a screen cell, if any.
func (m Model) paneAt(x, y int) (int, bool) {
	g, ok := m.splitGeom()
	if !ok {
		return 0, false
	}
	switch {
	case x < g.lw-1:
		return paneStatement, true
	case x > g.lw && g.detH > 0 && y >= contentTop+g.infoH+g.testsH:
		return paneDetail, true
	}
	return 0, false
}

func (m Model) startSelect(x, y int) Model {
	m.sel = selection{}
	if p, ok := m.paneAt(x, y); ok {
		if i, ok := m.selAt(x, y, p); ok {
			m.sel = selection{pane: p, a: i, b: i, on: true}
		}
	}
	return m
}

func (m Model) dragSelect(x, y int) Model {
	if !m.sel.on {
		return m
	}
	if i, ok := m.selAt(x, y, m.sel.pane); ok {
		m.sel.b = i
		m.sel.moved = m.sel.moved || i != m.sel.a
	}
	return m
}

// endSelect copies the dragged lines. A plain click (no movement) selects nothing.
func (m Model) endSelect() (Model, tea.Cmd) {
	s := m.sel
	if !s.on {
		return m, nil
	}
	if !s.moved {
		m.sel = selection{}
		return m, nil
	}
	lo, hi := min(s.a, s.b), max(s.a, s.b)
	var body []string
	if s.pane == paneStatement {
		body = m.stmtBody()
	} else if g, ok := m.splitGeom(); ok {
		body = m.detailBody(g.rw - 4)
	}
	if hi >= len(body) {
		return m, nil
	}
	var b strings.Builder
	for _, l := range body[lo : hi+1] {
		b.WriteString(strings.TrimRight(xansiStrip(l), " ") + "\n")
	}
	m.Notice = "copied selection"
	return m, tea.SetClipboard(b.String())
}

// highlight reverses the selected lines of a pane's visible slice (lines off..off+len).
func (m Model) highlight(pane, off int, lines []string) []string {
	s := m.sel
	if !s.on || !s.moved || s.pane != pane {
		return lines
	}
	lo, hi := min(s.a, s.b), max(s.a, s.b)
	out := append([]string(nil), lines...)
	for i := range out {
		if n := off + i; n >= lo && n <= hi {
			out[i] = "\x1b[7m" + xansiStrip(out[i]) + "\x1b[27m"
		}
	}
	return out
}
