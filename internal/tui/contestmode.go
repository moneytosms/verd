package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/session"
	"github.com/moneytosms/verd/internal/store"
)

// Contest mode is the screen while a session runs, whatever its kind: the session's Problems on a
// rail at the left, the open one in the Problem view, and no tabs. It ends with the session.

// Rail modes, cycled by contest_mode.rail. Auto is compact next to an embedded editor or in a
// narrow window, full otherwise.
const (
	railAuto = iota
	railFull
	railCompact
	railOff
)

const railFullW, railCompactW = 26, 8

// cmState is contest mode's own state.
type cmState struct {
	rail   int
	mem    map[string]cmMem // per-Problem view state, by index
	stats  session.Stats
	ticks  int    // seconds since the stats were last saved
	fetch  bool   // the session's statements were prefetched
	shown  bool   // a session Problem has been open: a Problem opened from outside then stays open
	report string // where the ended session's report went, so a retried end does not write it again
}

// cmMem is a Problem's view as it was left: switching back restores it.
type cmMem struct {
	scroll, tsel, pane int
	run                *testRun
}

func (m Model) cmOn() bool { return m.sess != nil }

// cmProblems are the session contest's Problems in index order.
func (m Model) cmProblems() []cf.Problem {
	if m.sess == nil {
		return nil
	}
	return m.contestProblems(cf.Contest{ID: m.sess.ContestID})
}

// cmCurrent is the position of the open Problem in cmProblems, -1 if it is not one of them.
func (m Model) cmCurrent() int {
	for i, p := range m.cmProblems() {
		if m.open != nil && p.ContestID == m.open.ContestID && p.Index == m.open.Index {
			return i
		}
	}
	return -1
}

// cmEnsure keeps contest mode showing a session Problem: the first one when nothing of the session
// is open (start, resume, Problems just arrived). It also prefetches every statement once.
func (m Model) cmEnsure() (Model, tea.Cmd) {
	ps := m.cmProblems()
	if !m.cmOn() || len(ps) == 0 {
		return m, nil
	}
	var cmds []tea.Cmd
	if !m.cm.fetch && m.deps.Load != nil {
		m.cm.fetch = true
		load := m.deps.Load
		for _, p := range ps {
			cmds = append(cmds, func() tea.Msg { load(p, false); return nil })
		}
	}
	m.tab, m.openTab = 0, 0
	switch {
	case m.cmCurrent() >= 0:
		m.cm.shown = true
	case !m.cm.shown || m.open == nil: // before the session's first Problem, any open one gives way
		nm, cmd := m.cmSwitch(ps[0])
		m, cmds = nm, append(cmds, cmd)
		m.cm.shown = true
	}
	return m, tea.Batch(cmds...)
}

// cmSwitch opens p in place of the current Problem, keeping the current one's view to come back to
// and moving an open split-pane editor along.
func (m Model) cmSwitch(p cf.Problem) (Model, tea.Cmd) {
	if m.cm.mem == nil {
		m.cm.mem = map[string]cmMem{}
	}
	if m.open != nil && m.cmCurrent() >= 0 {
		mem := cmMem{scroll: m.scroll, tsel: m.tsel, pane: m.pane}
		if m.run != nil && !m.run.running {
			mem.run = m.run
		}
		m.cm.mem[m.open.Index] = mem
	}
	m.tab, m.openTab = 0, 0
	nm, cmd := m.openProblem(p)
	m = nm.(Model)
	mem := m.cm.mem[p.Index]
	m.run, m.tsel, m.pane = mem.run, mem.tsel, mem.pane
	m = m.cmSave()
	if m.editorOpen && m.deps.Edit != nil { // the open editor pane follows
		c, err := m.deps.Edit(p, m.lang())
		nm, ecmd := m.afterOpen(c, err, "edit")
		return nm.(Model), tea.Batch(cmd, ecmd)
	}
	return m, cmd
}

// cmRestoreScroll puts back the scroll of a Problem switched back to, once its statement is laid out.
func (m Model) cmRestoreScroll() Model {
	if m.cmOn() && m.open != nil {
		if mem, ok := m.cm.mem[m.open.Index]; ok {
			m.scroll = mem.scroll
		}
	}
	return m
}

// cmTick counts a second on the open Problem and notes when it was first opened (also after a
// restart, where the Problem never went through cmSwitch); the stats are saved every half minute.
func (m Model) cmTick() Model {
	if i := m.cmCurrent(); i >= 0 && m.sess.Started(m.clock()) && !m.sess.Over(m.clock()) {
		if m.cm.stats == nil {
			m.cm.stats = session.Stats{}
		}
		ps := m.cm.stats.Of(m.open.Index)
		if ps.FirstOpen == 0 {
			ps.FirstOpen = m.clock().Unix()
		}
		ps.Seconds++
	}
	if m.cm.ticks++; m.cm.ticks >= 30 {
		m = m.cmSave()
	}
	return m
}

