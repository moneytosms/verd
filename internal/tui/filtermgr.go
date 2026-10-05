package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Fields of the filter modal.
const (
	ffMin = iota
	ffMax
	ffStatus
	ffSort
	ffTags
	ffFields
)

var (
	statusOpts = []string{"any", "unsolved", "solved", "attempted"}
	sortOpts   = []string{"default", "rating", "-rating", "solved", "id"}
	sortNames  = map[string]string{"default": "default order", "rating": "rating, easiest first", "-rating": "rating, hardest first", "solved": "most solved", "id": "contest id"}
)

// filterMgr is the filter modal: rating range, status, sort and a tag picker.
type filterMgr struct {
	field int
	query string // tag list search
	tsel  int
	tags  []tagCount
}

type tagCount struct {
	name string
	n    int
}

// openFilters opens the filter modal over the Problems list.
func (m Model) openFilters() Model {
	counts := map[string]int{}
	for _, p := range m.Problems {
		for _, t := range p.Tags {
			counts[t]++
		}
	}
	var tags []tagCount
	for t, n := range counts {
		tags = append(tags, tagCount{t, n})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].n != tags[j].n {
			return tags[i].n > tags[j].n
		}
		return tags[i].name < tags[j].name
	})
	m.fm = &filterMgr{tags: tags}
	return m
}

// shownTags are the tags matching the query, best match first.
func (f *filterMgr) shownTags() []tagCount {
	if strings.TrimSpace(f.query) == "" {
		return f.tags
	}
	type scored struct {
		tagCount
		s int
	}
	var out []scored
	for _, t := range f.tags {
		if s, ok := fuzzyScore(f.query, t.name); ok {
			out = append(out, scored{t, s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].s > out[j].s })
	res := make([]tagCount, len(out))
	for i, o := range out {
		res[i] = o.tagCount
	}
	return res
}

func (m Model) tagState(name string) int {
	for _, t := range m.filter.Include {
		if t == name {
			return 1
		}
	}
	for _, t := range m.filter.Exclude {
		if t == name {
			return -1
		}
	}
	return 0
}

// cycleTag moves a tag off -> included -> excluded -> off.
func (m Model) cycleTag(name string) Model {
	drop := func(l []string) []string {
		var out []string
		for _, t := range l {
			if t != name {
				out = append(out, t)
			}
		}
		return out
	}
	switch m.tagState(name) {
	case 0:
		m.filter.Include = append(drop(m.filter.Include), name)
	case 1:
		m.filter.Include = drop(m.filter.Include)
		m.filter.Exclude = append(m.filter.Exclude, name)
	default:
		m.filter.Exclude = drop(m.filter.Exclude)
	}
	return m
}

func (m Model) applyFilter() Model {
	m.cursor = 0
	m.filterExpr = m.filter.Expr()
	return m.refilter()
}

func (m Model) updateFilters(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	f := m.fm
	k := msg.String()
	switch k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.fm = nil
		return m, nil
	case "ctrl+u": // reset everything
		m.filter, m.sortBy, f.query, f.tsel = Filter{Search: m.filter.Search, SearchMode: m.filter.SearchMode}, "", "", 0
		return m.applyFilter(), nil
	case "tab":
		f.field = (f.field + 1) % ffFields
		return m, nil
	case "shift+tab":
		f.field = (f.field + ffFields - 1) % ffFields
		return m, nil
	}
	switch f.field {
	case ffMin, ffMax:
		cur := &m.filter.MinRating
		if f.field == ffMax {
			cur = &m.filter.MaxRating
		}
		switch k {
		case "up":
			f.field = max(0, f.field-1)
		case "down", "enter":
			f.field++
		case "right", "+":
			*cur = min(3500, ((*cur)/100+1)*100)
			if *cur < 800 && *cur > 0 {
				*cur = 800
			}
		case "left", "-":
			if *cur > 0 {
				*cur = max(0, ((*cur-1)/100)*100)
				if *cur < 800 {
					*cur = 0
				}
			}
		case "backspace":
			*cur /= 10
		case "q":
			m.fm = nil
			return m, nil
		default:
			if len(msg.Text) == 1 && msg.Text[0] >= '0' && msg.Text[0] <= '9' && *cur < 10000 {
				n, _ := strconv.Atoi(strconv.Itoa(*cur) + msg.Text)
				*cur = n
			}
		}
		return m.applyFilter(), nil
	case ffStatus:
		switch k {
		case "up":
			f.field--
		case "down", "enter":
			f.field++
		case "right", "l", " ":
			m.filter.Status = statusChoice(nextOf(statusOpts, orAny(m.filter.Status)))
		case "left", "h":
			m.filter.Status = statusChoice(prevOf(statusOpts, orAny(m.filter.Status)))
		case "q":
			m.fm = nil
			return m, nil
		}
		return m.applyFilter(), nil
	case ffSort:
		cur := m.sortBy
		if cur == "" {
			cur = "default"
		}
		switch k {
		case "up":
			f.field--
		case "down", "enter":
			f.field++
		case "right", "l", " ":
			cur = nextOf(sortOpts, cur)
		case "left", "h":
			cur = prevOf(sortOpts, cur)
		case "q":
			m.fm = nil
			return m, nil
		}
		m.sortBy = cur
		if cur == "default" {
			m.sortBy = ""
		}
		return m.applyFilter(), nil
	}
	// tags
	shown := f.shownTags()
	switch k {
	case "up":
		if f.tsel == 0 {
			f.field--
		} else {
			f.tsel--
		}
	case "down":
		f.tsel = min(f.tsel+1, max(0, len(shown)-1))
	case "pgup":
		f.tsel = max(0, f.tsel-8)
	case "pgdown":
		f.tsel = min(f.tsel+8, max(0, len(shown)-1))
	case "enter", "space", " ", "right":
		if f.tsel < len(shown) {
			return m.cycleTag(shown[f.tsel].name).applyFilter(), nil
		}
	case "backspace":
		if r := []rune(f.query); len(r) > 0 {
			f.query = string(r[:len(r)-1])
			f.tsel = 0
		}
	default:
		if msg.Text != "" {
			f.query += clean(msg.Text)
			f.tsel = 0
		}
	}
	return m, nil
}

