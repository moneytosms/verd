package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// palette is the command palette: one fuzzy box over every action available right now, every
// setting and the snippet library. Picking an action presses its key; a setting jumps to it; a
// snippet is copied.
type palette struct {
	q   string
	sel int
	ctx string
}

// palItem is one line of the palette: an action (key set), a setting (setting >= 0) or a snippet.
type palItem struct {
	title, group, keys string
	key                string // the key that runs the action
	setting            int    // index into settingDefs, or -1
	snip               *Snippet
}

// noisy actions are plain navigation: pressing them from a palette is pointless.
var noisyActions = map[string]bool{"up": true, "down": true, "page_up": true, "page_down": true, "top": true, "bottom": true,
	"scroll_up": true, "scroll_down": true, "pan_left": true, "pan_right": true, "close": true, "back": true, "page_next": true, "page_prev": true,
	"next_value": true, "prev_value": true, "dismiss_toast": true, "palette": true}

// palItems are everything the palette can run in context ctx.
func (m Model) palItems(ctx string) []palItem {
	var out []palItem
	groups := keyGroups[ctx]
	for _, a := range editableActions() {
		if !slices.Contains(groups, a.ctx) || noisyActions[a.id] {
			continue
		}
		keys := m.km.keys(a)
		if len(keys) == 0 {
			continue
		}
		out = append(out, palItem{title: a.desc, group: keyGroupTitle[a.ctx], keys: strings.Join(keys, " "), key: keys[0], setting: -1})
	}
	if !m.viewing() {
		for i, d := range settingDefs {
			out = append(out, palItem{title: "Setting: " + d.label, group: d.group, setting: i})
		}
	}
	if m.deps.Snippets != nil {
		for _, sn := range m.deps.Snippets(m.lang()) {
			out = append(out, palItem{title: "Snippet: " + sn.Name, group: sn.Desc, setting: -1, snip: &sn})
		}
	}
	return out
}

// palMatches are the items matching the query, best first (all in order when it is empty).
func (m Model) palMatches() []palItem {
	items := m.palItems(m.pal.ctx)
	q := strings.TrimSpace(m.pal.q)
	if q == "" {
		return items
	}
	type hit struct {
		it palItem
		s  int
	}
	var hits []hit
	for _, it := range items {
		hay := it.title + " " + it.group + " " + it.keys
		total, ok := 0, true
		for _, w := range strings.Fields(q) {
			s, h := fuzzyScore(w, hay)
			total, ok = total+s, ok && h
		}
		if ok {
			hits = append(hits, hit{it, total})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].s > hits[j].s })
	out := make([]palItem, len(hits))
	for i, h := range hits {
		out[i] = h.it
	}
	return out
}

func (m Model) openPalette() Model {
	if ctx := m.keyCtx(); ctx != "" && keyGroups[ctx] != nil {
		m.pal = &palette{ctx: ctx}
	}
	return m
}

func (m Model) updatePalette(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := m.pal
	ms := m.palMatches()
	switch k := msg.String(); k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.pal = nil
	case "down", "ctrl+n", "tab":
		p.sel = min(p.sel+1, max(0, len(ms)-1))
	case "up", "ctrl+p", "shift+tab":
		p.sel = max(0, p.sel-1)
	case "backspace":
		if r := []rune(p.q); len(r) > 0 {
			p.q, p.sel = string(r[:len(r)-1]), 0
		}
	case "ctrl+u":
		p.q, p.sel = "", 0
	case "enter":
		if p.sel >= len(ms) {
			return m, nil
		}
		it := ms[p.sel]
		m.pal = nil
		if it.snip != nil {
			m.Notice = fmt.Sprintf("copied snippet %s (%s)", it.snip.Name, m.lang())
			return m, tea.SetClipboard(it.snip.Code)
		}
		if it.setting >= 0 {
			m.tab, m.setQ, m.setFind, m.setSel = 4, "", false, it.setting
			return m.onTab(), nil
		}
		press, ok := keyPress(it.key)
		if !ok {
			m.Notice = "cannot press " + it.key
			return m, nil
		}
		return m.Update(press)
	default:
		if msg.Text != "" {
			p.q, p.sel = p.q+clean(msg.Text), 0
		}
	}
	return m, nil
}

func (m Model) paletteBox() []string {
	st := m.styles()
	p := m.pal
	w, h := m.modalSize(90, 30)
	ms := m.palMatches()
	p.sel = min(p.sel, max(0, len(ms)-1))
	rows := max(3, h-5)
	body := []string{st.Accent.Render("> ") + p.q + st.Dim.Render("▏")}
	start := max(0, min(p.sel-rows/2, len(ms)-rows))
	for i := start; i < min(len(ms), start+rows); i++ {
		it := ms[i]
		cur := "  "
		if i == p.sel {
			cur = st.Accent.Render("> ")
		}
		line := fmt.Sprintf("%-46s %s", it.title, st.Dim.Render(it.group))
		if it.keys != "" {
			line = fmt.Sprintf("%-46s %s  %s", it.title, st.Chip.Render(" "+it.keys+" "), st.Dim.Render(it.group))
		}
		body = append(body, cur+cell([]string{line}, 0, w-6))
	}
	if len(ms) == 0 {
		body = append(body, st.Dim.Render("  nothing matches"))
	}
	note := fmt.Sprintf("%d/%d  enter run  up/down  esc close", min(p.sel+1, len(ms)), len(ms))
	return box("Command palette", body, note, w, min(h, len(body)+2), true, st, m.borderStyle())
}

var namedKeys = map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "backspace": tea.KeyBackspace,
	"delete": tea.KeyDelete, "up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown}

// keyPress builds the key press whose String() is key ("ctrl+r", "shift+tab", "space", "G", ...).
func keyPress(key string) (tea.KeyPressMsg, bool) {
	var mod tea.KeyMod
	for {
		switch {
		case strings.HasPrefix(key, "ctrl+") && len(key) > 5:
			mod, key = mod|tea.ModCtrl, key[5:]
			continue
		case strings.HasPrefix(key, "alt+") && len(key) > 4:
			mod, key = mod|tea.ModAlt, key[4:]
			continue
		case strings.HasPrefix(key, "shift+") && len(key) > 6:
			mod, key = mod|tea.ModShift, key[6:]
			continue
		}
		break
	}
	var msg tea.KeyPressMsg
	switch r := []rune(key); {
	case key == "space":
		msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case namedKeys[key] != 0:
		msg = tea.KeyPressMsg{Code: namedKeys[key]}
	case len(r) == 1:
		msg = tea.KeyPressMsg{Code: r[0], Text: key}
	default:
		return msg, false
	}
	msg.Mod = mod
	if mod&(tea.ModCtrl|tea.ModAlt) != 0 {
		msg.Text = ""
	}
	return msg, true
}
