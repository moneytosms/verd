package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/theme"
)

// fit truncates s to w columns and pads it with spaces to exactly w (ANSI-aware).
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = xansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-xansi.StringWidth(s)))
}

// box draws body inside a w x h rounded frame with title in the top border and note in the bottom
// one. The frame is accented when focused. Body lines are truncated, missing lines left blank.
func box(title string, body []string, note string, w, h int, focus bool, st theme.Styles) []string {
	if w < 6 || h < 3 {
		return nil
	}
	line := st.Dim
	if focus {
		line = st.Accent
	}
	top := "╭─ " + title + " "
	bot := ""
	if note != "" {
		bot = " " + note + " "
	}
	topFill := max(0, w-2-xansi.StringWidth(top)+1)
	out := make([]string, 0, h)
	out = append(out, line.Render(xansi.Truncate(top+strings.Repeat("─", topFill), w-1, ""))+line.Render("╮"))
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(body) {
			l = body[i]
		}
		out = append(out, line.Render("│")+" "+fit(l, w-4)+" "+line.Render("│"))
	}
	out = append(out, line.Render("╰"+strings.Repeat("─", max(0, w-2-xansi.StringWidth(bot)))+bot+"╯"))
	return out
}

// joinCols places two blocks side by side; the shorter one is padded with blank lines.
func joinCols(a []string, aw int, b []string, gap string) []string {
	n := max(len(a), len(b))
	out := make([]string, n)
	for i := range out {
		l, r := "", ""
		if i < len(a) {
			l = a[i]
		}
		if i < len(b) {
			r = b[i]
		}
		out[i] = fit(l, aw) + gap + r
	}
	return out
}

// overlay draws box centered over base (a w x h screen).
func overlay(base []string, box []string, w, h int) []string {
	if len(box) == 0 {
		return base
	}
	bw := 0
	for _, l := range box {
		bw = max(bw, xansi.StringWidth(l))
	}
	x, y := max(0, (w-bw)/2), max(0, (h-len(box))/2)
	out := append([]string(nil), base...)
	for len(out) < y+len(box) {
		out = append(out, "")
	}
	for i, l := range box {
		row := out[y+i]
		head := fit(xansi.Truncate(row, x, ""), x)
		tail := xansi.TruncateLeft(row, x+bw, "")
		out[y+i] = head + l + tail
	}
	return out
}

// ratingText is the Problem's rating coloured like the Codeforces rank bands (ANSI palette, so themes apply).
func (m Model) ratingText(p cf.Problem) string {
	s := fmt.Sprintf("%6s", ratingStr(p))
	if p.Rating <= 0 {
		return m.styles().Dim.Render(s)
	}
	band := []int{1200, 1400, 1600, 1900, 2100, 2400}
	colors := []int{8, 2, 6, 4, 5, 3, 1} // gray green cyan blue violet orange red
	i := 0
	for i < len(band) && p.Rating >= band[i] {
		i++
	}
	return lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(colors[i])).Render(s)
}
