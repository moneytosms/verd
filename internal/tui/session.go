package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
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
		m.cm = cmState{rail: m.cm.rail}
		return m
	}
	if m.cm.stats == nil {
		m.cm.stats = d.SessionStats
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
	switch {
	case s.Mode == session.Live && !s.Started(now):
		return fmt.Sprintf("live  starts in %s", clockText(s.UntilStart(now)))
	case s.End > 0:
		return fmt.Sprintf("%s  %s left  %s", s.Mode, clockText(s.Left(now)), m.sessScore())
	}
	return fmt.Sprintf("%s  %s elapsed  %s", s.Mode, clockText(s.Elapsed(now)), m.sessScore())
}

// liveRow is the Codeforces standings row of a live session, when it has arrived and the handle took part.
func (m Model) liveRow() *cf.StandingRow {
	if m.sess == nil || m.sess.Mode != session.Live || m.standing == nil {
		return nil
	}
	return m.standing.Row
}

// sessScore is the score text: the Codeforces standings row for a live session, else the local score.
func (m Model) sessScore() string {
	if m.sess.Mode == session.Live && m.standing != nil {
		return liveScore(m.standing)
	}
	line := fmt.Sprintf("solved %d/%d", m.sessResult.Solved, len(m.sessResult.Rows))
	if m.sess.Mode != session.Practice {
		line += fmt.Sprintf("  penalty %d", m.sessResult.Penalty)
	}
	return line
}

// liveScore is the score text of a live session, from the Codeforces standings.
func liveScore(s *cf.Standing) string {
	r := s.Row
	switch {
	case r == nil:
		return "not in the standings (register on the site)"
	case s.Contest.Type == "ICPC":
		solved := 0
		for _, pr := range r.Results {
			if pr.Solved() {
				solved++
			}
		}
		return fmt.Sprintf("rank %d  solved %d  penalty %d", r.Rank, solved, r.Penalty)
	}
	return fmt.Sprintf("rank %d  %.0f pts", r.Rank, r.Points)
}

// startSession begins a session after a second press of the same key confirms it. A finished
// contest is replayed (virtual or practice); one that is running or upcoming is joined live.
func (m Model) startSession(c cf.Contest, mode string) (Model, tea.Cmd) {
	live := !c.Finished()
	if nm, cmd, refused := m.sessPrecheck(c, mode, live); refused {
		return nm, cmd
	}
	if live {
		mode = session.Live
	}
	if m.sessConfirm != mode {
		m.sessConfirm = mode
		m.contestNote = m.kx("ratings and tags will be hidden. Press the same key again to start " + sessWhat(c, mode, live) + ", {contest.close} to cancel")
		return m, nil
	}
	m.sessConfirm = ""
	s, err := m.deps.StartSession(c, mode)
	if err != nil {
		m.contestNote = "session: " + clean(err.Error())
		return m, nil
	}
	m.raw.Session = &s
	m.standing, m.standAt, m.standErr = nil, time.Time{}, ""
	m = m.WithData(m.raw)
	m.contestNote, m.cpCursor = "", 0
	m.cm = cmState{rail: m.cm.rail, stats: session.Stats{}}
	m, cmd := m.cmEnsure()
	return m, tea.Batch(sessTick(), cmd)
}

// sessPrecheck refuses a session start that cannot run, with a note saying why. refused is false
// when the start may go ahead.
func (m Model) sessPrecheck(c cf.Contest, mode string, live bool) (Model, tea.Cmd, bool) {
	switch {
	case m.deps.StartSession == nil:
		return m, nil, true
	case m.sess != nil && m.sess.ContestID == c.ID:
		m.contestNote = "this contest's session is already running"
		return m, nil, true
	case m.sess != nil:
		m.contestNote = m.kx(fmt.Sprintf("end the running session first (open contest %d and press {contest.end_session})", m.sess.ContestID))
		return m, nil, true
	case live && mode == session.Practice:
		m.contestNote = "practice replays a finished contest"
		return m, nil, true
	case live && m.handle == "":
		m.contestNote = "set your handle first: live results come from the standings"
		return m, nil, true
	case !live && len(m.contestProblems(c)) == 0:
		m.contestNote = ""
		nm, cmd := m.fetchContest()
		m = nm.(Model)
		if cmd != nil {
			m.contestNote = "fetching Problems: press the key again once they are listed"
		}
		return m, cmd, true
	}
	return m, nil, false
}

