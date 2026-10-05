package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// testMgr is the test manager modal: browse, add, edit, copy and delete tests.
type testMgr struct {
	sel        int
	confirmDel bool
	ed         *caseEditor
	note       string
}

// caseEditor edits one Custom Test's input and expected output in place.
type caseEditor struct {
	name     string // "" while adding a new test
	field    int    // 0 input, 1 expected
	buf      [2][][]rune
	row, col int
}

func newEditor(name, input, want string) *caseEditor {
	e := &caseEditor{name: name}
	for i, t := range []string{input, want} {
		for _, l := range strings.Split(strings.TrimSuffix(t, "\n"), "\n") {
			e.buf[i] = append(e.buf[i], []rune(l))
		}
	}
	return e
}

func (e *caseEditor) text(i int) string {
	var ls []string
	for _, l := range e.buf[i] {
		ls = append(ls, string(l))
	}
	t := strings.TrimRight(strings.Join(ls, "\n"), "\n")
	if t == "" {
		return ""
	}
	return t + "\n"
}

func (e *caseEditor) line() *[]rune { return &e.buf[e.field][e.row] }

func (e *caseEditor) clamp() {
	e.row = max(0, min(e.row, len(e.buf[e.field])-1))
	e.col = max(0, min(e.col, len(*e.line())))
}

func (e *caseEditor) insert(s string) {
	for _, r := range s {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			l := *e.line()
			rest := append([]rune(nil), l[e.col:]...)
			*e.line() = l[:e.col]
			e.buf[e.field] = append(e.buf[e.field][:e.row+1], append([][]rune{rest}, e.buf[e.field][e.row+1:]...)...)
			e.row, e.col = e.row+1, 0
			continue
		}
		l := *e.line()
		*e.line() = append(l[:e.col], append([]rune{r}, l[e.col:]...)...)
		e.col++
	}
}

// key applies one key press; it reports "save", "cancel" or "".
func (e *caseEditor) key(msg tea.KeyPressMsg) string {
	e.clamp()
	switch k := msg.String(); k {
	case "ctrl+s":
		return "save"
	case "esc":
		return "cancel"
	case "tab", "shift+tab":
		e.field = 1 - e.field
		e.row, e.col = 0, 0
	case "enter":
		e.insert("\n")
	case "backspace":
		switch {
		case e.col > 0:
			l := *e.line()
			*e.line() = append(l[:e.col-1], l[e.col:]...)
			e.col--
		case e.row > 0:
			prev := e.buf[e.field][e.row-1]
			e.col = len(prev)
			e.buf[e.field][e.row-1] = append(prev, *e.line()...)
			e.buf[e.field] = append(e.buf[e.field][:e.row], e.buf[e.field][e.row+1:]...)
			e.row--
		}
	case "delete":
		if l := *e.line(); e.col < len(l) {
			*e.line() = append(l[:e.col], l[e.col+1:]...)
		}
	case "left":
		e.col--
	case "right":
		e.col++
	case "up":
		e.row--
	case "down":
		e.row++
	case "home", "ctrl+a":
		e.col = 0
	case "end", "ctrl+e":
		e.col = len(*e.line())
	case "ctrl+u":
		*e.line() = (*e.line())[e.col:]
		e.col = 0
	default:
		if msg.Text != "" {
			e.insert(clean(msg.Text))
		}
	}
	e.clamp()
	return ""
}

// openTM opens the test manager, optionally straight into adding a new test.
func (m Model) openTM(add bool) Model {
	m.tm = &testMgr{sel: m.tsel}
	if add {
		m.tm.ed = newEditor("", "", "")
	}
	return m
}

func (m Model) updateTM(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	t := m.tm
	if k := msg.String(); k == "ctrl+c" {
		return m, tea.Quit
	} else if t.ed != nil {
		switch t.ed.key(msg) {
		case "cancel":
			t.ed = nil
		case "save":
			p, e := *m.open, t.ed
			if m.deps.SaveCase == nil {
				t.note, t.ed = "saving is unavailable", nil
				break
			}
			name, err := m.deps.SaveCase(p, e.name, e.text(0), e.text(1))
			if err != nil {
				t.note = "save: " + err.Error()
				break
			}
			t.ed, t.note = nil, "saved "+name
			m = m.loadCases()
			for i, c := range m.cases {
				if c.Name == name {
					t.sel, m.tsel = i, i
				}
			}
		}
		return m, nil
	}
	if t.confirmDel {
		t.confirmDel = false
		if msg.String() == "y" && t.sel < len(m.cases) && m.cases[t.sel].Custom && m.deps.DeleteCase != nil {
			name := m.cases[t.sel].Name
			if err := m.deps.DeleteCase(*m.open, name); err != nil {
				t.note = "delete: " + err.Error()
			} else {
				t.note = "deleted " + name
				m = m.loadCases()
				t.sel, m.tsel = min(t.sel, max(0, len(m.cases)-1)), min(t.sel, max(0, len(m.cases)-1))
			}
		}
		return m, nil
	}
	t.note = ""
	cur := Case{}
	if t.sel < len(m.cases) {
		cur = m.cases[t.sel]
	}
	switch msg.String() {
	case "q", "esc":
		m.tm = nil
	case "j", "down":
		t.sel = min(t.sel+1, len(m.cases)-1)
		m.tsel = max(0, t.sel)
	case "k", "up":
		t.sel = max(0, t.sel-1)
		m.tsel = t.sel
	case "a":
		t.ed = newEditor("", "", "")
	case "e", "enter":
		switch {
		case len(m.cases) == 0:
		case !cur.Custom:
			t.note = "samples are read-only: press c to copy it into an editable Custom Test"
		default:
			t.ed = newEditor(cur.Name, cur.Input, cur.Want)
		}
	case "c":
		if len(m.cases) > 0 {
			t.ed = newEditor("", cur.Input, cur.Want)
		}
	case "d", "x":
		switch {
		case len(m.cases) == 0:
		case !cur.Custom:
			t.note = "samples cannot be deleted"
		default:
			t.confirmDel = true
		}
	case "E":
		if m.deps.AddCustom != nil {
			m.tm = nil
			cmd, err := m.deps.AddCustom(*m.open)
			return m.afterOpen(cmd, err, "add test")
		}
	case "t":
		m.tm = nil
		nm, cmd := m.startTests()
		return nm, cmd
	}
	return m, nil
}

