package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
)

const statsTopics = 12 // topic rows shown before "+N more"

// statsLines renders the Stats tab as lines, sized for the current width.
func (m Model) statsLines() []string {
	st, ds := m.stats, m.styles()
	if st.Submissions == 0 && len(st.RatingHistory) == 0 {
		return []string{"", "  No Submissions cached yet.", ds.Dim.Render("  Press ctrl+r to sync your history from Codeforces.")}
	}
	w := max(20, min(m.width-6, 64))
	var out []string
	add := func(ls ...string) { out = append(out, ls...) }

	add(fmt.Sprintf("Solved %d   Attempted %d   Submissions %d   AC %.0f%%", st.Solved, st.Attempted, st.Submissions, st.ACRate*100))
	if st.Rating > 0 {
		add(fmt.Sprintf("Rating %d (max %d)  %s", st.Rating, st.MaxRating, st.Rank))
	} else {
		add("Rating: unrated")
	}
	if len(st.RatingHistory) > 1 {
		vals := make([]int, len(st.RatingHistory))
		lo, hi := st.RatingHistory[0].NewRating, st.RatingHistory[0].NewRating
		for i, r := range st.RatingHistory {
			vals[i] = r.NewRating
			lo, hi = min(lo, r.NewRating), max(hi, r.NewRating)
		}
		label := max(len(fmt.Sprint(hi)), len(fmt.Sprint(lo)))
		rows := LineChart(vals, w-label-1, 5)
		for i, r := range rows {
			l := strings.Repeat(" ", label)
			switch i {
			case 0:
				l = fmt.Sprintf("%*d", label, hi)
			case len(rows) - 1:
				l = fmt.Sprintf("%*d", label, lo)
			}
			add(ds.Dim.Render(l) + " " + ds.Accent.Render(r))
		}
	}

	add("", ds.Accent.Render("Solved by rating"))
	var buckets []int
	for r := 800; r <= 3500; r += 100 {
		buckets = append(buckets, st.SolvedByRating[r])
	}
	for _, r := range Bars(buckets, 4) {
		add(ds.Good.Render(r))
	}
	add(ds.Dim.Render(Axis("800", "3500", len(buckets))))
	if st.SolvedUnrated > 0 {
		add(ds.Dim.Render(fmt.Sprintf("+ %d solved without a rating", st.SolvedUnrated)))
	}

	add("", ds.Accent.Render(fmt.Sprintf("Strengths and weaknesses (rating %d-%d)", st.Band[0], st.Band[1])))
	if len(st.Strengths)+len(st.Weaknesses) == 0 {
		add(ds.Dim.Render("  not enough Problems per topic in this band yet"))
	}
	for _, t := range st.Strengths {
		add("  " + ds.Good.Render("+ ") + clean(t.String()))
	}
	for _, t := range st.Weaknesses {
		add("  " + ds.Bad.Render("- ") + clean(t.String()))
	}

	add("", ds.Accent.Render("Topics"), ds.Dim.Render(fmt.Sprintf("  %-26s %6s %6s %9s", "tag", "solved", "avg", "first-try")))
	for i, t := range st.Tags {
		if i == statsTopics {
			add(ds.Dim.Render(fmt.Sprintf("  (+%d more)", len(st.Tags)-statsTopics)))
			break
		}
		avg := "-"
		if t.AvgRating > 0 {
			avg = fmt.Sprintf("%.0f", t.AvgRating)
		}
		bar := strings.Repeat("█", max(1, t.Solved*14/max(1, st.Tags[0].Solved)))
		rate := fmt.Sprintf("%8.0f%%", t.FirstTryRate*100)
		switch {
		case t.FirstTryRate >= 0.85:
			rate = ds.Good.Render(rate)
		case t.FirstTryRate >= 0.7:
			rate = ds.Warn.Render(rate)
		default:
			rate = ds.Bad.Render(rate)
		}
		add(fmt.Sprintf("  %-26.26s %6d %6s %s  %s", clean(t.Tag), t.Solved, avg, rate, ds.Accent.Render(bar)))
	}

	add("", fmt.Sprintf("Streak: %d day(s), best %d", st.Streak, st.MaxStreak))
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
	add("Verdicts: " + strings.Join(parts, "  "))

	add("", ds.Accent.Render("Attempted, not solved")+ds.Dim.Render("  (n/N select, enter open)"))
	if len(st.Unsolved) == 0 {
		add(ds.Dim.Render("  none"))
	}
	for i, a := range st.Unsolved {
		line := fmt.Sprintf("  %d%-3s %-34.34s %d attempt(s)", a.ContestID, clean(a.Index), clean(a.Name), a.Attempts)
		if i == m.statsSel {
			line = paintRow(ds.Accent.Render("▌ ")+strings.TrimPrefix(line, "  "), m.contentWidth()-2, ds.Selected)
		}
		add(line)
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
	return "j/k scroll  n/N select  enter open  p weak-topics Picker  ? help  q quit"
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
