package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// bgSeq is the escape sequence that starts s's background (empty if the style has none).
func bgSeq(s lipgloss.Style) string {
	r := s.Render("X")
	if i := strings.Index(r, "X"); i > 0 {
		return r[:i]
	}
	return ""
}

// paintRow pads line to w columns and gives the whole row s's background, re-applying it after
// every reset inside the line so coloured cells do not punch holes in it.
func paintRow(line string, w int, s lipgloss.Style) string {
	line = xansi.Truncate(line, w, "…")
	line += strings.Repeat(" ", max(0, w-xansi.StringWidth(line)))
	seq := bgSeq(s)
	if seq == "" {
		return line
	}
	line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+seq)
	line = strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+seq)
	return seq + line + "\x1b[0m"
}

// ratingStyle colours a rating like the Codeforces rank bands (ANSI palette, so themes apply).
func ratingStyle(r int) lipgloss.Style {
	band := []int{1200, 1400, 1600, 1900, 2100, 2400}
	colors := []int{8, 2, 6, 4, 5, 3, 1} // gray green cyan blue violet orange red
	i := 0
	for i < len(band) && r >= band[i] {
		i++
	}
	return lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(colors[i]))
}

const logo = " verd "

// tabRanges are the [start, end) columns of each tab pill in the header, for mouse clicks.
// It only includes visible tabs.
func (m Model) tabRanges() [][2]int {
	visible := m.visibleTabs()
	x := xansi.StringWidth(logo) + 1
	out := make([][2]int, len(visible))
	for i, tabName := range visible {
		label := fmt.Sprintf(" %d %s ", i+1, tabName)
		w := xansi.StringWidth(label)
		out[i] = [2]int{x, x + w}
		x += w + 1
	}
	return out
}

// header is the top bar: logo, tab pills, and who you are on the right.
func (m Model) header(w int) string {
	st := m.styles()
	var b strings.Builder
	b.WriteString(st.PillOn.Render(logo) + " ")
	visible := m.visibleTabs()
	for i, tabName := range visible {
		tabID := m.tabIDFromPos(i)
		label := fmt.Sprintf(" %d %s ", i+1, tabName)
		if tabID == m.tab {
			b.WriteString(st.PillOn.Render(label))
		} else {
			b.WriteString(st.Dim.Render(" ") + st.Key.Render(fmt.Sprint(i+1)) + st.PillOff.Render(" "+tabName+" "))
		}
		if i < len(visible)-1 {
			b.WriteString(" ")
		}
	}
	left := b.String()
	var who string
	if m.handle != "" {
		who = st.Fg().Render(clean(m.handle))
		if m.rating > 0 {
			who += " " + ratingStyle(m.rating).Bold(true).Render(fmt.Sprint(m.rating))
		}
	}
	gap := w - xansi.StringWidth(left) - xansi.StringWidth(who) - 1
	if gap < 1 {
		who, gap = "", 1
	}
	return paintRow(left+strings.Repeat(" ", gap)+who, w, st.Bar)
}

// rule is the thin line under the header.
func (m Model) rule(w int) string { return m.styles().Dim.Render(strings.Repeat("─", w)) }

// hintBar renders "key desc  key desc" with the keys picked out, on the footer bar.
func (m Model) hintBar(s string, w int) string {
	st := m.styles()
	var parts []string
	for _, p := range strings.Split(s, "  ") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		first, rest, ok := strings.Cut(p, " ")
		if ok && len(first) <= 14 && !(first[0] >= '0' && first[0] <= '9') {
			parts = append(parts, st.Key.Render(first)+" "+st.Dim.Render(rest))
		} else {
			parts = append(parts, st.Dim.Render(p))
		}
	}
	return paintRow(" "+strings.Join(parts, st.Dim.Render("  ")), w, st.Bar)
}

// badge is a small coloured tag for the status line.
func badge(s lipgloss.Style, text string) string { return s.Render("● " + text) }
