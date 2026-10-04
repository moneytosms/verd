package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
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

func (m Model) updateContests(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.contestOpen != nil {
		ps := m.contestProblems(*m.contestOpen)
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.contestOpen, m.cpCursor = nil, 0
		case "up", "k":
			m.cpCursor = max(0, m.cpCursor-1)
		case "down", "j":
			m.cpCursor = min(len(ps)-1, m.cpCursor+1)
		case "enter":
			if len(ps) > 0 {
				return m.openProblem(ps[m.cpCursor])
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.contestCursor = max(0, m.contestCursor-1)
	case "down", "j":
		m.contestCursor = min(len(m.contests)-1, m.contestCursor+1)
	case "pgup":
		m.contestCursor = max(0, m.contestCursor-m.page())
	case "pgdown":
		m.contestCursor = max(0, min(len(m.contests)-1, m.contestCursor+m.page()))
	case "enter":
		if len(m.contests) > 0 {
			c := m.contests[m.contestCursor]
			m.contestOpen, m.cpCursor = &c, 0
		}
	}
	return m, nil
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

func (m Model) viewContests(b *strings.Builder) string {
	if c := m.contestOpen; c != nil {
		b.WriteString(fmt.Sprintf(" %d  %s\n\n", c.ID, clean(c.Name)))
		ps := m.contestProblems(*c)
		if len(ps) == 0 {
			b.WriteString("  no Problems cached for this contest\n")
		}
		for i, p := range ps {
			cur := " "
			if i == m.cpCursor {
				cur = m.styles().Accent.Render(">")
			}
			b.WriteString(fmt.Sprintf("%s %s %-3s %-40.40s %6s\n", cur, m.markOf(m.statusOf(p)), clean(p.Index), clean(p.Name), ratingStr(p)))
		}
		return "esc back  enter open  j/k move  ? help  q quit"
	}
	if len(m.contests) == 0 {
		b.WriteString("  no contests cached\n")
	}
	rows := m.page() - 2
	start := max(0, min(m.contestCursor-rows/2, len(m.contests)-rows))
	now := m.clock()
	lastSection := ""
	for i := start; i < min(start+rows, len(m.contests)); i++ {
		c := m.contests[i]
		section := "Past"
		if i < m.upcoming {
			section = "Upcoming"
		}
		if section != lastSection {
			b.WriteString(" " + section + "\n")
			lastSection = section
		}
		cur := " "
		if i == m.contestCursor {
			cur = m.styles().Accent.Render(">")
		}
		when := ""
		switch {
		case c.Finished():
			when = time.Unix(c.Start, 0).Format("2006-01-02")
		case c.Phase == "BEFORE":
			when = countdown(time.Unix(c.Start, 0).Sub(now))
		default:
			when = "running"
		}
		b.WriteString(fmt.Sprintf("%s %-12s %-50.50s %s\n", cur, when, clean(c.Name), fmt.Sprint(c.ID)))
	}
	return fmt.Sprintf("%d contests  enter Problems  ? help  q quit", len(m.contests))
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
