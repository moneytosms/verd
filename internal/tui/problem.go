package tui

import (
	"fmt"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/store"
)

// Case is one test shown in the Tests pane: a Sample Test or a Custom Test.
type Case struct {
	Name, Input, Want string
	Custom            bool
}

// caseRow is a Case with its latest Local Verdict, if a Test Run has reported it.
type caseRow struct {
	Case
	Res *runner.Result
}

// Panes of the split Problem view.
const (
	paneStatement = iota
	paneTests
	paneDetail
	panes
)

// loadCases rebuilds the test list: the statement's Samples, then the Custom Tests on disk.
func (m Model) loadCases() Model {
	m.cases = nil
	if m.detail != nil {
		for i, s := range m.detail.Samples {
			m.cases = append(m.cases, Case{Name: fmt.Sprintf("sample-%d", i+1), Input: s.Input, Want: s.Output})
		}
	}
	if m.deps.Customs != nil && m.open != nil {
		if cs, err := m.deps.Customs(*m.open); err == nil {
			for _, c := range cs {
				c.Custom = true
				m.cases = append(m.cases, c)
			}
		}
	}
	m.tsel = min(m.tsel, max(0, len(m.cases)-1))
	return m
}

// rows pairs every Case with its result from the latest Test Run.
func (m Model) rows() []caseRow {
	rows := make([]caseRow, len(m.cases))
	seen := map[string]bool{}
	for i, c := range m.cases {
		rows[i].Case = c
		seen[c.Name] = true
		if m.run != nil {
			for j := range m.run.results {
				if m.run.results[j].Name == c.Name {
					rows[i].Res = &m.run.results[j]
				}
			}
		}
	}
	if m.run != nil { // a result for a test no longer listed (e.g. deleted mid-run)
		for j := range m.run.results {
			if r := &m.run.results[j]; !seen[r.Name] {
				rows = append(rows, caseRow{Case: Case{Name: r.Name, Want: r.Expected}, Res: r})
			}
		}
	}
	return rows
}

// selected is the highlighted row of the Tests pane.
func (m Model) selected() (caseRow, bool) {
	rows := m.rows()
	if m.tsel < 0 || m.tsel >= len(rows) {
		return caseRow{}, false
	}
	return rows[m.tsel], true
}

// split reports whether the Problem view has room for the two-column layout, and its column widths.
func (m Model) split() (left, right int, ok bool) {
	w := m.contentWidth()
	if w < 100 || m.height < 16 {
		return 0, 0, false
	}
	right = min(58, max(42, w*2/5))
	return w - right - 1, right, true
}

// stmtWidth is the width the statement is wrapped to.
func (m Model) stmtWidth() int {
	if l, _, ok := m.split(); ok {
		return l - 4
	}
	return m.contentWidth() - 4
}

// paneHeight is the height of the split view's panes.
func (m Model) paneHeight() int { return max(6, m.height-5) }

// stmtLines is the statement as shown in the split view's left pane.
func (m Model) stmtLines() []string {
	lines := make([]string, 0, len(m.body)+2)
	for _, l := range m.body {
		lines = append(lines, stripHeadingMarks(l))
	}
	return lines
}

// stripHeadingMarks turns glamour's "### Input" into "Input": the colour already marks a heading.
func stripHeadingMarks(l string) string {
	i := strings.Index(l, "### ")
	if i < 0 || strings.TrimSpace(xansiStrip(l[:i])) != "" {
		return l
	}
	return l[:i] + l[i+4:]
}

