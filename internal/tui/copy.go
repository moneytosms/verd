package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// copyText is the plain text of the focused Problem pane: the whole question, every test, or the selected test.
func (m Model) copyText() (what, text string) {
	var b strings.Builder
	switch m.pane {
	case paneTests:
		what = "tests"
		for _, r := range m.rows() {
			writeCase(&b, r)
		}
	case paneDetail:
		row, ok := m.selected()
		if !ok {
			return "", ""
		}
		what = row.Name
		writeCase(&b, row)
	default:
		what = "question"
		p := *m.open
		fmt.Fprintf(&b, "%d%s  %s\n", p.ContestID, p.Index, p.Name)
		rating := "unrated"
		if p.Rating > 0 {
			rating = fmt.Sprint(p.Rating)
		}
		fmt.Fprintf(&b, "Difficulty: %s\n", rating)
		if len(p.Tags) > 0 {
			fmt.Fprintf(&b, "Tags: %s\n", strings.Join(p.Tags, ", "))
		}
		if d := m.detail; d != nil {
			fmt.Fprintf(&b, "Limits: %.4g s, %d MB\n", float64(d.TimeLimitMS)/1000, d.MemoryLimitMB)
		}
		fmt.Fprintf(&b, "https://codeforces.com/problemset/problem/%d/%s\n\n", p.ContestID, p.Index)
		for _, l := range m.stmtLines() {
			b.WriteString(strings.TrimRight(xansiStrip(l), " ") + "\n")
		}
		for _, r := range m.rows() {
			writeCase(&b, r)
		}
	}
	return what, strings.TrimSpace(b.String()) + "\n"
}

func writeCase(b *strings.Builder, r caseRow) {
	fmt.Fprintf(b, "\n%s\nInput:\n%s\nExpected:\n%s\n", r.Name, strings.TrimRight(r.Input, "\n"), strings.TrimRight(r.Want, "\n"))
	if r.Res != nil {
		fmt.Fprintf(b, "Verdict: %s\nOutput:\n%s\n", strings.TrimSpace(r.Res.Verdict), strings.TrimRight(r.Res.Output, "\n"))
	}
}

// copyPane puts the focused pane on the clipboard (OSC 52, works over ssh/tmux).
func (m Model) copyPane() (Model, tea.Cmd) {
	what, text := m.copyText()
	if text == "" {
		m.Notice = "nothing to copy"
		return m, nil
	}
	m.Notice = "copied " + what
	return m, tea.SetClipboard(text)
}
