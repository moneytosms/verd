package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/stats"
	"github.com/moneytosms/verd/internal/theme"
)

const statsTopics = 12 // topic rows shown before "+N more"

// statsLines renders the Stats tab as lines, sized for the current width.
func (m Model) statsLines() []string {
	st, ds := m.stats, m.styles()
	if st.Submissions == 0 && len(st.RatingHistory) == 0 {
		return []string{"", "  No Submissions cached yet.", ds.Dim.Render(m.kx("  Press {app.refresh} to sync your history from Codeforces."))}
	}
	w := max(20, min(m.width-6, 64))
	var out []string
	out = append(out, summaryLines(st)...)
	out = append(out, ratingChartLines(st, w, ds)...)
	out = append(out, solvedBarLines(st, ds)...)
	out = append(out, bandLines(st, ds)...)
	out = append(out, topicLines(st, ds)...)
	out = append(out, m.activityLines(st, w, ds)...)
	out = append(out, m.unsolvedLines(st, ds)...)
	return out
}

func summaryLines(st stats.Stats) []string {
	lines := []string{fmt.Sprintf("Solved %d   Attempted %d   Submissions %d   AC %.0f%%", st.Solved, st.Attempted, st.Submissions, st.ACRate*100)}
	if st.Rating > 0 {
		return append(lines, fmt.Sprintf("Rating %d (max %d)  %s", st.Rating, st.MaxRating, st.Rank))
	}
	return append(lines, "Rating: unrated")
}

// ratingChartLines plots the rating history, labelled with its high and low.
func ratingChartLines(st stats.Stats, w int, ds theme.Styles) []string {
	if len(st.RatingHistory) <= 1 {
		return nil
	}
	vals := make([]int, len(st.RatingHistory))
	lo, hi := st.RatingHistory[0].NewRating, st.RatingHistory[0].NewRating
	for i, r := range st.RatingHistory {
		vals[i] = r.NewRating
		lo, hi = min(lo, r.NewRating), max(hi, r.NewRating)
	}
	label := max(len(fmt.Sprint(hi)), len(fmt.Sprint(lo)))
	rows := LineChart(vals, w-label-1, 5)
	var out []string
	for i, r := range rows {
		l := strings.Repeat(" ", label)
		switch i {
		case 0:
			l = fmt.Sprintf("%*d", label, hi)
		case len(rows) - 1:
			l = fmt.Sprintf("%*d", label, lo)
		}
		out = append(out, ds.Dim.Render(l)+" "+ds.Accent.Render(r))
	}
	return out
}

func solvedBarLines(st stats.Stats, ds theme.Styles) []string {
	out := []string{"", ds.Accent.Render("Solved by rating")}
	var buckets []int
	for r := 800; r <= 3500; r += 100 {
		buckets = append(buckets, st.SolvedByRating[r])
	}
	for _, r := range Bars(buckets, 4) {
		out = append(out, ds.Good.Render(r))
	}
	out = append(out, ds.Dim.Render(Axis("800", "3500", len(buckets))))
	if st.SolvedUnrated > 0 {
		out = append(out, ds.Dim.Render(fmt.Sprintf("+ %d solved without a rating", st.SolvedUnrated)))
	}
	return out
}

func bandLines(st stats.Stats, ds theme.Styles) []string {
	out := []string{"", ds.Accent.Render(fmt.Sprintf("Strengths and weaknesses (rating %d-%d)", st.Band[0], st.Band[1]))}
	if len(st.Strengths)+len(st.Weaknesses) == 0 {
		out = append(out, ds.Dim.Render("  not enough Problems per topic in this band yet"))
	}
	for _, t := range st.Strengths {
		out = append(out, "  "+ds.Good.Render("+ ")+clean(t.String()))
	}
	for _, t := range st.Weaknesses {
		out = append(out, "  "+ds.Bad.Render("- ")+clean(t.String()))
	}
	return out
}

func topicLines(st stats.Stats, ds theme.Styles) []string {
	out := []string{"", ds.Accent.Render("Topics"), ds.Dim.Render(fmt.Sprintf("  %-26s %6s %6s %9s", "tag", "solved", "avg", "first-try"))}
	for i, t := range st.Tags {
		if i == statsTopics {
			return append(out, ds.Dim.Render(fmt.Sprintf("  (+%d more)", len(st.Tags)-statsTopics)))
		}
		avg := "-"
		if t.AvgRating > 0 {
			avg = fmt.Sprintf("%.0f", t.AvgRating)
		}
		bar := strings.Repeat("█", max(1, t.Solved*14/max(1, st.Tags[0].Solved)))
		out = append(out, fmt.Sprintf("  %-26.26s %6d %6s %s  %s", clean(t.Tag), t.Solved, avg, firstTryStyle(t.FirstTryRate, ds), ds.Accent.Render(bar)))
	}
	return out
}