// viewSplit draws the two-column Problem view and returns the footer hints.
func (m Model) viewSplit(b *strings.Builder, lw, rw int) string {
	st := m.styles()
	h := m.paneHeight()
	p := *m.open

	// left: the statement
	body := m.stmtLines()
	if m.loading {
		body = append(body, "", "loading...")
	}
	if m.errMsg != "" {
		body = append(body, "", st.Bad.Render(clean(m.errMsg)))
	}
	inner := h - 2
	off := min(m.scroll, max(0, len(body)-inner))
	end := min(len(body), off+inner)
	note := ""
	if len(body) > inner {
		note = fmt.Sprintf("%d%%", 100*end/len(body))
	}
	title := fmt.Sprintf("%d%s  %s", p.ContestID, clean(p.Index), clean(p.Name))
	left := box(title, body[off:end], note, lw, h, m.pane == paneStatement, st)

	// right: info, tests, detail
	info := m.infoLines(rw - 4)
	infoH := min(len(info)+2, max(5, h/3))
	rows := m.rows()
	testsH := min(max(len(rows)+3+m.compileLines(), 5), max(5, (h-infoH)/2))
	detH := h - infoH - testsH
	if detH < 4 { // very short terminal: drop the detail pane
		detH, testsH = 0, h-infoH
	}
	var right []string
	right = append(right, box("Problem", info, "", rw, infoH, false, st)...)
	tl, tnote := m.testsLines(rw-4, testsH-2)
	right = append(right, box("Tests ["+m.tag()+"]", tl, tnote, rw, testsH, m.pane == paneTests, st)...)
	if detH > 0 {
		dl, dtitle := m.detailLines(rw - 4)
		if m.strs != nil {
			dl, dtitle = m.stressPanel(), "Stress"
		}
		off := min(m.detScroll, max(0, len(dl)-(detH-2)))
		dnote := ""
		if len(dl) > detH-2 {
			dnote = fmt.Sprintf("%d/%d", off+1, len(dl))
		}
		end := min(len(dl), off+detH-2)
		right = append(right, box(dtitle, dl[off:end], dnote, rw, detH, m.pane == paneDetail, st)...)
	}
	for _, l := range joinCols(left, lw, right, " ") {
		b.WriteString(l + "\n")
	}
	return "tab pane  j/k move  e edit  T tests  t run  s submit  S stress  c mode  l lang  o browser  esc back  ? help  q quit"
}

// infoLines is the Problem box: rating, limits, tags, language and Submission status.
func (m Model) infoLines(w int) []string {
	st := m.styles()
	p := *m.open
	rating := "unrated"
	if p.Rating > 0 {
		rating = fmt.Sprint(p.Rating)
	}
	mark := ""
	switch m.statusOf(p) {
	case store.StatusSolved:
		mark = "  " + st.Good.Render("✓ solved")
	case store.StatusAttempted:
		mark = "  " + st.Warn.Render("~ attempted")
	}
	lines := []string{st.Dim.Render("rating ") + rating + mark}
	if d := m.detail; d != nil {
		lim := fmt.Sprintf("%.4g s · %d MB", float64(d.TimeLimitMS)/1000, d.MemoryLimitMB)
		if d.Interactive {
			lim += " · interactive"
		}
		lines = append(lines, st.Dim.Render("limits ")+lim)
	}
	if len(p.Tags) > 0 {
		cur := st.Dim.Render("tags   ")
		first := true
		for _, t := range p.Tags {
			chip := clean(t)
			if !first && lenPlain(cur)+len(chip)+3 > w {
				lines = append(lines, cur)
				cur = "       "
				first = true
			}
			if !first {
				cur += st.Dim.Render(" · ")
			}
			cur += st.Accent.Render(chip)
			first = false
		}
		lines = append(lines, cur)
	}
	for _, l := range m.submissionLines() {
		lines = append(lines, l)
	}
	return lines
}

func lenPlain(s string) int { return len([]rune(xansiStrip(s))) }

// compileLines is how many lines the compile status takes in the Tests pane.
func (m Model) compileLines() int {
	if m.run == nil || m.run.compile == "" {
		return 0
	}
	return min(5, 1+strings.Count(strings.TrimRight(m.run.compile, "\n"), "\n"))
}

