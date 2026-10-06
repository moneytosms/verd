package tui

import (
	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

const contentTop = 2 // header and rule

// modalRect is where the open modal is drawn (same placement as overlay).
func (m Model) modalRect() (x, y, w, h int, ok bool) {
	bx := m.modalBox()
	if bx == nil {
		return 0, 0, 0, 0, false
	}
	for _, l := range bx {
		w = max(w, xansi.StringWidth(l))
	}
	return max(0, (m.width-w)/2), max(0, (m.height-3-len(bx))/2), w, len(bx), true
}

// closeModal dismisses whichever modal is open.
func (m Model) closeModal() Model {
	m.tm, m.fm, m.help, m.subModal = nil, nil, false, false
	if m.run != nil {
		m.run.diff = false
	}
	if m.strs != nil {
		m.strs.diff = false
	}
	if m.tab == 1 && m.open == nil {
		m.contestOpen, m.cpCursor = nil, 0
	}
	return m
}

func (m Model) onClick(x, y int) (tea.Model, tea.Cmd) {
	if mx, my, mw, mh, ok := m.modalRect(); ok {
		if x < mx || x >= mx+mw || y < my || y >= my+mh {
			if m.tm != nil && m.tm.ed != nil {
				return m, nil // never lose an edit to a stray click
			}
			return m.closeModal(), nil
		}
		return m.clickInModal(x-mx, y-my)
	}
	if y == 0 {
		for i, r := range tabRanges() {
			if x >= r[0] && x < r[1] {
				m = m.closeProblem()
				m.tab = i
				return m.onTab(), nil
			}
		}
		return m, nil
	}
	if m.open != nil {
		return m.clickProblem(x, y)
	}
	switch m.tab {
	case 0:
		return m.clickList(x, y)
	case 1:
		return m.clickContests(y)
	case 4:
		return m.clickSettings(x, y)
	}
	return m, nil
}

func (m Model) clickList(x, y int) (tea.Model, tea.Cmd) {
	if y == contentTop { // the chips bar opens the filters
		return m.openFilters(), nil
	}
	i := m.listStart() + (y - (contentTop + 2))
	if y < contentTop+2 || i >= len(m.visible) {
		return m, nil
	}
	if i == m.cursor { // a second click on the selected row opens it
		return m.openProblem(m.visible[i])
	}
	m.cursor = i
	return m, nil
}

// contestLines maps each content row of the contests list to a contest index (-1 for a heading).
func (m Model) contestLines() []int {
	rows := m.page() - 2
	start := max(0, min(m.contestCursor-rows/2, len(m.contests)-rows))
	var out []int
	last := ""
	for i := start; i < min(start+rows, len(m.contests)); i++ {
		section := "Past"
		if i < m.upcoming {
			section = "Upcoming"
		}
		if section != last {
			out = append(out, -1)
			last = section
		}
		out = append(out, i)
	}
	return out
}

func (m Model) clickContests(y int) (tea.Model, tea.Cmd) {
	lines := m.contestLines()
	r := y - contentTop
	if r < 0 || r >= len(lines) || lines[r] < 0 {
		return m, nil
	}
	if lines[r] == m.contestCursor {
		c := m.contests[m.contestCursor]
		m.contestOpen, m.cpCursor = &c, 0
		return m, nil
	}
	m.contestCursor = lines[r]
	return m, nil
}

func (m Model) clickSettings(x, y int) (tea.Model, tea.Cmd) {
	h := m.paneHeight()
	lw := min(max(46, m.contentWidth()*3/5), m.contentWidth()-30)
	if x >= lw {
		return m, nil
	}
	lines := m.settingLines(lw - 4)
	start := m.settingsStart(lines, h-2)
	r := start + (y - contentTop - 1)
	if r < 0 || r >= len(lines) || lines[r].idx < 0 {
		return m, nil
	}
	d := settingDefs[lines[r].idx]
	if lines[r].idx == m.setSel && x >= 22 { // a click on the value changes it
		if d.kind == "text" {
			m.setEdit = &settingEdit{text: m.setting(d.key)}
			return m, nil
		}
		return m.step(d, 1), nil
	}
	m.setSel, m.setEdit = lines[r].idx, nil
	return m, nil
}

func (m Model) settingsStart(lines []settingLine, inner int) int {
	selAt := 0
	for i, l := range lines {
		if l.idx == m.setSel {
			selAt = i
		}
	}
	return max(0, min(selAt-inner/2, len(lines)-inner))
}

func (m Model) clickProblem(x, y int) (tea.Model, tea.Cmd) {
	m = m.startSelect(x, y)
	g, ok := m.splitGeom()
	if !ok {
		return m, nil
	}
	if x == g.lw-1 || x == g.lw { // the gap before the right column: drag to resize
		m.dragging = true
		return m, nil
	}
	if x < g.lw {
		m.pane = paneStatement
		return m, nil
	}
	ty := contentTop + g.infoH
	switch {
	case y < ty:
		m.showTags = !m.showTags
	case y >= ty && y < ty+g.testsH:
		m.pane = paneTests
		_, _, rowAt := m.testsLines(g.rw-4, g.testsH-2)
		if r := y - ty - 1; r >= 0 && r < len(rowAt) && rowAt[r] >= 0 {
			if rowAt[r] == m.tsel {
				if row, ok := m.selected(); ok && row.Res != nil && row.Res.Verdict != "AC" && m.run != nil {
					m.run.diff, m.run.diffInit = true, false // a second click opens the diff
				}
			}
			m.tsel, m.detScroll = rowAt[r], 0
		}
	case y >= ty+g.testsH:
		m.pane = paneDetail
	}
	return m, nil
}

func (m Model) clickInModal(rx, ry int) (tea.Model, tea.Cmd) {
	switch {
	case m.help && ry == 1:
		x := 1
		for i, p := range helpPages {
			w := len([]rune(p)) + 2
			if rx >= x && rx < x+w+2 {
				m.helpPage, m.helpScroll = i, 0
			}
			x += w + 1
		}
	case m.tm != nil && m.tm.ed == nil:
		lw := min(30, min(m.width-4, 100)/3)
		if rx > 0 && rx <= lw+1 && ry >= 1 {
			inner := min(m.height-4, 30) - 2
			start := max(0, min(m.tm.sel-inner/2, len(m.cases)-inner))
			if i := start + ry - 1; i < len(m.cases) {
				m.tm.sel, m.tsel, m.tm.confirmDel = i, i, false
			}
		}
	case m.fm != nil:
		f := m.fm
		switch {
		case ry == 1:
			f.field = ffMin
			if rx >= 26 {
				f.field = ffMax
			}
		case ry == 2:
			if f.field == ffStatus {
				m.filter.Status = statusChoice(nextOf(statusOpts, orAny(m.filter.Status)))
				return m.applyFilter(), nil
			}
			f.field = ffStatus
		case ry == 3:
			if f.field == ffSort {
				cur := m.sortBy
				if cur == "" {
					cur = "default"
				}
				if cur = nextOf(sortOpts, cur); cur == "default" {
					cur = ""
				}
				m.sortBy = cur
				return m.applyFilter(), nil
			}
			f.field = ffSort
		case ry >= 6:
			f.field = ffTags
			cols, cw, perCol, start := m.fmGrid()
			col := (rx - 3) / cw
			i := start + col*perCol + (ry - 6)
			if shown := f.shownTags(); col >= 0 && col < cols && i >= 0 && i < len(shown) {
				f.tsel = i
				return m.cycleTag(shown[i].name).applyFilter(), nil
			}
		}
	case m.tab == 1 && m.contestOpen != nil && m.open == nil:
		ps := m.contestProblems(*m.contestOpen)
		if i := ry - 1; i >= 0 && i < len(ps) {
			if i == m.cpCursor {
				return m.openProblem(ps[i])
			}
			m.cpCursor = i
		}
	}
	return m, nil
}

func (m Model) onWheel(x, y int, up bool) (tea.Model, tea.Cmd) {
	d := 3
	if up {
		d = -3
	}
	switch {
	case m.tm != nil && m.tm.ed == nil:
		m.tm.sel = max(0, min(m.tm.sel+d/3, len(m.cases)-1))
		m.tsel = m.tm.sel
		return m, nil
	case m.fm != nil:
		if m.fm.field == ffTags {
			m.fm.tsel = max(0, min(m.fm.tsel+d, len(m.fm.shownTags())-1))
		}
		return m, nil
	case m.run != nil && m.run.diff:
		m.run.diffOff = max(0, m.run.diffOff+d)
		return m, nil
	case m.strs != nil && m.strs.diff:
		m.strs.diffOff = max(0, m.strs.diffOff+d)
		return m, nil
	case m.help:
		m.helpScroll = max(0, m.helpScroll+d/3)
		return m, nil
	case m.subModal:
		return m, nil
	case m.tab == 1 && m.contestOpen != nil && m.open == nil:
		ps := m.contestProblems(*m.contestOpen)
		m.cpCursor = max(0, min(m.cpCursor+d/3, len(ps)-1))
		return m, nil
	}
	if m.open != nil {
		if g, ok := m.splitGeom(); ok && x >= g.lw {
			ty := contentTop + g.infoH
			switch {
			case y >= ty && y < ty+g.testsH:
				m.tsel = max(0, min(m.tsel+d/3, len(m.rows())-1))
				m.detScroll = 0
			case y >= ty+g.testsH:
				m.detScroll = max(0, m.detScroll+d)
			}
			return m, nil
		}
		m.scroll = max(0, min(m.scroll+d, m.maxScroll()))
		return m, nil
	}
	switch m.tab {
	case 0:
		m.cursor = max(0, min(m.cursor+d, len(m.visible)-1))
	case 1:
		m.contestCursor = max(0, min(m.contestCursor+d, len(m.contests)-1))
	case 2:
		m.statsScroll = max(0, m.statsScroll+d)
	case 4:
		m.setSel = max(0, min(m.setSel+d/3, len(settingDefs)-1))
	}
	return m, nil
}
