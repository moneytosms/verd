package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/session"
	"github.com/moneytosms/verd/internal/store"
)

// WithContests sets the contest list: upcoming (soonest first) then past (newest first).
func (m Model) WithContests(cs []cf.Contest) Model {
	var up, past []cf.Contest
	for _, c := range cs {
		if c.Finished() {
			past = append(past, c)
		} else {
			up = append(up, c)
		}
	}
	sort.SliceStable(up, func(i, j int) bool { return up[i].Start < up[j].Start })
	sort.SliceStable(past, func(i, j int) bool { return past[i].Start > past[j].Start })
	m.contests, m.upcoming = append(up, past...), len(up)
	return m
}

func (m Model) clock() time.Time {
	if m.deps.Now != nil {
		return m.deps.Now()
	}
	return time.Now()
}

// contestProblems returns the cached Problems of a contest, in index order.
func (m Model) contestProblems(c cf.Contest) []cf.Problem {
	var out []cf.Problem
	for _, p := range m.Problems {
		if p.ContestID == c.ID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

type contestFetchedMsg struct{ err error }

// fetchContest downloads the open contest's Problems. Not offered before the contest starts: CF
// publishes nothing until then.
func (m Model) fetchContest() (tea.Model, tea.Cmd) {
	c := m.contestOpen
	if c == nil || m.deps.FetchContest == nil || m.contestBusy {
		return m, nil
	}
	if c.Phase == "BEFORE" {
		m.contestNote = "not started: Codeforces publishes Problems when the contest begins"
		return m, nil
	}
	m.contestBusy, m.contestNote = true, ""
	fetch, id := m.deps.FetchContest, c.ID
	return m, func() tea.Msg { return contestFetchedMsg{fetch(id)} }
}

func (m Model) updateContests(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.contestOpen != nil {
		return m.updateContestOpen(msg)
	}
	if m.conFind {
		return m.contestFilterKey(msg)
	}
	if m.conQ != "" && msg.String() == "esc" { // first esc clears the filter
		m.conQ = ""
		return m.keepContestVisible(), nil
	}
	return m.updateContestList(msg)
}

// updateContestOpen handles keys while one contest's Problem list is open.
func (m Model) updateContestOpen(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	ps := m.contestProblems(*m.contestOpen)
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.contestOpen, m.cpCursor, m.contestNote, m.sessConfirm = nil, 0, "", ""
	case "r", "ctrl+r":
		return m.fetchContest()
	case "s":
		return m.startSession(*m.contestOpen, session.Virtual)
	case "p":
		return m.startSession(*m.contestOpen, session.Practice)
	case "e":
		m = m.contestSummary()
	case "H", "enter":
		return m.openContestProblem(ps, msg.String() == "H")
	case "up", "k":
		m.cpCursor = max(0, m.cpCursor-1)
	case "down", "j":
		m.cpCursor = min(len(ps)-1, m.cpCursor+1)
	}
	return m, nil
}

// contestSummary shows the session summary when the open contest is the running session's.
func (m Model) contestSummary() Model {
	if m.sess != nil && m.sess.ContestID == m.contestOpen.ID {
		m.sessSum = true
	}
	return m
}

// openContestProblem opens the cursor's Problem, or its editorial when editorial. None when the
// contest has no cached Problems.
func (m Model) openContestProblem(ps []cf.Problem, editorial bool) (tea.Model, tea.Cmd) {
	if len(ps) == 0 {
		return m, nil
	}
	if editorial {
		return m.openEditorial(ps[m.cpCursor])
	}
	return m.openProblem(ps[m.cpCursor])
}

// updateContestList handles keys on the contest list.
func (m Model) updateContestList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "/":
		m.conFind = true
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m = m.contestMove(-1)
	case "down", "j":
		m = m.contestMove(1)
	case "pgup":
		m = m.contestMove(-m.page())
	case "pgdown":
		m = m.contestMove(m.page())
	case "a", "u", "p", "r", "s":
		m = m.setChip(msg.String())
	case "o":
		m = m.toggleContestSort()
	case "enter":
		return m.openContest()
	}
	return m, nil
}

