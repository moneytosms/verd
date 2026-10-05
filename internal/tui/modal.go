package tui

import (
	"fmt"
	"strings"

	"github.com/moneytosms/verd/internal/runner"
)

// modalBox is the open modal drawn as a centered frame, or nil when none is open.
// Modals close with q or esc and sit on top of the screen they were opened from.
func (m Model) modalBox() []string {
	switch {
	case m.tm != nil:
		return m.tmBox()
	case m.fm != nil:
		return m.filterBox()
	case m.help:
		return m.helpBox()
	case m.tab == 1 && m.contestOpen != nil && m.open == nil:
		return m.contestBox()
	case m.strs != nil && m.strs.diff:
		s := m.strs
		return m.diffBox(s.counterexample(), &s.diffOff, &s.diffInit, s.saveHint())
	case m.run != nil && m.run.diff:
		if row, ok := m.selected(); ok && row.Res != nil {
			return m.diffBox(*row.Res, &m.run.diffOff, &m.run.diffInit, "")
		}
	case m.subModal && m.sub != nil:
		return m.subBox()
	}
	return nil
}

func (m Model) modalSize(maxW, maxH int) (w, h int) {
	return max(20, min(m.width-4, maxW)), max(6, min(m.height-4, maxH))
}

func (m Model) helpBox() []string {
	screen, keys := m.keys()
	st := m.styles()
	w, h := m.modalSize(84, 40)
	var body []string
	for _, k := range keys {
		body = append(body, fmt.Sprintf("%s %s", st.Accent.Render(fmt.Sprintf("%-16s", k[0])), k[1]))
	}
	return box("Keys: "+screen, body, "q close", w, min(h, len(body)+2), true, st)
}

// diffBox is the side-by-side expected/actual view of one test.
func (m Model) diffBox(res runner.Result, diffOff *int, diffInit *bool, extra string) []string {
	st := m.styles()
	w, h := m.modalSize(100, 36)
	want, got := cleanLines(res.Expected), cleanLines(res.Output)
	var body []string
	if res.Mismatch != nil {
		mm := res.Mismatch
		body = append(body, fmt.Sprintf("first mismatch at line %d col %d: want %q got %q", mm.Line, mm.Col, clean(clip(mm.Want, 40)), clean(clip(mm.Got, 40))))
	}
	rows := max(3, h-2-len(body)-1)
	total := max(len(want), len(got))
	if !*diffInit { // start near the first mismatch
		*diffInit = true
		*diffOff = 0
		if res.Mismatch != nil {
			*diffOff = max(0, res.Mismatch.Line-1-rows/3)
		}
	}
	off := min(*diffOff, max(0, total-rows))
	cw := max(10, (w-4-3)/2)
	body = append(body, st.Dim.Render(fmt.Sprintf("%-*s │ %s", cw, "expected", "actual")))
	end := min(total, off+rows)
	for i := off; i < end; i++ {
		l, a := cell(want, i, cw), cell(got, i, cw)
		if res.Mismatch != nil && i == res.Mismatch.Line-1 {
			l, a = st.Bad.Render(l), st.Bad.Render(a)
		}
		body = append(body, l+st.Dim.Render(" │ ")+a)
	}
	note := fmt.Sprintf("%d-%d/%d%s  j/k scroll  q close", min(off+1, total), end, total, extra)
	return box("Diff: "+res.Name+"  "+strings.TrimSpace(res.Verdict), body, note, w, min(h, len(body)+2), true, st)
}

// subBox is the Submission modal: where it was sent and the Verdict as it arrives.
func (m Model) subBox() []string {
	st := m.styles()
	s := m.sub
	w, _ := m.modalSize(84, 20)
	title := fmt.Sprintf("Submission %d%s", s.problem.ContestID, clean(s.problem.Index))
	status := clean(s.status)
	switch {
	case s.final && s.ok:
		status = st.Good.Render("✓ " + status)
	case s.final:
		status = st.Bad.Render("✗ " + status)
	default:
		status = st.Warn.Render("… ") + status
	}
	body := []string{status, ""}
	for _, n := range s.notes {
		body = append(body, st.Dim.Render(clean(n)))
	}
	if s.final && s.detail != "" {
		body = append(body, "", s.detail)
	}
	note := "q close (tracking continues)"
	if s.final {
		note = "q close"
	}
	return box(title, body, note, w, len(body)+2, true, st)
}