// cmCountRun records a finished Test Run of a session Problem.
func (m Model) cmCountRun(verdict string) Model {
	if m.cmCurrent() < 0 || !m.sess.Started(m.clock()) || m.sess.Over(m.clock()) {
		return m
	}
	if m.cm.stats == nil {
		m.cm.stats = session.Stats{}
	}
	ps := m.cm.stats.Of(m.open.Index)
	ps.Runs++
	if verdict == "AC" {
		ps.Green++
	}
	return m.cmSave()
}

// FlushStats saves contest mode's stats now, for quitting between two saves.
func (m Model) FlushStats() { m.cmSave() }

func (m Model) cmSave() Model {
	m.cm.ticks = 0
	if m.deps.SaveSessionStats != nil && m.cm.stats != nil {
		if err := m.deps.SaveSessionStats(m.cm.stats); err != nil {
			m.Notice = "session stats: " + clean(err.Error())
		}
	}
	return m
}

// cmKey handles contest mode's keys, which work even while the embedded editor has the keyboard.
func (m Model) cmKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	k := msg.String()
	if m.km != nil {
		canon, drop := m.km.translate("contest_mode", k)
		if drop {
			return m, nil, false
		}
		k = canon
	}
	ps, cur := m.cmProblems(), m.cmCurrent()
	switch {
	case k == "alt+n" || k == "alt+p":
		if len(ps) == 0 {
			return m, nil, true
		}
		d := 1
		if k == "alt+p" {
			d = len(ps) - 1
		}
		next := 0 // no session Problem open: next is A, previous the last
		if cur >= 0 {
			next = (cur + d) % len(ps)
		} else if k == "alt+p" {
			next = len(ps) - 1
		}
		nm, cmd := m.cmSwitch(ps[next])
		return nm, cmd, true
	case len(k) == 5 && strings.HasPrefix(k, "alt+") && k[4] >= '1' && k[4] <= '9':
		if i := int(k[4] - '1'); i < len(ps) && i != cur {
			nm, cmd := m.cmSwitch(ps[i])
			return nm, cmd, true
		}
		return m, nil, true
	case k == "alt+b":
		m.cm.rail = map[int]int{railFull: railCompact, railCompact: railOff, railOff: railFull}[m.railEffective(m.verdWidth())]
		return m, m.relayout(), true
	case k == "alt+x":
		m.sessSum = true
		return m, nil, true
	}
	return m, nil, false
}

// railEffective is the rail mode in force for a verd column w wide (auto resolved).
func (m Model) railEffective(w int) int {
	switch {
	case !m.cmOn() || m.zen:
		return railOff
	case m.cm.rail != railAuto:
		return m.cm.rail
	case w < 60:
		return railOff
	case m.embed.term != nil || w < 140:
		return railCompact
	}
	return railFull
}

// railCols is the rail's width in a verd column w wide, 0 when hidden.
func (m Model) railCols(w int) int {
	switch m.railEffective(w) {
	case railFull:
		if w-railFullW-1 >= 40 {
			return railFullW
		}
		fallthrough
	case railCompact:
		if w-railCompactW-1 >= 30 {
			return railCompactW
		}
	}
	return 0
}

// verdWidth is verd's column: the window less an embedded editor.
func (m Model) verdWidth() int {
	if m.embed.term != nil {
		if left, right := m.layout(); right > 0 {
			return left
		}
	}
	return m.width
}

