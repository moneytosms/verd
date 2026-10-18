package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/editorial"
	"github.com/moneytosms/verd/internal/scrape"
)

// editorialView is the editorial modal: the post's part for one problem, its spoilers opened one
// at a time and strictly in order.
type editorialView struct {
	p        cf.Problem
	loading  bool
	err      string
	res      *editorial.Result
	revealed int // spoilers opened so far
	scroll   int
	cache    map[int][]string // block index -> rendered lines
	cacheW   int
	marks    []int // line of each spoiler's heading in the last layout
}

type editorialMsg struct {
	p   cf.Problem
	res *editorial.Result
	err error
}

// openEditorial shows a problem's editorial, hidden while a contest session runs on that contest.
func (m Model) openEditorial(p cf.Problem) (Model, tea.Cmd) {
	switch {
	case m.deps.Editorial == nil:
		return m, nil
	case m.isBlind(p):
		m.Notice = "the editorial is hidden during the session"
		return m, nil
	case p.Source() != cf.SourceCF:
		m.Notice = "only Codeforces problems have editorials"
		return m, nil
	}
	m.edv = &editorialView{p: p, loading: true}
	fetch := m.deps.Editorial
	return m, func() tea.Msg {
		res, err := fetch(p)
		return editorialMsg{p, res, err}
	}
}

func (m Model) onEditorial(msg editorialMsg) Model {
	ev := m.edv
	if ev == nil || ev.p.ContestID != msg.p.ContestID || ev.p.Index != msg.p.Index {
		return m // closed or replaced meanwhile
	}
	ev.loading = false
	if msg.err != nil {
		ev.err = clean(msg.err.Error())
		return m
	}
	ev.res, ev.err = msg.res, ""
	return m
}

func (m Model) updateEditorial(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	ev := m.edv
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.edv = nil
	case "j", "down":
		ev.scroll++
	case "k", "up":
		ev.scroll = max(0, ev.scroll-1)
	case "pgdown":
		ev.scroll += m.page()
	case "pgup":
		ev.scroll = max(0, ev.scroll-m.page())
	case "home", "g":
		ev.scroll = 0
	case "end", "G":
		ev.scroll = 1 << 20 // clamped when drawn
	case "enter", " ", "n":
		if ev.res != nil && ev.revealed < ev.res.Spoilers() {
			ev.revealed++
			ev.scroll = -1 // jump to the heading just opened, set when drawn
		}
	case "o":
		if ev.res != nil && ev.res.URL != "" && m.deps.OpenURL != nil {
			open, url := m.deps.OpenURL, ev.res.URL
			return m, func() tea.Msg { open(url); return nil }
		}
	case "r":
		if ev.err != "" {
			return m.openEditorial(ev.p)
		}
	}
	return m, nil
}

// blockLines renders one block to terminal lines, cached for the current width.
func (m Model) blockLines(i int, html string, w int) []string {
	ev := m.edv
	if ev.cache == nil || ev.cacheW != w {
		ev.cache, ev.cacheW = map[int][]string{}, w
	}
	if l, ok := ev.cache[i]; ok {
		return l
	}
	out, err := scrape.Render(`<div class="problem-statement">`+html+`</div>`, w, m.styles().Glamour, m.readingOptions())
	if err != nil {
		out = clean(err.Error())
	}
	ev.cache[i] = strings.Split(out, "\n")
	return ev.cache[i]
}

// editorialLines lays the whole post out: text, opened spoilers, the next locked one and the closed rest.
func (m Model) editorialLines(w int) []string {
	ev, st := m.edv, m.styles()
	res := ev.res
	var lines []string
	ev.marks = ev.marks[:0]
	if res.Whole {
		lines = append(lines, st.Warn.Render("no section for this problem: showing the whole post"), "")
	}
	k := 0
	for i, b := range res.Blocks {
		switch {
		case !b.Spoiler:
			lines = append(lines, m.blockLines(i, b.HTML, w)...)
		case k < ev.revealed:
			ev.marks = append(ev.marks, len(lines))
			lines = append(lines, st.Accent.Render("▾ "+clean(b.Title)))
			lines = append(lines, m.blockLines(i, b.HTML, w)...)
			k++
		case k == ev.revealed:
			ev.marks = append(ev.marks, len(lines))
			lines = append(lines, st.Warn.Render("▸ "+clean(b.Title)+m.kx("   {editorial.reveal} to reveal")))
			k++
		default:
			ev.marks = append(ev.marks, len(lines))
			lines = append(lines, st.Dim.Render("▸ "+clean(b.Title)+" (locked)"))
			k++
		}
		lines = append(lines, "")
	}
	if len(res.Links) > 0 {
		lines = append(lines, st.Accent2.Render("More materials"))
		for _, l := range res.Links {
			lines = append(lines, fmt.Sprintf("  %-22s %s", clean(l.Label), st.Dim.Render(l.URL)))
		}
	}
	return lines
}

func (m Model) editorialBox() []string {
	st := m.styles()
	ev := m.edv
	w, h := m.modalSize(100, 36)
	title := fmt.Sprintf("Editorial %s", ev.p.Code())
	var body []string
	note := m.kx("{editorial.close} close")
	switch {
	case ev.loading:
		body = []string{st.Dim.Render("loading the editorial...")}
	case ev.err != "":
		body = []string{st.Bad.Render(fit(ev.err, w-6)), "", st.Dim.Render(m.kx("{editorial.retry} to retry"))}
	case ev.res.Status != editorial.OK:
		switch ev.res.Status {
		case editorial.NotEnglish:
			body = []string{"Only a Russian editorial exists; verd shows English ones."}
		default:
			body = []string{"No editorial is published for this contest."}
		}
		if len(ev.res.Links) > 0 {
			body = append(body, "", st.Accent2.Render("Materials"))
			for _, l := range ev.res.Links {
				body = append(body, fmt.Sprintf("  %-22s %s", clean(l.Label), st.Dim.Render(l.URL)))
			}
		}
	default:
		if ev.res.Title != "" {
			title += "  " + clean(ev.res.Title)
		}
		all := m.editorialLines(w - 4)
		room := max(3, h-3)
		if ev.scroll < 0 && ev.revealed > 0 && ev.revealed <= len(ev.marks) {
			ev.scroll = ev.marks[ev.revealed-1]
		}
		ev.scroll = max(0, min(ev.scroll, len(all)-room))
		end := min(len(all), ev.scroll+room)
		body = all[ev.scroll:end]
		note = m.kx(fmt.Sprintf("%d/%d hints  {editorial.reveal} reveal next  {editorial.down}/{editorial.up} scroll  {editorial.browser} open post  {editorial.close} close", ev.revealed, ev.res.Spoilers()))
	}
	return box(title, body, note, w, min(h, len(body)+2), true, st, m.borderStyle())
}
