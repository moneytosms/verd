package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
)

// State is where the user left off: saved on quit and restored on the next start (resume).
type State struct {
	Tab           int
	Filter        Filter
	FilterExpr    string
	SortBy        string
	Cursor        int
	Contest       int // the open contest's id, 0 = none
	ContestCursor int
	CPCursor      int
	OpenContest   int // the open Problem, if any
	OpenIndex     string
	OpenTab       int
	Rail          int // contest mode's rail (railAuto, ...)
}

// resumeMsg reopens the Problem that was open at the last quit.
type resumeMsg struct{ p cf.Problem }

func (m Model) State() State {
	s := State{Tab: m.tab, Filter: m.filter, FilterExpr: m.filterExpr, SortBy: m.sortBy, Cursor: m.cursor,
		ContestCursor: m.contestCursor, CPCursor: m.cpCursor, OpenTab: m.openTab, Rail: m.cm.rail}
	if m.contestOpen != nil {
		s.Contest = m.contestOpen.ID
	}
	if m.open != nil {
		s.OpenContest, s.OpenIndex = m.open.ContestID, m.open.Index
	}
	return s
}

// WithState restores a saved State; anything that no longer exists (a hidden tab, an uncached
// contest or Problem) is skipped. Call it after WithData.
func (m Model) WithState(s State) Model {
	m.filter, m.filterExpr, m.sortBy, m.cursor = s.Filter, s.FilterExpr, s.SortBy, s.Cursor
	m.cm.rail = min(max(s.Rail, railAuto), railOff)
	if m.filter.Source != "" && !m.sourceOn(m.filter.Source) {
		m.filter.Source = ""
	}
	m = m.refilter()
	if s.Tab >= 0 && s.Tab < len(tabs) && slices.Contains(m.visibleTabs(), tabs[s.Tab]) {
		m.tab = s.Tab
	}
	m.contestCursor = min(max(0, s.ContestCursor), max(0, len(m.contests)-1))
	for _, c := range m.contests {
		if c.ID == s.Contest {
			m.contestOpen, m.cpCursor = &c, max(0, s.CPCursor)
			break
		}
	}
	for _, p := range m.Problems {
		if s.OpenIndex != "" && p.ContestID == s.OpenContest && p.Index == s.OpenIndex {
			m.resume, m.openTab = &p, s.OpenTab
			break
		}
	}
	return m.onTab()
}

// reopen opens the resumed Problem on the tab it lived on, keeping the current tab.
func (m Model) reopen(p cf.Problem) (tea.Model, tea.Cmd) {
	tab := m.tab
	m.tab = m.openTab
	nm, cmd := m.openProblem(p)
	mm := nm.(Model)
	mm.tab = tab
	return mm, cmd
}
