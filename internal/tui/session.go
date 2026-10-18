package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/session"
	"github.com/moneytosms/verd/internal/store"
)

type sessTickMsg struct{}

func sessTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return sessTickMsg{} })
}

// isBlind reports whether p belongs to the running session's contest: its rating, tags, solved
// count and old marks stay hidden until the session ends.
func (m Model) isBlind(p cf.Problem) bool {
	return m.sess != nil && p.ContestID == m.sess.ContestID && p.Source() == cf.SourceCF
}

func (m Model) solvedText(p cf.Problem) string {
	if m.isBlind(p) {
		return fmt.Sprintf(" %7s  ", "?")
	}
	return fmt.Sprintf(" %7d  ", p.SolvedCount)
}

// applySession scores the running session and hides what a contest would hide: it blanks the
// session contest's ratings, tags and solved counts and replaces its marks with the session's own.
func (m Model) applySession(d Data) Model {
	m.sess, m.sessResult = d.Session, session.Result{}
	if m.sess == nil {
		return m
	}
	var idx []string
	ps := make([]cf.Problem, len(d.Problems))
	copy(ps, d.Problems)
	for i, p := range ps {
		if m.isBlind(p) {
			idx = append(idx, p.Index)
			ps[i].Rating, ps[i].Tags, ps[i].SolvedCount = 0, nil, 0
		}
	}
	sort.Strings(idx)
	m.sessResult = session.Score(*m.sess, idx, d.Subs)
	st := make(map[string]store.Status, len(d.Statuses))
	for k, v := range d.Statuses {
		st[k] = v
	}
	for _, r := range m.sessResult.Rows {
		key := fmt.Sprintf("%d%s", m.sess.ContestID, r.Index)
		switch {
		case r.Solved:
			st[key] = store.StatusSolved
		case r.Wrong > 0:
			st[key] = store.StatusAttempted
		default:
			delete(st, key)
		}
	}
	m.Problems, m.status = ps, st
	return m
}

// clockText formats a duration as h:mm:ss.
func clockText(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

// sessLine is the one-line status of the running session: mode, clock and score.
func (m Model) sessLine() string {
	s, now := m.sess, m.clock()
	clock := clockText(s.Elapsed(now)) + " elapsed"
	if s.End > 0 {
		clock = clockText(s.Left(now)) + " left"
	}
	line := fmt.Sprintf("%s  %s  solved %d/%d", s.Mode, clock, m.sessResult.Solved, len(m.sessResult.Rows))
	if s.Mode == session.Virtual {
		line += fmt.Sprintf("  penalty %d", m.sessResult.Penalty)
	}
	return line
}

func (m Model) sessionDeadline() time.Duration {
	if m.sess == nil || m.sess.End == 0 {
		return 0
	}
	return time.Unix(m.sess.End, 0).Sub(time.Unix(m.sess.Start, 0))
}

// startSession begins a session after a second press of the same key confirms it.
func (m Model) startSession(c cf.Contest, mode string) (Model, tea.Cmd) {
	switch {
	case m.deps.StartSession == nil:
		return m, nil
	case m.sess != nil && m.sess.ContestID == c.ID:
		m.contestNote = "this contest's session is already running"
		return m, nil
	case m.sess != nil:
		m.contestNote = fmt.Sprintf("end the running session first (open contest %d and press {contest.end_session})", m.sess.ContestID)
		m.contestNote = m.kx(m.contestNote)
		return m, nil
	case !c.Finished():
		m.contestNote = "sessions replay finished contests; joining a live contest comes later"
		return m, nil
	case len(m.contestProblems(c)) == 0:
		m.contestNote = ""
		nm, cmd := m.fetchContest()
		m = nm.(Model)
		if cmd != nil {
			m.contestNote = "fetching Problems: press the key again once they are listed"
		}
		return m, cmd
	}
	if m.sessConfirm != mode {
		m.sessConfirm = mode
		what := "a virtual contest of " + clockText(time.Duration(c.Duration)*time.Second)
		if mode == session.Practice {
			what = "practice with no timer"
		}
		m.contestNote = m.kx("ratings and tags will be hidden. Press the same key again to start " + what + ", {contest.close} to cancel")
		return m, nil
	}
	m.sessConfirm = ""
	s, err := m.deps.StartSession(c, mode)
	if err != nil {
		m.contestNote = "session: " + clean(err.Error())
		return m, nil
	}
	m.raw.Session = &s
	m = m.WithData(m.raw)
	m.contestNote, m.cpCursor = "", 0
	return m, sessTick()
}

// endSession forgets the session and shows everything again.
func (m Model) endSession() Model {
	if m.deps.EndSession != nil {
		if err := m.deps.EndSession(); err != nil {
			m.errMsg = "end session: " + clean(err.Error())
			return m
		}
	}
	m.raw.Session, m.sessSum = nil, false
	return m.WithData(m.raw)
}

func (m Model) onSessTick() (tea.Model, tea.Cmd) {
	if m.sess == nil {
		return m, nil
	}
	if m.sess.Over(m.clock()) {
		m.sessSum = true // time is up: show the summary and stop the clock
		return m, nil
	}
	return m, sessTick()
}

func (m Model) updateSessSummary(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.sessSum = false
		if m.sess != nil && m.sess.Over(m.clock()) {
			return m, nil
		}
		return m, sessTick()
	case "e", "enter":
		return m.endSession(), nil
	}
	return m, nil
}