// toggleContestSort turns the least solved first sort on or off.
func (m Model) toggleContestSort() Model {
	if m.conSort == "" {
		m.conSort = "least"
	} else {
		m.conSort = ""
	}
	return m.keepContestVisible()
}

// openContest opens the highlighted contest's Problem list, fetching its Problems when none are cached.
func (m Model) openContest() (tea.Model, tea.Cmd) {
	if !slices.Contains(m.contestIdxs(), m.contestCursor) {
		return m, nil
	}
	c := m.contests[m.contestCursor]
	m.contestOpen, m.cpCursor, m.contestNote = &c, 0, ""
	if len(m.contestProblems(c)) == 0 && c.Phase != "BEFORE" {
		return m.fetchContest()
	}
	return m, nil
}

var conChips = map[string]string{"a": "all", "u": "upcoming", "p": "past", "r": "running", "s": "started"}

// setChip shows one chip's contests; its key again, or "a", goes back to all.
func (m Model) setChip(key string) Model {
	c := conChips[key]
	if c == "all" || m.conChip == c {
		c = "all"
	}
	m.conChip = c
	return m.keepContestVisible()
}

// problemsByContest groups the cached Problems by contest ID, each group in index order.
func (m Model) problemsByContest() map[int][]cf.Problem {
	out := map[int][]cf.Problem{}
	for _, p := range m.Problems {
		out[p.ContestID] = append(out[p.ContestID], p)
	}
	for _, ps := range out {
		sort.Slice(ps, func(i, j int) bool { return ps[i].Index < ps[j].Index })
	}
	return out
}

// contestMatch reports whether every search word hits the contest's name or id, or one of its
// Problems (ps). hit is the first Problem that matched a word the name and id missed.
func (m Model) contestMatch(c cf.Contest, ps []cf.Problem) (hit *cf.Problem, ok bool) {
	hay := clean(c.Name) + " " + fmt.Sprint(c.ID)
	for _, w := range strings.Fields(m.conQ) {
		if _, in := fuzzyScore(w, hay); in {
			continue
		}
		j := slices.IndexFunc(ps, func(p cf.Problem) bool {
			_, in := fuzzyScore(w, p.Index+" "+clean(p.Name))
			return in
		})
		if j < 0 {
			return nil, false
		}
		if hit == nil {
			hit = &ps[j]
		}
	}
	return hit, true
}

// contestIdxs are the shown contests: chip and search applied, in list order. Sorted least solved
// first inside each section when conSort is "least".
func (m Model) contestIdxs() []int {
	byID := m.problemsByContest()
	solved := func(i int) int {
		n := 0
		for _, p := range byID[m.contests[i].ID] {
			if m.statusOf(p) == store.StatusSolved {
				n++
			}
		}
		return n
	}
	var out []int
	for i, c := range m.contests {
		switch m.conChip {
		case "upcoming":
			if c.Phase != "BEFORE" {
				continue
			}
		case "past":
			if !c.Finished() {
				continue
			}
		case "running":
			if c.Phase == "BEFORE" || c.Finished() {
				continue
			}
		case "started":
			if solved(i) == 0 {
				continue
			}
		}
		if _, ok := m.contestMatch(c, byID[c.ID]); ok {
			out = append(out, i)
		}
	}
	if m.conSort == "least" {
		sort.SliceStable(out, func(a, b int) bool {
			x, y := out[a], out[b]
			if (x < m.upcoming) != (y < m.upcoming) {
				return x < m.upcoming
			}
			return solved(x) < solved(y)
		})
	}
	return out
}

// progressText is solved/total of a contest's Problems, "?" during its session, "-" when not cached.
func (m Model) progressText(c cf.Contest, ps []cf.Problem) string {
	if len(ps) == 0 {
		return "-"
	}
	if m.sess != nil && m.sess.ContestID == c.ID {
		return "?"
	}
	n := 0
	for _, p := range ps {
		if m.statusOf(p) == store.StatusSolved {
			n++
		}
	}
	return fmt.Sprintf("%d/%d", n, len(ps))
}

