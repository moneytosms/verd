package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/runner"
)

// fullRow is one visual row of the full-screen test view.
type fullRow struct {
	text      string
	bad, head bool
	note      bool // an inline hint under a wrong line (what was expected)
	tone      byte // stderr coloring, see stderrTones
}

var fullSecs = [4]string{"Input", "Expected", "Output", "Stderr (never compared)"}

// diffLines marks the lines of want and got that differ under mode, comparing line by line
// with the runner's own Compare so Tokens/Exact/Float behave as in the verdict.
func diffLines(mode runner.Mode, want, got []string) (badWant, badGot []bool) {
	badWant, badGot = make([]bool, len(want)), make([]bool, len(got))
	if mode == runner.None {
		return
	}
	for i := 0; i < max(len(want), len(got)); i++ {
		if i >= len(want) || i >= len(got) {
			if i < len(want) && strings.TrimSpace(want[i]) != "" {
				badWant[i] = true
			}
			if i < len(got) && strings.TrimSpace(got[i]) != "" {
				badGot[i] = true
			}
			continue
		}
		if ok, _ := runner.Compare(mode, want[i], got[i], 1e-6); !ok {
			badWant[i], badGot[i] = true, true
		}
	}
	return
}

// result is the row's result, or while a Live run has the row's test going, its partial output.
func (m Model) result(row caseRow) *runner.Result {
	if r := m.run; r != nil && r.running && r.live != nil && r.live.Name == row.Name {
		return r.live
	}
	return row.Res
}

// fullRows lays out the four sections for a content width cw (wrapped, or left as is when
// wrap is off) and returns the first row of each section.
func (m Model) fullRows(row caseRow, cw int, wrap bool) (rows []fullRow, starts [4]int) {
	texts, bw, bg, tones := m.fullTexts(row)
	for s, name := range fullSecs {
		starts[s] = len(rows)
		rows = append(rows, fullRow{text: name, head: true})
		if strings.TrimSpace(texts[s]) == "" {
			note := "(empty)"
			if s == 2 && m.result(row) == nil {
				note = "(not run)"
			}
			rows = append(rows, fullRow{text: note})
			continue
		}
		var bad []bool
		var tone []byte
		switch s {
		case 1:
			bad = bw
		case 2:
			bad = bg
		case 3:
			tone = tones
		}
		rows = append(rows, fullSectionRows(row, s, texts[s], bad, tone, cw, wrap)...)
	}
	return
}

// fullSectionRows lays out one section's text, wrapped to cw when wrap. A wrong Output line is
// followed by the Expected line it should match.
func fullSectionRows(row caseRow, s int, text string, bad []bool, tone []byte, cw int, wrap bool) []fullRow {
	var rows []fullRow
	for i, l := range cleanLines(text) {
		b := i < len(bad) && bad[i]
		var tn byte
		if i < len(tone) {
			tn = tone[i]
		}
		r := []rune(l)
		for first := true; first || (wrap && len(r) > 0); first = false {
			n := len(r)
			if wrap {
				n = min(n, cw)
			}
			rows = append(rows, fullRow{text: string(r[:n]), bad: b, tone: tn})
			r = r[n:]
		}
		if b && s == 2 {
			w := cleanLines(row.Want)
			hint := "(not in expected)"
			if i < len(w) {
				hint = "want: " + w[i]
			}
			rows = append(rows, fullRow{text: "  " + hint, note: true})
		}
	}
	return rows
}

// fullTexts is the four sections' text for row, stderr folded unless raw. bw and bg mark the lines
// of Expected and Output that differ; tones colors the stderr lines.
func (m Model) fullTexts(row caseRow) (texts [4]string, bw, bg []bool, tones []byte) {
	texts = [4]string{row.Input, row.Want, "", ""}
	if res := m.result(row); res != nil {
		texts[2], texts[3] = res.Output, res.Stderr
		if !m.fullRaw {
			texts[3] = runner.Fold(res.Stderr)
		}
		sol, _, _ := strings.Cut(runner.Explain(*res).Loc, ":")
		tones = stderrTones(cleanLines(texts[3]), sol)
		if res.Verdict != "" && res.Verdict != runner.AC { // "" is a test still running
			bw, bg = diffLines(runner.Mode(m.mode()), cleanLines(row.Want), cleanLines(res.Output))
		}
	}
	return texts, bw, bg, tones
}