// firstTryStyle renders a first-try rate, coloured by how good it is.
func firstTryStyle(rate float64, ds theme.Styles) string {
	s := fmt.Sprintf("%8.0f%%", rate*100)
	switch {
	case rate >= 0.85:
		return ds.Good.Render(s)
	case rate >= 0.7:
		return ds.Warn.Render(s)
	default:
		return ds.Bad.Render(s)
	}
}

// activityLines covers the streak, the submission heatmap and the verdict counts.
func (m Model) activityLines(st stats.Stats, w int, ds theme.Styles) []string {
	out := []string{fmt.Sprintf("Streak: %d day(s), best %d", st.Streak, st.MaxStreak)}
	for _, r := range Heatmap(st.Days, m.clock(), min(26, (w-4)/2)) {
		out = append(out, ds.Good.Render(r))
	}
	return append(out, verdictLine(st))
}

func verdictLine(st stats.Stats) string {
	type vc struct {
		v string
		n int
	}
	var vs []vc
	for v, n := range st.Verdicts {
		vs = append(vs, vc{v, n})
	}
	sort.Slice(vs, func(i, j int) bool {
		if vs[i].n != vs[j].n {
			return vs[i].n > vs[j].n
		}
		return vs[i].v < vs[j].v
	})
	var parts []string
	for _, x := range vs {
		parts = append(parts, fmt.Sprintf("%s %d", shortVerdict(x.v), x.n))
	}
	return "Verdicts: " + strings.Join(parts, "  ")
}

func (m Model) unsolvedLines(st stats.Stats, ds theme.Styles) []string {
	out := []string{"", ds.Accent.Render("Attempted, not solved") + ds.Dim.Render("  (n/N select, enter open)")}
	if len(st.Unsolved) == 0 {
		out = append(out, ds.Dim.Render("  none"))
	}
	for i, a := range st.Unsolved {
		line := fmt.Sprintf("  %d%-3s %-34.34s %d attempt(s)", a.ContestID, clean(a.Index), clean(a.Name), a.Attempts)
		if i == m.statsSel {
			line = paintRow(ds.Accent.Render("▌ ")+strings.TrimPrefix(line, "  "), m.contentWidth()-2, ds.Selected)
		}
		out = append(out, line)
	}
	return out
}

func shortVerdict(v string) string {
	switch v {
	case "OK":
		return "AC"
	case "WRONG_ANSWER":
		return "WA"
	case "TIME_LIMIT_EXCEEDED":
		return "TLE"
	case "RUNTIME_ERROR":
		return "RE"
	case "MEMORY_LIMIT_EXCEEDED":
		return "MLE"
	case "COMPILATION_ERROR":
		return "CE"
	}
	return clean(strings.ToLower(v))
}

func (m Model) viewStats(b *strings.Builder) string {
	lines := m.statsLines()
	end := min(len(lines), m.statsScroll+m.page())
	for _, l := range lines[min(m.statsScroll, end):end] {
		b.WriteString(" " + l + "\n")
	}
	return "{stats.down}/{stats.up} scroll  {stats.next_problem}/{stats.prev_problem} select  {stats.open} open  {stats.picker} weak-topics Picker  {common.help} help  {stats.quit} quit"
}

func (m Model) updateStats(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	n := len(m.statsLines())
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.statsScroll = min(max(0, n-m.page()), m.statsScroll+1)
	case "k", "up":
		m.statsScroll = max(0, m.statsScroll-1)
	case "pgdown":
		m.statsScroll = min(max(0, n-m.page()), m.statsScroll+m.page())
	case "pgup":
		m.statsScroll = max(0, m.statsScroll-m.page())
	case "n":
		m.statsSel = min(len(m.stats.Unsolved)-1, m.statsSel+1)
	case "N":
		m.statsSel = max(0, m.statsSel-1)
	case "p": // jump to the Picker with the weak-topics preset
		m.tab = 3
		return m.withWeakPreset(), nil
	case "enter":
		if m.statsSel < len(m.stats.Unsolved) {
			a := m.stats.Unsolved[m.statsSel]
			return m.openProblem(m.problemFor(a.ContestID, a.Index, a.Name))
		}
	}
	return m, nil
}

// problemFor finds a cached Problem, else builds a bare one (the detail view loads by id).
func (m Model) problemFor(contest int, index, name string) cf.Problem {
	for _, p := range m.Problems {
		if p.ContestID == contest && p.Index == index {
			return p
		}
	}
	return cf.Problem{ContestID: contest, Index: index, Name: name}
}