// keepContestVisible moves the cursor onto a contest the search still shows.
func (m Model) keepContestVisible() Model {
	if vis := m.contestIdxs(); len(vis) > 0 && !slices.Contains(vis, m.contestCursor) {
		m.contestCursor = vis[0]
	}
	return m
}

// contestMove moves the cursor n shown contests down (negative: up), clamped to the ends.
func (m Model) contestMove(n int) Model {
	m = m.keepContestVisible()
	if vis := m.contestIdxs(); len(vis) > 0 {
		pos := slices.Index(vis, m.contestCursor)
		m.contestCursor = vis[max(0, min(len(vis)-1, pos+n))]
	}
	return m
}

// contestFilterKey edits the contests search: type to filter, enter keeps it, esc clears it.
func (m Model) contestFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.conFind, m.conQ = false, ""
	case "enter", "down":
		m.conFind = false
	case "backspace":
		if r := []rune(m.conQ); len(r) > 0 {
			m.conQ = string(r[:len(r)-1])
		}
	case "ctrl+u":
		m.conQ = ""
	case "ctrl+c":
		return m, tea.Quit
	default:
		if msg.Text != "" {
			m.conQ += clean(msg.Text)
		}
	}
	return m.keepContestVisible(), nil
}

// contestWindow are the shown contests (see contestIdxs) and the first one on screen.
func (m Model) contestWindow() ([]int, int) {
	vis := m.contestIdxs()
	rows := m.contestRows()
	pos := max(0, slices.Index(vis, m.contestCursor))
	return vis, max(0, min(pos-rows/2, len(vis)-rows))
}