// sessSummaryBox is the end-of-session summary.
func (m Model) sessSummaryBox() []string {
	st := m.styles()
	s := m.sess
	w, _ := m.modalSize(70, 24)
	over := s.Over(m.clock())
	head := "running"
	if over {
		head = "time is up"
	}
	body := []string{st.Accent.Render(clean(s.Name)), st.Dim.Render(s.Mode + ", " + head), ""}
	var upsolve []string
	for _, r := range m.sessResult.Rows {
		switch {
		case r.Solved:
			body = append(body, fmt.Sprintf("  %s %-3s %s", st.Good.Render("✓"), r.Index, m.resultCell(r)))
		default:
			upsolve = append(upsolve, r.Index)
			body = append(body, fmt.Sprintf("  %s %-3s %s", st.Dim.Render("·"), r.Index, m.resultCell(r)))
		}
	}
	body = append(body, "", fmt.Sprintf("solved %d/%d", m.sessResult.Solved, len(m.sessResult.Rows)))
	if s.Mode == session.Virtual {
		body = append(body, fmt.Sprintf("penalty %d", m.sessResult.Penalty))
	}
	if len(upsolve) > 0 {
		body = append(body, "", st.Dim.Render("upsolve: "+strings.Join(upsolve, " ")))
	}
	return box("Session summary", body, m.kx("{contest.end_session}/enter end the session  esc keep it"), w, len(body)+2, true, st, m.borderStyle())
}

// resultCell is "12m +1" for a solve (minute, wrong attempts before it) or "-2" for attempts so far.
func (m Model) resultCell(r session.Row) string {
	st := m.styles()
	switch {
	case r.Solved && r.Wrong > 0:
		return st.Good.Render(fmt.Sprintf("%dm", r.Minute)) + st.Bad.Render(fmt.Sprintf(" +%d", r.Wrong))
	case r.Solved:
		return st.Good.Render(fmt.Sprintf("%dm", r.Minute))
	case r.Wrong > 0:
		return st.Bad.Render(fmt.Sprintf("-%d", r.Wrong))
	}
	return ""
}

// contestResult is the session cell for a problem of the session's contest.
func (m Model) contestResult(p cf.Problem) string {
	for _, r := range m.sessResult.Rows {
		if r.Index == p.Index {
			return m.resultCell(r)
		}
	}
	return ""
}