// fullBox is the selected test (input, expected, stdout, stderr) in a near full-screen modal.
// Lines that differ from Expected are marked; stderr is shown but never compared.
func (m Model) fullBox() []string {
	st := m.styles()
	w, h := m.modalSize(m.width, m.height)
	row, ok := m.selected()
	if !ok {
		return box("Test", []string{st.Dim.Render("select a test")}, "", w, 3, true, st, m.borderStyle())
	}
	cw := max(4, w-4-2)
	rows, _ := m.fullRows(row, cw, m.fullWrap)
	title := row.Name
	var head []string
	if res := m.result(row); res != nil {
		title += "  " + strings.TrimSpace(res.Verdict) + fmt.Sprintf("  %d ms  %.1f MB", res.TimeMS, res.MemoryMB)
		head = m.fullHead(row, res, cw)
	}
	if m.run != nil && m.run.running {
		title += "  running..."
	}
	room := max(1, h-2-len(head))
	maxLen := 0
	for _, r := range rows {
		maxLen = max(maxLen, len([]rune(r.text)))
	}
	off, x := min(m.fullOff, max(0, len(rows)-room)), min(m.fullX, max(0, maxLen-cw))
	if m.fullWrap {
		x = 0
	}
	end := min(len(rows), off+room)
	body := append(head, m.fullLines(rows[off:end], x, cw)...)
	return box(title, body, m.fullNote(off, end, len(rows)), w, min(h, len(body)+2), true, st, m.borderStyle())
}

// fullHead is the result's lines above the rows: the mismatch or note, the detail, then the why.
func (m Model) fullHead(row caseRow, res *runner.Result, cw int) []string {
	st := m.styles()
	var head []string
	switch {
	case res.Mismatch != nil:
		head = append(head, st.Bad.Render(m.fullMismatch(row, res)))
	case res.Note != "":
		head = append(head, st.Warn.Render(clean(res.Note)))
	}
	if d := res.Detail(); d != "" {
		head = append(head, st.Bad.Render(clean(d)))
	}
	return append(head, m.fullWhy(res, cw)...)
}

// fullMismatch is the first differing line and how many lines differ.
func (m Model) fullMismatch(row caseRow, res *runner.Result) string {
	mm := res.Mismatch
	_, bg := diffLines(runner.Mode(m.mode()), cleanLines(row.Want), cleanLines(res.Output))
	n := 0
	for _, b := range bg {
		if b {
			n++
		}
	}
	return fmt.Sprintf("first difference: line %d col %d (%d/%d lines differ): want %q got %q", mm.Line, mm.Col, n, len(bg), clip(clean(mm.Want), 30), clip(clean(mm.Got), 30))
}

// fullWhy is why the test failed: the hint, the failing location with its code, and the trace.
func (m Model) fullWhy(res *runner.Result, cw int) []string {
	st := m.styles()
	x := runner.Explain(*res)
	if x.Hint == "" {
		return nil
	}
	var head []string
	for i, l := range strings.Split(xansi.Wordwrap(clean(x.Hint), cw-5, ""), "\n") {
		head = append(head, st.Warn.Render([]string{"why: ", "     "}[min(i, 1)]+l))
	}
	if x.Loc != "" {
		file, _, _ := strings.Cut(x.Loc, ":")
		at := st.Warn.Render("at:  ") + st.Accent.Render(filepath.Base(clean(x.Loc)))
		if x.Code != "" {
			at += st.Dim.Render("  │ ") + hlCode(st, file, clean(x.Code))
		}
		head = append(head, at)
	}
	if x.Trace != "" {
		head = append(head, st.Warn.Render("via: ")+st.Dim.Render(clean(x.Trace)))
	}
	return head
}

// fullLines styles the visible rows: panned left by x and cut to cw.
func (m Model) fullLines(rows []fullRow, x, cw int) []string {
	st := m.styles()
	var body []string
	for _, r := range rows {
		t := []rune(r.text)
		t = t[min(x, len(t)):]
		if len(t) > cw {
			t = t[:cw]
		}
		switch {
		case r.head:
			body = append(body, st.Accent.Render(string(t)))
		case r.bad:
			body = append(body, st.Bad.Render("! "+string(t)))
		case r.note, r.tone == 'f':
			body = append(body, st.Dim.Render("  "+string(t)))
		case r.tone == 'e':
			body = append(body, st.Bad.Render("  "+string(t)))
		case r.tone == 's':
			body = append(body, st.Accent.Render("  "+string(t)))
		default:
			body = append(body, "  "+string(t))
		}
	}
	return body
}