// sessWhat names the session being confirmed, for the confirm note.
func sessWhat(c cf.Contest, mode string, live bool) string {
	switch {
	case mode == session.Practice:
		return "practice with no timer"
	case !live:
		return "a virtual contest of " + clockText(time.Duration(c.Duration)*time.Second)
	case c.Phase == "BEFORE":
		return "waiting for the start (register on the site first)"
	}
	return "the live contest: results come from the standings every minute (register on the site first)"
}

// endSession saves the session's report, forgets the session and shows everything again.
func (m Model) endSession() Model {
	m = m.cmSave()
	note := ""
	if m.cm.report == "" && m.deps.SaveReport != nil && m.sess != nil { // once: a retry after a failed end must not write again
		md := session.Report(*m.sess, m.cmProblemsForReport(), m.sessResult, m.cm.stats, m.sessScore(), m.clock())
		if path, err := m.deps.SaveReport(*m.sess, md); err != nil {
			note = "report not saved: " + clean(err.Error())
		} else {
			m.cm.report = path
		}
	}
	if m.cm.report != "" {
		note = "session report saved to " + m.cm.report
	}
	if m.deps.EndSession != nil {
		if err := m.deps.EndSession(); err != nil {
			m.errMsg = "end session: " + clean(err.Error())
			return m
		}
	}
	m.raw.Session, m.sessSum, m.standing = nil, false, nil
	m = m.WithData(m.raw)
	m.Notice = note
	return m
}

type standingMsg struct {
	st  *cf.Standing
	err error
	at  time.Time
}

// standEvery is how often the live standings are downloaded: they are the whole table, many MB for a big round.
func (m Model) standEvery() time.Duration {
	if m.standing != nil && m.standing.Bytes > 8<<20 {
		return 2 * time.Minute
	}
	return time.Minute
}

func (m Model) pollStanding() (Model, tea.Cmd) {
	m.standBusy = true
	fetch, id, at := m.deps.Standings, m.sess.ContestID, m.clock()
	return m, func() tea.Msg {
		st, err := fetch(id)
		return standingMsg{st, err, at}
	}
}

func (m Model) onStanding(msg standingMsg) Model {
	m.standBusy, m.standAt = false, msg.at
	if msg.err != nil {
		m.standErr = clean(msg.err.Error())
		return m
	}
	m.standing, m.standErr = msg.st, ""
	return m
}

