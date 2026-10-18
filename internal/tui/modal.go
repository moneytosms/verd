package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/moneytosms/verd/internal/runner"
)

// modalBox is the open modal drawn as a centered frame, or nil when none is open.
// Modals close with q or esc and sit on top of the screen they were opened from.
func (m Model) modalBox() []string {
	switch {
	case m.ke != nil:
		return m.keyEditorBox()
	case m.sp != nil:
		return m.snippetBox()
	case m.tm != nil:
		return m.tmBox()
	case m.fm != nil:
		return m.filterBox()
	case m.help:
		return m.helpBox()
	case m.full && m.viewing():
		return m.fullBox()
	case m.tab == 1 && m.contestOpen != nil && !m.viewing():
		return m.contestBox()
	case m.strs != nil && m.strs.diff:
		s := m.strs
		return m.diffBox(s.counterexample(), &s.diffOff, &s.diffInit, s.saveHint())
	case m.run != nil && m.run.diff:
		if row, ok := m.selected(); ok && row.Res != nil {
			return m.diffBox(*row.Res, &m.run.diffOff, &m.run.diffInit, "")
		}
		if m.run.overall == runner.CE {
			return m.textBox("Compile error", cleanLines(m.run.compile), &m.run.diffOff, true)
		}
	case m.subModal && m.sub != nil:
		return m.subBox()
	}
	return nil
}

func (m Model) modalSize(maxW, maxH int) (w, h int) {
	return max(20, min(m.width-4, maxW)), max(6, min(m.height-4, maxH))
}

// diffBox is the side-by-side expected/actual view of one test.
func (m Model) diffBox(res runner.Result, diffOff *int, diffInit *bool, extra string) []string {
	switch res.Verdict {
	case runner.RE, runner.TLE, runner.MLE:
		body := []string{res.Detail(), ""}
		if strings.TrimSpace(res.Stderr) == "" {
			body = append(body, "(stderr empty)")
		} else {
			body = append(body, "stderr:")
			body = append(body, cleanLines(res.Stderr)...)
		}
		if strings.TrimSpace(res.Input) != "" {
			body = append(body, "", "input:")
			body = append(body, cleanLines(res.Input)...)
		}
		return m.textBox(res.Name+"  "+strings.TrimSpace(res.Verdict), body, diffOff, false)
	}
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
	note := m.kx(fmt.Sprintf("%d-%d/%d%s  {problem.down}/{problem.up} scroll  {problem.back} close", min(off+1, total), end, total, extra))
	return box("Diff: "+res.Name+"  "+strings.TrimSpace(res.Verdict), body, note, w, min(h, len(body)+2), true, st, m.borderStyle())
}

// textBox is a scrollable plain-text modal (compiler output, runtime errors); hl marks file:line lines.
func (m Model) textBox(title string, lines []string, off *int, hl bool) []string {
	st := m.styles()
	w, h := m.modalSize(100, 36)
	rows := max(3, h-3)
	o := min(*off, max(0, len(lines)-rows))
	end := min(len(lines), o+rows)
	body := make([]string, 0, rows)
	for _, l := range lines[o:end] {
		l = cell([]string{l}, 0, w-4)
		if hl && fileLine.MatchString(l) {
			l = st.Bad.Render(l)
		}
		body = append(body, l)
	}
	hint := ""
	if hl && m.deps.EditAt != nil {
		hint = "  {problem.edit} open at first error"
	}
	note := m.kx(fmt.Sprintf("%d-%d/%d  {problem.down}/{problem.up} scroll%s  {problem.back} close", min(o+1, len(lines)), end, len(lines), hint))
	return box(title, body, note, w, min(h, len(body)+2), true, st, m.borderStyle())
}

// fileLine matches compiler "file:line" locations.
var fileLine = regexp.MustCompile(`:\d+(:\d+)?:`)

// subBox is the Submission modal: where it was sent and the Verdict as it arrives.
func (m Model) subBox() []string {
	st := m.styles()
	s := m.sub
	w, _ := m.modalSize(84, 20)
	title := fmt.Sprintf("Submission %s", s.problem.Code())
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
	note := m.kx("{submission.close} close (tracking continues)")
	if s.final {
		note = m.kx("{submission.close} close")
	}
	return box(title, body, note, w, len(body)+2, true, st, m.borderStyle())
}