func countdown(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("in %dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("in %dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("in %dm", max(1, int(d.Minutes())))
}

// contestBox is the modal listing one contest's cached Problems.
func (m Model) contestBox() []string {
	c := m.contestOpen
	st := m.styles()
	w, _ := m.modalSize(90, 30)
	ps := m.contestProblems(*c)
	var body []string
	if len(ps) == 0 {
		switch {
		case m.contestBusy:
			body = append(body, st.Dim.Render("fetching Problems..."))
		case c.Phase == "BEFORE":
			body = append(body, st.Dim.Render("not started: Problems appear when the contest begins"))
		default:
			body = append(body, st.Dim.Render("no Problems cached for this contest"))
		}
	}
	if m.contestNote != "" {
		for _, l := range wrap(m.contestNote, w-6) {
			body = append(body, st.Bad.Render(l))
		}
	}
	for i, p := range ps {
		cur := " "
		if i == m.cpCursor {
			cur = st.Accent.Render(">")
		}
		last := m.ratingText(p)
		if m.isBlind(p) {
			last = fmt.Sprintf("%-8s", m.contestResult(p))
		}
		body = append(body, fmt.Sprintf("%s %s %-3s %s %s", cur, m.markOf(m.statusOf(p)), clean(p.Index), fit(clean(p.Name), w-24), last))
	}
	hint := "{contest.open} open  {contest.down}/{contest.up} move  r refetch  {contest.start} session  {contest.practice} practice  {contest.close} close"
	if m.sess != nil && m.sess.ContestID == c.ID {
		body = append([]string{st.Warn.Render(m.sessLine()), ""}, body...)
		hint = "{contest.open} open  {contest.down}/{contest.up} move  {contest.end_session} summary and end  {contest.close} close"
	}
	return box(fmt.Sprintf("%d  %s", c.ID, clean(c.Name)), body, m.kx(hint), w, len(body)+2, true, st, m.borderStyle())
}

func (m Model) viewContests(b *strings.Builder) string {
	if len(m.contests) == 0 {
		b.WriteString("  no contests cached\n")
	}
	vis, start := m.contestWindow()
	rows := m.contestRows()
	if len(vis) == 0 && len(m.contests) > 0 {
		b.WriteString("  no contest matches\n")
	}
	st := m.styles()
	now := m.clock()
	lastSection := ""
	for _, i := range vis[start:min(start+rows, len(vis))] {
		section := "Past"
		if i < m.upcoming {
			section = "Upcoming"
		}
		if section != lastSection {
			b.WriteString(" " + st.Accent2.Render(strings.ToUpper(section)) + "\n")
			lastSection = section
		}
		b.WriteString(m.contestLine(i, now) + "\n")
	}
	return m.contestFooter()
}

// contestLine is one row of the contest list: when, progress, name, id, and the Problem a search
// word hit.
func (m Model) contestLine(i int, now time.Time) string {
	st := m.styles()
	c := m.contests[i]
	when, whenStyle := "running", st.Bad
	switch {
	case c.Finished():
		when, whenStyle = time.Unix(c.Start, 0).Format("2006-01-02"), st.Dim
	case c.Phase == "BEFORE":
		d := time.Unix(c.Start, 0).Sub(now)
		when, whenStyle = countdown(d), st.Good
		if d < 24*time.Hour {
			whenStyle = st.Warn
		}
	}
	ps := m.contestProblems(c)
	hit, _ := m.contestMatch(c, ps)
	nameW := max(30, min(72, m.contentWidth()-30))
	hitText := ""
	if hit != nil { // room for the matched Problem at the end
		nameW = max(12, nameW-26)
		hitText = "  " + st.Dim.Render(xansi.Truncate(clean(hit.Index)+"  "+clean(hit.Name), 24, "…"))
	}
	line := "  " + whenStyle.Render(fmt.Sprintf("%-12s", when)) + " " + fmt.Sprintf("%5s", m.progressText(c, ps)) + " " + fit(clean(c.Name), nameW) + " " + st.Dim.Render(fmt.Sprint(c.ID))
	if m.sess != nil && m.sess.ContestID == c.ID {
		line += " " + st.Warn.Render("● session")
	}
	line += hitText
	if i == m.contestCursor {
		line = paintRow(st.Accent.Render("▌")+" "+strings.TrimPrefix(line, "  "), m.contentWidth(), st.Selected)
	}
	return line
}

// contestFooter is the line under the contest list: the search, chip and sort in force, or the keys.
func (m Model) contestFooter() string {
	if m.conFind {
		return "/ " + m.conQ + "▏  type to filter  enter keep  esc clear"
	}
	chip := m.conChip != "" && m.conChip != "all"
	parts := m.contestParts(chip)
	if len(parts) == 0 {
		return fmt.Sprintf("%d contests  enter Problems  / search  ? help  q quit", len(m.contests))
	}
	if m.conQ != "" || chip {
		parts = append(parts, fmt.Sprintf("%d of %d", len(m.contestIdxs()), len(m.contests)))
	}
	hint := "/ search"
	if m.conQ != "" {
		hint = "/ edit  esc clear"
	}
	if chip {
		hint += "  a all"
	}
	return strings.Join(append(parts, hint), "  ")
}

// contestParts names the search, chip and sort in force.
func (m Model) contestParts(chip bool) []string {
	var parts []string
	if m.conQ != "" {
		parts = append(parts, "/ "+m.conQ)
	}
	if chip {
		parts = append(parts, "chip: "+m.conChip)
	}
	if m.conSort == "least" {
		parts = append(parts, "sort: least solved")
	}
	return parts
}

func (m Model) markOf(s store.Status) string {
	switch s {
	case store.StatusSolved:
		return m.styles().Good.Render("✓")
	case store.StatusAttempted:
		return m.styles().Bad.Render("✗")
	}
	return " "
}

func ratingStr(p cf.Problem) string {
	if p.Rating > 0 {
		return fmt.Sprint(p.Rating)
	}
	return "-"
}

// contestRows is how many contest rows fit under the heading lines; at least one.
func (m Model) contestRows() int { return max(1, m.page()-2) }