// railLines draws the rail, w wide and h tall.
func (m Model) railLines(w, h int) []string {
	st := m.styles()
	s, now := m.sess, m.clock()
	clock := clockText(s.Elapsed(now))
	switch {
	case !s.Started(now):
		clock = "-" + clockText(s.UntilStart(now))
	case s.End > 0:
		clock = clockText(s.Left(now))
	}
	var out []string
	add := func(l string) { out = append(out, l) }
	rule := st.Dim.Render(strings.Repeat("─", w))
	ps, cur := m.cmProblems(), m.cmCurrent()
	compact := w < railFullW
	if compact {
		add(st.Warn.Render(clock))
	} else {
		add(st.Accent.Render(fit(clean(s.Name), w)))
		add(st.Warn.Render(s.Mode + "  " + clock))
		add(fit(m.sessScore(), w))
	}
	add(rule)
	for i, p := range ps {
		mark := " "
		if i == cur {
			mark = st.Accent.Render("▸")
		}
		idx := p.Index
		if i == cur {
			idx = st.Accent.Bold(true).Render(idx)
		}
		cell := m.contestResult(p)
		if compact {
			cell = m.railGlyph(p)
		}
		add(mark + idx + " " + cell)
	}
	if len(ps) == 0 {
		add(st.Dim.Render(fit("waiting for problems", w)))
	}
	if sd := m.standing; sd != nil && s.Mode == session.Live && sd.Row != nil {
		add(rule)
		if compact {
			add(st.Warn.Render(fmt.Sprintf("#%d", sd.Row.Rank)))
		} else {
			add(st.Warn.Render(fmt.Sprintf("rank %d/%d", sd.Row.Rank, sd.Total)))
			last := 0
			for _, rows := range [][]cf.StandingRow{sd.Top, sd.Near} {
				for _, r := range rows {
					if r.Rank <= last {
						continue // the top rows and the neighbours can overlap
					}
					if last > 0 && r.Rank > last+1 {
						add(st.Dim.Render("  ⋯"))
					}
					last = r.Rank
					l := fit(fmt.Sprintf("%5d %s", r.Rank, clean(r.Handle)), w)
					if r.Rank == sd.Row.Rank && r.Handle == sd.Row.Handle {
						l = st.Accent.Bold(true).Render(l)
					}
					add(l)
				}
			}
		}
	}
	if !compact {
		hints := []string{"{contest_mode.next}/{contest_mode.prev} problem", "{contest_mode.jump_1}-9 jump", "{contest_mode.rail} rail", "{contest_mode.exit} end"}
		if len(out)+len(hints)+1 < h {
			for len(out)+len(hints) < h {
				add("")
			}
			for _, l := range hints {
				add(st.Dim.Render(fit(m.kx(l), w)))
			}
		}
	}
	return out
}

// railGlyph is a compact result: ✓ solved, -n wrong attempts.
func (m Model) railGlyph(p cf.Problem) string {
	st := m.styles()
	if status, ok := m.liveStatus(p); ok && status == store.StatusSolved {
		return st.Good.Render("✓")
	}
	for _, r := range m.sessResult.Rows {
		switch {
		case r.Index != p.Index:
		case r.Solved:
			return st.Good.Render("✓")
		case r.Wrong > 0:
			return st.Bad.Render(fmt.Sprintf("-%d", r.Wrong))
		}
	}
	return ""
}

// withRail draws the rail left of main, a verd column w wide: main is rendered rw+1 narrower.
func (m Model) withRail(rw int, main func(Model) string) string {
	mm := m
	mm.width, mm.inner = m.width-rw-1, m.width-rw-1
	body := strings.Split(main(mm), "\n")
	rail := m.railLines(rw, m.height)
	div := m.styles().Dim.Render("│")
	lines := make([]string, max(1, m.height))
	for i := range lines {
		l, r := "", ""
		if i < len(rail) {
			l = xansi.Truncate(rail[i], rw, "")
		}
		if i < len(body) {
			r = body[i]
		}
		lines[i] = l + strings.Repeat(" ", max(0, rw-xansi.StringWidth(l))) + div + r
	}
	return strings.Join(lines, "\n")
}

// railMouse handles a click on the rail (jump to that Problem) and moves other mouse events into
// the main area's coordinates. msg is in verd-column coordinates.
func (m Model) railMouse(msg tea.Msg) (Model, tea.Msg, tea.Cmd, bool) {
	mm, ok := msg.(tea.MouseMsg)
	rw := m.railCols(m.verdWidth())
	if !ok || rw == 0 || m.modalBox() != nil { // a modal is drawn over the rail too
		return m, msg, nil, false
	}
	rel := mm.Mouse()
	if rel.X > rw {
		rel.X -= rw + 1
		return m, translateMouse(mm, rel), nil, false
	}
	if c, ok := msg.(tea.MouseClickMsg); ok && c.Button == tea.MouseLeft {
		top := 4 // name, mode and clock, score, rule
		if rw < railFullW {
			top = 2 // clock, rule
		}
		if i, ps := rel.Y-top, m.cmProblems(); i >= 0 && i < len(ps) {
			nm, cmd := m.cmSwitch(ps[i])
			return nm, nil, cmd, true
		}
	}
	return m, nil, nil, true
}

// cmHeader replaces the tab bar in contest mode: the contest and its clock.
func (m Model) cmHeader(w int) string {
	st := m.styles()
	left := st.PillOn.Render(logo) + " " + st.PillOn.Render(" contest ") + " " + st.Accent.Render(clean(m.sess.Name))
	right := st.Warn.Render(m.sessLine())
	gap := w - xansi.StringWidth(left) - xansi.StringWidth(right) - 1
	if gap < 1 {
		right, gap = "", 1
	}
	return paintRow(left+strings.Repeat(" ", gap)+right, w, st.Bar)
}

// cmProblemsForReport lists the session's Problems for session.Report.
func (m Model) cmProblemsForReport() []session.Problem {
	var out []session.Problem
	for _, p := range m.cmProblems() {
		out = append(out, session.Problem{Index: p.Index, Name: p.Name, URL: p.URL()})
	}
	return out
}
