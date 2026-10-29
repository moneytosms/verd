package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Snippet is one library entry: a name, a one-line description and the code to copy.
type Snippet struct{ Name, Desc, Code string }

// snippetPicker is the snippet library modal: type to filter, enter copies the selection.
// mode is "copy" (enter copies), "edit" (enter opens the selection in the editor) or "add" (enter
// creates a snippet named after the query).
type snippetPicker struct {
	lang  string
	query string
	sel   int
	items []Snippet
	mode  string
}

// matches are the snippets whose name or description fuzzy-matches the query, best first
// (a name match counts double).
func (p *snippetPicker) matches() []Snippet {
	q := strings.TrimSpace(p.query)
	if q == "" {
		return p.items
	}
	type hit struct {
		s     Snippet
		score int
	}
	var hits []hit
	for _, s := range p.items {
		best, ok := 0, false
		if sc, hitName := fuzzyScore(q, s.Name); hitName {
			best, ok = sc*2+20, true
		}
		if sc, hitDesc := fuzzyScore(q, s.Desc); hitDesc && (!ok || sc > best) {
			best, ok = sc, true
		}
		if ok {
			hits = append(hits, hit{s, best})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Snippet, len(hits))
	for i, h := range hits {
		out[i] = h.s
	}
	return out
}

func (m Model) openSnippets(lang, mode string) Model {
	if m.deps.Snippets == nil || (mode != "copy" && m.deps.EditSnippet == nil) {
		m.errMsg = "snippets are unavailable"
		return m
	}
	m.sp = &snippetPicker{lang: lang, items: m.deps.Snippets(lang), mode: mode}
	return m
}

func (m Model) updateSnippets(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := m.sp
	switch k := msg.String(); k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.sp = nil
	case "down", "ctrl+n":
		p.sel = min(p.sel+1, max(0, len(p.matches())-1))
	case "up", "ctrl+p":
		p.sel = max(0, p.sel-1)
	case "tab":
		if i := slices.Index(m.deps.Langs, p.lang); len(m.deps.Langs) > 0 {
			p.lang = m.deps.Langs[(i+1)%len(m.deps.Langs)]
			p.items, p.sel = m.deps.Snippets(p.lang), 0
		}
	case "backspace":
		if r := []rune(p.query); len(r) > 0 {
			p.query, p.sel = string(r[:len(r)-1]), 0
		}
	case "enter":
		if p.mode == "add" {
			name := strings.TrimSpace(p.query)
			if name == "" {
				return m, nil
			}
			m.sp = nil
			cmd, err := m.deps.EditSnippet(p.lang, name, true)
			return m.afterOpen(cmd, err, "snippet")
		}
		ms := p.matches()
		if p.sel >= len(ms) {
			return m, nil
		}
		if p.mode == "edit" {
			m.sp = nil
			cmd, err := m.deps.EditSnippet(p.lang, ms[p.sel].Name, false)
			return m.afterOpen(cmd, err, "snippet")
		}
		m.Notice = fmt.Sprintf("copied snippet %s (%s)", ms[p.sel].Name, p.lang)
		m.sp = nil
		return m, tea.SetClipboard(ms[p.sel].Code)
	default:
		if msg.Text != "" {
			p.query, p.sel = p.query+clean(msg.Text), 0
		}
	}
	return m, nil
}

// snippetBox renders the snippet modal: filter line, matching snippets, preview of the selection.
func (m Model) snippetBox() []string {
	st := m.styles()
	p := m.sp
	w, h := m.modalSize(100, 32)
	ms := p.matches()
	title, verb := "Snippets", "enter copy"
	switch p.mode {
	case "edit":
		title, verb = "Edit snippet", "enter edit"
	case "add":
		title, verb = "Add snippet", "enter create "+strings.TrimSpace(p.query)
		if strings.TrimSpace(p.query) == "" {
			verb = "type a name"
		}
	}
	p.sel = min(p.sel, max(0, len(ms)-1))
	listRows := max(3, (h-5)/2)
	body := []string{st.Accent.Render("> ") + p.query + st.Dim.Render("▏")}
	start := max(0, min(p.sel-listRows/2, len(ms)-listRows))
	for i := start; i < min(len(ms), start+listRows); i++ {
		cur := "  "
		if i == p.sel {
			cur = st.Accent.Render("> ")
		}
		body = append(body, cur+cell([]string{fmt.Sprintf("%-16s %s", ms[i].Name, st.Dim.Render(ms[i].Desc))}, 0, w-6))
	}
	if len(ms) == 0 {
		body = append(body, st.Dim.Render("  no snippet matches"))
	}
	body = append(body, st.Dim.Render(strings.Repeat("─", w-4)))
	if p.sel < len(ms) {
		code := cleanLines(ms[p.sel].Code)
		for i := 0; i < min(len(code), h-len(body)-3); i++ {
			body = append(body, cell(code, i, w-4))
		}
	}
	note := fmt.Sprintf("%s  %d/%d  %s  tab language  esc close", p.lang, min(p.sel+1, len(ms)), len(ms), verb)
	return box(title, body, note, w, min(h, len(body)+2), true, st, m.borderStyle())
}
