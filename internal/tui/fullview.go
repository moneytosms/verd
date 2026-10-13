package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/runner"
)

// fullRow is one visual row of the full-screen test view.
type fullRow struct {
	text      string
	bad, head bool
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

// fullRows lays out the four sections for a content width cw (wrapped, or left as is when
// wrap is off) and returns the first row of each section.
func (m Model) fullRows(row caseRow, cw int, wrap bool) (rows []fullRow, starts [4]int) {
	texts := [4]string{row.Input, row.Want, "", ""}
	var bw, bg []bool
	if res := row.Res; res != nil {
		texts[2], texts[3] = res.Output, res.Stderr
		if res.Verdict != runner.AC {
			bw, bg = diffLines(runner.Mode(m.mode()), cleanLines(row.Want), cleanLines(res.Output))
		}
	}
	for s, name := range fullSecs {
		starts[s] = len(rows)
		rows = append(rows, fullRow{text: name, head: true})
		if strings.TrimSpace(texts[s]) == "" {
			note := "(empty)"
			if s == 2 && row.Res == nil {
				note = "(not run)"
			}
			rows = append(rows, fullRow{text: note})
			continue
		}
		var bad []bool
		switch s {
		case 1:
			bad = bw
		case 2:
			bad = bg
		}
		for i, l := range cleanLines(texts[s]) {
			b := i < len(bad) && bad[i]
			r := []rune(l)
			for first := true; first || (wrap && len(r) > 0); first = false {
				n := len(r)
				if wrap {
					n = min(n, cw)
				}
				rows = append(rows, fullRow{text: string(r[:n]), bad: b})
				r = r[n:]
			}
		}
	}
	return
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
	if res := row.Res; res != nil {
		title += "  " + strings.TrimSpace(res.Verdict) + fmt.Sprintf("  %d ms  %.1f MB", res.TimeMS, res.MemoryMB)
		switch {
		case res.Mismatch != nil:
			mm := res.Mismatch
			head = append(head, st.Bad.Render(fmt.Sprintf("first difference: line %d col %d: want %q got %q", mm.Line, mm.Col, clip(clean(mm.Want), 30), clip(clean(mm.Got), 30))))
		case res.Note != "":
			head = append(head, st.Warn.Render(clean(res.Note)))
		}
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
	body := head
	for _, r := range rows[off:end] {
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
		default:
			body = append(body, "  "+string(t))
		}
	}
	wrap := "wrap"
	if m.fullWrap {
		wrap = "pan"
	}
	note := m.kx(fmt.Sprintf("%d-%d/%d  {problem.down}/{problem.up} scroll  h/l pan  w %s  1-4 section  y copy  r rerun  {problem.back} close", min(off+1, len(rows)), end, len(rows), wrap))
	return box(title, body, note, w, min(h, len(body)+2), true, st, m.borderStyle())
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
	case "1", "2", "3", "4":
		if row, ok := m.selected(); ok {
			w, _ := m.modalSize(m.width, m.height)
			_, starts := m.fullRows(row, max(4, w-6), m.fullWrap)
			m.fullOff = starts[int(k[0]-'1')]
		}
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
	case "r": // the runner runs the whole list; the view updates as results arrive
		return m.startTests()
	case "ctrl+c":
		return m.stopTests(), tea.Quit
	}
	return m, nil
}
