package tui

import (
	"fmt"
	"hash/fnv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/moneytosms/verd/internal/cf"
)

// tagStyle gives every tag its own stable colour from the theme.
func (m Model) tagStyle(tag string) lipgloss.Style {
	st := m.styles()
	h := fnv.New32a()
	h.Write([]byte(tag))
	pick := []lipgloss.Style{st.Accent, st.Accent2, st.Good, st.Warn}
	return pick[h.Sum32()%uint32(len(pick))].Bold(false)
}

func (m Model) tagChips(p cf.Problem, w int) string {
	var parts []string
	used := 0
	for _, t := range p.Tags {
		t = clean(t)
		if used+len([]rune(t))+1 > w {
			parts = append(parts, m.styles().Dim.Render("…"))
			break
		}
		parts = append(parts, m.tagStyle(t).Render(t))
		used += len([]rune(t)) + 2
	}
	return strings.Join(parts, m.styles().Dim.Render(", "))
}

// chipsBar shows the active filters, search and sort as chips (or the problem count when none).
func (m Model) chipsBar() string {
	st := m.styles()
	chip := func(s string) string { return st.Chip.Render(" " + s + " ") }
	count := fmt.Sprintf("%d problems", len(m.visible))
	if len(m.visible) != len(m.Problems) {
		count = fmt.Sprintf("%d of %d problems", len(m.visible), len(m.Problems))
	}
	out := []string{" " + st.Dim.Render(count)}
	f := m.filter
	switch {
	case f.MinRating > 0 && f.MaxRating > 0:
		out = append(out, chip(fmt.Sprintf("rating %d–%d", f.MinRating, f.MaxRating)))
	case f.MinRating > 0:
		out = append(out, chip(fmt.Sprintf("rating ≥ %d", f.MinRating)))
	case f.MaxRating > 0:
		out = append(out, chip(fmt.Sprintf("rating ≤ %d", f.MaxRating)))
	}
	for _, t := range f.Include {
		out = append(out, st.Good.Render("+")+chip(clean(t)))
	}
	for _, t := range f.Exclude {
		out = append(out, st.Bad.Render("−")+chip(clean(t)))
	}
	if f.Unsolved || f.Status != "" {
		s := f.Status
		if s == "" {
			s = "unsolved"
		}
		out = append(out, chip(s))
	}
	if m.sortBy != "" {
		out = append(out, chip("sort: "+sortNames[m.sortBy]))
	}
	if f.Search != "" {
		out = append(out, chip(fmt.Sprintf("/ %s [%s]", clean(f.Search), m.searchMode())))
	}
	if len(out) == 1 {
		out = append(out, st.Dim.Render("f filters   / search   : expression"))
	}
	return strings.Join(out, " ")
}

func (m Model) viewList(b *strings.Builder) string {
	st := m.styles()
	w := m.contentWidth()
	nameW := max(20, min(46, (w-43)/2))
	b.WriteString(m.chipsBar() + "\n")
	b.WriteString(st.Dim.Render(fmt.Sprintf("    %-5s %-10s %s %6s %7s  %s", "#", "Source", fit("Name", nameW), "Rating", "Solved", "Tags")) + "\n")
	rows := m.listRows()
	start := m.listStart()
	for i := start; i < min(start+rows, len(m.visible)); i++ {
		p := m.visible[i]
		id := fmt.Sprintf("%-5d %-10s", i+1, cf.SourceName(p.Source()))
		tagW := max(10, w-nameW-43)
		if i == m.cursor {
			line := st.Accent.Render("▌") + " " + m.markOf(m.statusOf(p)) + " " + st.Accent.Render(id) + " " + fit(clean(p.Name), nameW) + " " + m.ratingText(p) + fmt.Sprintf(" %7d  ", p.SolvedCount) + m.tagChips(p, tagW)
			b.WriteString(paintRow(line, w, st.Selected) + "\n")
			continue
		}
		b.WriteString("  " + m.markOf(m.statusOf(p)) + " " + id + " " + fit(clean(p.Name), nameW) + " " + m.ratingText(p) + fmt.Sprintf(" %7d  ", p.SolvedCount) + m.tagChips(p, tagW) + "\n")
	}
	if len(m.visible) == 0 {
		b.WriteString("\n  " + st.Dim.Render("no problems match: X clears the filters") + "\n")
	}
	if m.input != nil {
		if m.input.kind == '/' {
			b.WriteString("\n " + st.Accent.Render("/ ") + m.input.text + st.Accent.Render("▏") + "   " + st.Chip.Render(" "+m.searchMode()+" "))
			return fmt.Sprintf("%d matches  type to search (fuzzy, #tag)  tab mode: all/name/tag/id  enter keep  esc undo", len(m.visible))
		}
		b.WriteString("\n " + st.Accent.Render(string(m.input.kind)+" ") + m.input.text + st.Accent.Render("▏"))
		if m.inputErr != "" {
			b.WriteString("   " + st.Bad.Render(m.inputErr))
		}
		return "enter apply  esc cancel"
	}
	return "j/k move  enter open  f filters  / search  : expression  X clear  ? help  q quit"
}

// listRows is how many Problem rows fit; listStart is the first one shown.
func (m Model) listRows() int { return max(1, m.page()-2) }

func (m Model) listStart() int {
	rows := m.listRows()
	return max(0, min(m.cursor-rows/2, len(m.visible)-rows))
}