// liveTick drives a live session: fetch the Problems once the contest starts, then poll the standings.
func (m Model) liveTick() (Model, tea.Cmd) {
	now := m.clock()
	if !m.sess.Started(now) {
		return m, nil
	}
	var cmds []tea.Cmd
	c := cf.Contest{ID: m.sess.ContestID}
	if len(m.contestProblems(c)) == 0 && !m.contestBusy && m.deps.FetchContest != nil && now.Sub(m.sessFetchAt) >= 5*time.Second {
		m.contestBusy, m.sessFetchAt = true, now
		fetch, id := m.deps.FetchContest, c.ID
		cmds = append(cmds, func() tea.Msg { return contestFetchedMsg{fetch(id)} })
	}
	if m.deps.Standings != nil && !m.standBusy && (m.standAt.IsZero() || now.Sub(m.standAt) >= m.standEvery()) {
		var cmd tea.Cmd
		m, cmd = m.pollStanding()
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) onSessTick() (tea.Model, tea.Cmd) {
	if m.sess == nil {
		return m, nil
	}
	if m.sess.Over(m.clock()) {
		m.sessSum = true // time is up: show the summary and stop the clock
		if m.sess.Mode == session.Live && m.deps.Standings != nil && !m.standBusy {
			return m.pollStanding() // one last look at the standings
		}
		return m, nil
	}
	m = m.cmTick()
	m, ecmd := m.cmEnsure()
	if m.sess.Mode == session.Live {
		nm, cmd := m.liveTick()
		return nm, tea.Batch(cmd, ecmd, sessTick())
	}
	return m, tea.Batch(ecmd, sessTick())
}

func (m Model) updateSessSummary(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.sessSum = false // the tick chain kept running under the summary
		return m, nil
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
	body := []string{st.Accent.Render(clean(s.Name)), st.Dim.Render(s.Mode + ", " + head), "",
		st.Dim.Render(fmt.Sprintf("      %-10s %8s  %s", "result", "time", "test runs"))}
	var upsolve []string
	var spent int64
	for _, r := range m.sessResult.Rows {
		mark := st.Good.Render("✓")
		if !r.Solved {
			mark = st.Dim.Render("·")
			upsolve = append(upsolve, r.Index)
		}
		ps := m.cm.stats[r.Index]
		if ps == nil {
			ps = &session.ProblemStats{}
		}
		spent += ps.Seconds
		cell := m.resultCell(r)
		cell += strings.Repeat(" ", max(0, 10-xansi.StringWidth(cell)))
		body = append(body, fmt.Sprintf("  %s %-3s%s %8s  %d (%d passing)", mark, r.Index, cell, clockText(time.Duration(ps.Seconds)*time.Second), ps.Runs, ps.Green))
	}
	body = append(body, "", st.Dim.Render("time on problems "+clockText(time.Duration(spent)*time.Second)))
	body = append(body, "", m.sessScore())
	if row := m.liveRow(); row != nil {
		body = append(body, st.Dim.Render("results can change after system testing"))
	}
	if m.standErr != "" {
		body = append(body, st.Bad.Render(fit(m.standErr, w-6)))
	}
	if len(upsolve) > 0 {
		body = append(body, "", st.Dim.Render("upsolve: "+strings.Join(upsolve, " ")))
	}
	return box("Session summary", body, m.kx("{contest.end_session}/enter end the session and save its report  esc keep it"), w, len(body)+2, true, st, m.borderStyle())
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
	if row := m.liveRow(); row != nil {
		for i, sp := range m.standing.Problems {
			if sp.Index == p.Index && i < len(row.Results) {
				return m.liveCell(row.Results[i], m.standing.Contest.Type)
			}
		}
	}
	for _, r := range m.sessResult.Rows {
		if r.Index == p.Index {
			return m.resultCell(r)
		}
	}
	return ""
}

// liveCell is a standings result: points (or the solve minute for ICPC) and wrong attempts; a star
// marks a result that system testing has not confirmed yet.
func (m Model) liveCell(r cf.ProblemResult, typ string) string {
	st := m.styles()
	star := ""
	if r.Type == "PRELIMINARY" {
		star = "*"
	}
	switch {
	case r.Solved():
		cell := fmt.Sprintf("%.0f%s", r.Points, star)
		if typ == "ICPC" && r.BestSecs != nil {
			cell = fmt.Sprintf("%dm%s", *r.BestSecs/60, star)
		}
		cell = st.Good.Render(cell)
		if r.Rejected > 0 {
			cell += st.Bad.Render(fmt.Sprintf(" +%d", r.Rejected))
		}
		return cell
	case r.Rejected > 0:
		return st.Bad.Render(fmt.Sprintf("-%d", r.Rejected))
	}
	return ""
}

// liveStatus is the mark of a problem in a live session, from the standings row.
func (m Model) liveStatus(p cf.Problem) (store.Status, bool) {
	row := m.liveRow()
	if row == nil || !m.isBlind(p) {
		return 0, false
	}
	for i, sp := range m.standing.Problems {
		if sp.Index == p.Index && i < len(row.Results) {
			switch r := row.Results[i]; {
			case r.Solved():
				return store.StatusSolved, true
			case r.Rejected > 0:
				return store.StatusAttempted, true
			}
			return store.StatusNone, true
		}
	}
	return 0, false
}