// tmBox renders the test manager modal.
func (m Model) tmBox() []string {
	st := m.styles()
	t := m.tm
	w := min(m.width-4, 112)
	h := min(m.height-4, 30)
	if t.ed != nil {
		longest := max(len(t.ed.buf[0]), len(t.ed.buf[1]))
		return m.editorBox(w, min(h, max(12, longest+4)))
	}
	lw := min(30, w/3)
	rows := m.rows()
	var list []string
	for i, r := range rows {
		cur := "  "
		if i == t.sel {
			cur = st.Accent.Render("> ")
		}
		kind := st.Dim.Render("sample")
		if r.Custom {
			kind = st.Warn.Render("custom")
		}
		v := "  "
		if r.Res != nil {
			v = m.verdictStyle(r.Res.Verdict)
		}
		list = append(list, fmt.Sprintf("%s%-9s %s %s", cur, r.Name, kind, v))
	}
	if len(list) == 0 {
		list = []string{st.Dim.Render("no tests: press a to add")}
	}
	inner := h - 2
	start := max(0, min(t.sel-inner/2, len(list)-inner))
	list = list[start:min(len(list), start+inner)]

	var det []string
	if t.sel < len(rows) {
		r := rows[t.sel]
		sec := func(name, text string) {
			det = append(det, st.Accent.Render(name))
			if strings.TrimSpace(text) == "" {
				det = append(det, st.Dim.Render("  (empty)"))
				return
			}
			for _, l := range cleanLines(text) {
				det = append(det, "  "+l)
			}
		}
		sec("Input", r.Input)
		sec("Expected", r.Want)
		if r.Res != nil {
			sec("Output", r.Res.Output)
		}
	}
	note := "a add  e edit  c copy  d delete  E in editor  t run  q close"
	switch {
	case t.confirmDel:
		note = st.Warn.Render("delete " + m.cases[t.sel].Name + "? y/n")
	case t.note != "":
		note = st.Warn.Render(t.note)
	}
	body := joinCols(list, lw, det, st.Dim.Render(" │ "))
	body = body[:min(len(body), inner)]
	return box("Tests", body, note, w, min(h, max(12, len(body)+2)), true, st)
}

// editorBox renders the in-place Custom Test editor.
func (m Model) editorBox(w, h int) []string {
	st := m.styles()
	e := m.tm.ed
	pw := (w - 4 - 3) / 2
	render := func(i int) []string {
		var out []string
		head := "Input"
		if i == 1 {
			head = "Expected output"
		}
		if i == e.field {
			head = st.Accent.Render("▌" + head)
		} else {
			head = st.Dim.Render(" " + head)
		}
		out = append(out, head)
		rows := h - 4
		top := 0
		if i == e.field && e.row >= rows {
			top = e.row - rows + 1
		}
		for r := top; r < min(len(e.buf[i]), top+rows); r++ {
			l := e.buf[i][r]
			if i == e.field && r == e.row {
				c := min(e.col, len(l))
				ch := " "
				if c < len(l) {
					ch = string(l[c])
				}
				pre := string(l[:c])
				post := ""
				if c < len(l) {
					post = string(l[c+1:])
				}
				out = append(out, clean(pre)+"\x1b[7m"+clean(ch)+"\x1b[27m"+clean(post))
				continue
			}
			out = append(out, clean(string(l)))
		}
		return out
	}
	body := joinCols(render(0), pw, render(1), st.Dim.Render(" │ "))
	title := "Add test"
	if e.name != "" {
		title = "Edit " + e.name
	}
	note := "tab switch field  ctrl+s save  esc cancel"
	if m.tm.note != "" {
		note = st.Warn.Render(m.tm.note)
	}
	return box(title, body, note, w, h, true, st)
}