// fullNote is the modal's footer: the rows shown out of n, and the keys.
func (m Model) fullNote(off, end, n int) string {
	wrap := "wrap"
	if m.fullWrap {
		wrap = "pan"
	}
	raw := "raw stderr"
	if m.fullRaw {
		raw = "fold stderr"
	}
	return m.kx(fmt.Sprintf("%d-%d/%d  {full.down}/{full.up} scroll  {full.pan_left}/{full.pan_right} pan  {full.wrap} %s  {full.raw} %s  {full.next}/{full.prev} test  {full.copy} copy  {full.rerun} rerun  {full.close} close", min(off+1, n), end, n, wrap, raw))
}

// fullSection is the section the top visible row belongs to.
func (m Model) fullSection(row caseRow) int {
	w, _ := m.modalSize(m.width, m.height)
	_, starts := m.fullRows(row, max(4, w-6), m.fullWrap)
	s := 0
	for i, st := range starts {
		if st <= m.fullOff {
			s = i
		}
	}
	return s
}

// updateFull handles keys while the full-screen test view is open.
func (m Model) updateFull(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k := msg.String(); k {
	case "esc", "f", "q":
		m.full, m.fullOff, m.fullX = false, 0, 0
	case "j", "down":
		m.fullOff++
	case "k", "up":
		m.fullOff = max(0, m.fullOff-1)
	case "pgdown":
		m.fullOff += m.page()
	case "pgup":
		m.fullOff = max(0, m.fullOff-m.page())
	case "home", "g":
		m.fullOff = 0
	case "end", "G":
		m.fullOff = 1 << 30 // clamped when drawn
	case "l", "right":
		m.fullX += 8
	case "h", "left":
		m.fullX = max(0, m.fullX-8)
	case "w":
		m.fullWrap, m.fullX = !m.fullWrap, 0
	case "z":
		m.fullRaw = !m.fullRaw
	case "n", "p": // next or previous test, from the top
		d := 1
		if k == "p" {
			d = -1
		}
		m.tsel = max(0, min(m.tsel+d, len(m.rows())-1))
		m.fullOff, m.fullX = 0, 0
	case "y":
		row, ok := m.selected()
		if !ok {
			break
		}
		s := m.fullSection(row)
		text := [4]string{row.Input, row.Want, "", ""}[s]
		if row.Res != nil {
			text = [4]string{row.Input, row.Want, row.Res.Output, row.Res.Stderr}[s]
		}
		if strings.TrimSpace(text) == "" {
			m.Notice = "nothing to copy"
			break
		}
		m.Notice = "copied " + strings.ToLower(strings.Fields(fullSecs[s])[0])
		return m, tea.SetClipboard(strings.TrimRight(text, "\n") + "\n")
	case "r": // just this test; the other results stay
		if row, ok := m.selected(); ok {
			return m.runTests([]string{row.Name})
		}
	case "ctrl+c":
		return m.stopTests(), tea.Quit
	}
	return m, nil
}

var errLine = regexp.MustCompile(`Error:|ERROR:|runtime error:|^SUMMARY:|^Traceback|^\w+(Error|Exception)\b|terminate called|Assertion`)

// stderrTones colors stderr lines: 'e' the error message (a debug-mode "Error:" paragraph runs to the
// blank line), 's' a line naming the Solution's file sol, 'f' folded noise and verd's own notes.
func stderrTones(lines []string, sol string) []byte {
	t := make([]byte, len(lines))
	para := false
	for i, l := range lines {
		para = para && strings.TrimSpace(l) != ""
		switch {
		case strings.HasPrefix(l, "⋯ ") || strings.HasPrefix(l, "[verd:"):
			t[i] = 'f'
		case para || errLine.MatchString(l):
			t[i] = 'e'
			para = para || strings.HasPrefix(l, "Error: ")
		case sol != "" && strings.Contains(l, sol):
			t[i] = 's'
		}
	}
	return t
}