// testsLines lists every test with its verdict, scrolled to keep the selection visible.
func (m Model) testsLines(w, h int) ([]string, string) {
	st := m.styles()
	r := m.run
	var lines []string
	note := ""
	if r != nil {
		switch {
		case r.running:
			note = "running..."
		case r.overall != "":
			note = strings.TrimSpace(r.overall)
		}
		if r.compile != "" {
			for i, l := range strings.Split(strings.TrimRight(r.compile, "\n"), "\n") {
				if i == 0 && (r.compile == "compiling..." || strings.HasPrefix(r.compile, "ok")) {
					lines = append(lines, st.Dim.Render("compile: "+r.compile))
					break
				}
				if i == 0 {
					lines = append(lines, "compile: "+m.verdictStyle("CE"))
				}
				if len(lines) < 5 {
					lines = append(lines, st.Dim.Render(clean(l)))
				}
			}
		}
	}
	rows := m.rows()
	if len(rows) == 0 {
		lines = append(lines, st.Dim.Render("no tests yet: T to add one"))
	}
	var list []string
	passed := 0
	for i, row := range rows {
		cur := "  "
		if i == m.tsel {
			cur = st.Accent.Render("> ")
			if m.pane != paneTests {
				cur = st.Dim.Render("> ")
			}
		}
		kind := st.Dim.Render("sample")
		if row.Custom {
			kind = st.Warn.Render("custom")
		}
		line := fmt.Sprintf("%s%-10s %s", cur, row.Name, kind)
		if row.Res != nil {
			line = fmt.Sprintf("%s%-10s %s %5d ms %6.1f MB", cur, row.Name, m.verdictStyle(row.Res.Verdict), row.Res.TimeMS, row.Res.MemoryMB)
			if row.Res.Verdict == runner.AC {
				passed++
			}
		}
		list = append(list, line)
	}
	room := max(1, h-len(lines))
	start := max(0, min(m.tsel-room/2, len(list)-room))
	end := min(len(list), start+room)
	lines = append(lines, list[start:end]...)
	if r != nil && !r.running && len(r.results) > 0 {
		note = fmt.Sprintf("%d/%d AC", passed, len(r.results))
		if r.overall != "" && r.overall != runner.AC {
			note += " " + strings.TrimSpace(r.overall)
		}
	}
	return lines, note
}

// detailLines shows the selected test: input, expected, actual output and any mismatch.
func (m Model) detailLines(w int) ([]string, string) {
	st := m.styles()
	row, ok := m.selected()
	if !ok {
		return []string{st.Dim.Render("select a test")}, "Test"
	}
	title := row.Name
	var lines []string
	section := func(name string, text string, bad int) {
		lines = append(lines, st.Accent.Render(name))
		ls := cleanLines(text)
		if strings.TrimSpace(text) == "" {
			lines = append(lines, st.Dim.Render("  (empty)"))
		}
		for i, l := range ls {
			if strings.TrimSpace(text) == "" {
				break
			}
			if i == bad {
				l = st.Bad.Render(l)
			}
			lines = append(lines, "  "+l)
		}
	}
	bad := -1
	if res := row.Res; res != nil {
		title += "  " + strings.TrimSpace(res.Verdict)
		if res.Mismatch != nil {
			bad = res.Mismatch.Line - 1
		}
	}
	section("Input", row.Input, -1)
	section("Expected", row.Want, bad)
	if res := row.Res; res != nil {
		section("Output", res.Output, bad)
		if res.Mismatch != nil {
			mm := res.Mismatch
			lines = append(lines, st.Bad.Render(fmt.Sprintf("line %d col %d: want %q got %q", mm.Line, mm.Col, clip(clean(mm.Want), 24), clip(clean(mm.Got), 24))))
		} else if res.Note != "" {
			lines = append(lines, st.Warn.Render(clean(res.Note)))
		}
		if strings.TrimSpace(res.Stderr) != "" {
			section("Stderr", res.Stderr, -1)
		}
	} else {
		lines = append(lines, "", st.Dim.Render("press t to run"))
	}
	return lines, title
}

func xansiStrip(s string) string { return xansi.Strip(s) }
