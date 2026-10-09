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

// borderGlyphs returns the box-drawing characters for the given border style.
type borderGlyphs struct {
	topLeft, topRight, botLeft, botRight, horizontal, vertical string
}

func getBorderGlyphs(style string) borderGlyphs {
	switch style {
	case "square":
		return borderGlyphs{"┌", "┐", "└", "┘", "─", "│"}
	case "heavy":
		return borderGlyphs{"┏", "┓", "┗", "┛", "━", "┃"}
	case "double":
		return borderGlyphs{"╔", "╗", "╚", "╝", "═", "║"}
	case "ascii":
		return borderGlyphs{"+", "+", "+", "+", "-", "|"}
	case "none":
		return borderGlyphs{"", "", "", "", "", ""}
	default: // rounded
		return borderGlyphs{"╭", "╮", "╰", "╯", "─", "│"}
	}
}

// box draws body inside a w x h frame with title in the top border and note in the bottom
// one. The frame is accented when focused. Body lines are truncated, missing lines left blank.
// borderStyle is one of: rounded (default), square, heavy, double, ascii, none.
func box(title string, body []string, note string, w, h int, focus bool, st theme.Styles, borderStyle string) []string {
	if w < 6 || h < 3 {
		return nil
	}
	line := st.Dim
	if focus {
		line = st.Accent
	}
	bg := getBorderGlyphs(borderStyle)

	// For "none" style, draw just the top bar with the title
	if borderStyle == "none" {
		out := make([]string, 0, h)
		head := "─ " + title + " "
		out = append(out, line.Render(xansi.Truncate(head+strings.Repeat("─", max(0, w-xansi.StringWidth(head))), w, "")))
		for i := 1; i < h; i++ {
			l := ""
			if i-1 < len(body) {
				l = body[i-1]
			}
			out = append(out, " "+fit(l, w-2)+" ")
		}
		return out
	}

	cw := xansi.StringWidth
	head := bg.topLeft + bg.horizontal + " " + title + " "
	bot := ""
	if note != "" {
		bot = " " + note + " "
	}
	topFill := max(0, w-cw(head)-cw(bg.topRight))
	out := make([]string, 0, h)
	out = append(out, line.Render(xansi.Truncate(head+strings.Repeat(bg.horizontal, topFill), w-cw(bg.topRight), ""))+line.Render(bg.topRight))
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(body) {
			l = body[i]
		}
		out = append(out, line.Render(bg.vertical)+" "+fit(l, w-4)+" "+line.Render(bg.vertical))
	}
	out = append(out, line.Render(bg.botLeft+strings.Repeat(bg.horizontal, max(0, w-cw(bg.botLeft)-cw(bg.botRight)-cw(bot)))+bot+bg.botRight))
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