func orAny(s string) string {
	if s == "" {
		return "any"
	}
	return s
}

func statusChoice(s string) string {
	if s == "any" {
		return ""
	}
	return s
}

func prevOf(list []string, cur string) string {
	for i, x := range list {
		if x == cur {
			return list[(i+len(list)-1)%len(list)]
		}
	}
	return list[0]
}

// filterBox renders the filter modal.
func (m Model) filterBox() []string {
	st := m.styles()
	f := m.fm
	w, h := m.modalSize(96, 30)
	row := func(field int, label, value string) string {
		mark := "  "
		if f.field == field {
			mark = st.Accent.Render("▌ ")
		}
		return mark + st.Dim.Render(fit(label, 9)) + value
	}
	num := func(n int, field int) string {
		s := "any"
		if n > 0 {
			s = strconv.Itoa(n)
		}
		if f.field == field {
			return st.Accent.Render("[ " + s + "▏]")
		}
		return "[ " + s + " ]"
	}
	choice := func(field int, v string) string {
		if f.field == field {
			return st.Dim.Render("‹ ") + st.Accent.Render(v) + st.Dim.Render(" ›")
		}
		return v
	}
	sortV := m.sortBy
	if sortV == "" {
		sortV = "default"
	}
	body := []string{
		row(ffMin, "Rating", num(m.filter.MinRating, ffMin)+st.Dim.Render("  to  ")+num(m.filter.MaxRating, ffMax)),
		row(ffStatus, "Status", choice(ffStatus, orAny(m.filter.Status))),
		row(ffSort, "Sort", choice(ffSort, sortNames[sortV])),
		"",
	}
	_ = ffMax
	tagHead := "Tags"
	if f.field == ffTags {
		tagHead = st.Accent.Render("▌ Tags") + st.Dim.Render("  type to search · space cycles + / − / off")
	} else {
		tagHead = "  " + st.Dim.Render("Tags")
	}
	q := f.query
	if f.field == ffTags {
		q += "▏"
	}
	body = append(body, tagHead+"   "+st.Accent2.Render(q))
	shown := f.shownTags()
	room := max(3, h-2-len(body)-1)
	cols := 3
	cw := (w - 6) / cols
	perCol := room
	total := perCol * cols
	start := 0
	if f.tsel >= total {
		start = (f.tsel / perCol) * perCol / 1
		start = max(0, f.tsel-total+1)
	}
	for r := 0; r < perCol; r++ {
		var line string
		for c := 0; c < cols; c++ {
			i := start + c*perCol + r
			if i >= len(shown) {
				continue
			}
			t := shown[i]
			mark := st.Dim.Render("·")
			switch m.tagState(t.name) {
			case 1:
				mark = st.Good.Render("+")
			case -1:
				mark = st.Bad.Render("−")
			}
			cell := mark + " " + fit(clean(t.name), cw-9) + st.Dim.Render(fmt.Sprintf("%4d", t.n))
			if f.field == ffTags && i == f.tsel {
				cell = paintRow(cell, cw-1, st.Selected)
			} else {
				cell = fit(cell, cw-1)
			}
			line += cell + " "
		}
		body = append(body, "  "+line)
	}
	note := fmt.Sprintf("%d match  tab field  ctrl+u reset  esc close", len(m.visible))
	return box("Filters", body, note, w, min(h, len(body)+2), true, st)
}

