package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// keyEditor is the Settings modal that rebinds shortcuts. cap: 0 browsing, 1 waiting for the key
// that replaces the action's keys, 2 waiting for a key to add.
type keyEditor struct {
	sel      int
	cap      int
	note     string
	q        string // fuzzy filter over group, description, id and keys
	typing   bool   // keys go into q
	resetAll bool   // waiting for y/n to confirm reset all
}

// editableActions are the actions the editor lists, in group order.
func editableActions() []keyAction {
	var out []keyAction
	for _, g := range keyGroupOrder {
		for _, a := range keyActions {
			if a.ctx == g {
				out = append(out, a)
			}
		}
	}
	return out
}

// keRows are the actions that match the filter: all in group order, or best match first.
func (m Model) keRows() []keyAction {
	all := editableActions()
	if m.ke.q == "" {
		return all
	}
	type hit struct {
		a keyAction
		s int
	}
	var hits []hit
	for _, a := range all {
		hay := keyGroupTitle[a.ctx] + " " + a.desc + " " + actionID(a) + " " + strings.Join(m.km.keys(a), " ")
		total, ok := 0, true
		for _, w := range strings.Fields(m.ke.q) { // every word must match
			s, hit := fuzzyScore(w, hay)
			total, ok = total+s, ok && hit
		}
		if ok {
			hits = append(hits, hit{a, total})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].s > hits[j].s })
	out := make([]keyAction, len(hits))
	for i, h := range hits {
		out[i] = h.a
	}
	return out
}

func (m Model) openKeyEditor() Model {
	m.ke = &keyEditor{}
	return m
}

func (m Model) keyEditorBox() []string {
	st := m.styles()
	w, h := m.modalSize(96, 36)
	acts := m.keRows()
	m.ke.sel = max(0, min(m.ke.sel, len(acts)-1))
	var lines []string
	selAt := 0
	group := ""
	for i, a := range acts {
		if m.ke.q == "" && a.ctx != group {
			group = a.ctx
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, st.Accent2.Render(strings.ToUpper(keyGroupTitle[group])))
		}
		cur := m.km.keys(a)
		var caps []string
		for _, k := range cur {
			caps = append(caps, st.Chip.Render(" "+k+" "))
		}
		mark := " "
		if len(m.km.user[actionID(a)]) > 0 {
			mark = st.Warn.Render("*")
		}
		desc := a.desc
		if m.ke.q != "" {
			desc = keyGroupTitle[a.ctx] + ": " + desc
		}
		line := " " + mark + " " + fit(desc, 52) + " " + strings.Join(caps, " ")
		if i == m.ke.sel {
			selAt = len(lines)
			line = paintRow(line, w-4, st.Selected)
		}
		lines = append(lines, line)
	}
	if len(acts) == 0 {
		lines = []string{st.Dim.Render("  nothing matches")}
	}
	search := st.Dim.Render("/ search shortcuts")
	if m.ke.typing || m.ke.q != "" {
		search = st.Accent.Render("/ ") + m.ke.q
		if m.ke.typing {
			search += st.Accent.Render("▏")
		}
		search += st.Dim.Render(fmt.Sprintf("   %d match", len(acts)))
	}
	room := max(3, h-6)
	start := max(0, min(selAt-room/2, len(lines)-room))
	body := append([]string{search, ""}, lines[start:min(len(lines), start+room)]...)
	switch {
	case m.ke.resetAll:
		body = append(body, "", st.Accent.Render("reset all shortcuts to defaults? y/n (esc cancels)"))
	case m.ke.cap == 1:
		body = append(body, "", st.Accent.Render("press the new key for "+actionID(acts[m.ke.sel])+" (esc cancels)"))
	case m.ke.cap == 2:
		body = append(body, "", st.Accent.Render("press a key to add to "+actionID(acts[m.ke.sel])+" (esc cancels)"))
	case m.ke.note != "":
		style := st.Good
		if !strings.HasPrefix(m.ke.note, "saved") && !strings.HasPrefix(m.ke.note, "reset") && !strings.HasPrefix(m.ke.note, "cleared") {
			style = st.Bad
		}
		body = append(body, "", style.Render(clean(m.ke.note)))
	}
	note := "/ search  j/k move  enter rebind  a add a key  backspace reset  R reset all  esc close"
	if m.ke.typing {
		note = "type to filter  up/down move  enter done  esc clear"
	}
	return box("Keyboard shortcuts  (* = changed, saved to [keys.*] in config.toml)", body, note, w, min(h, len(body)+2), true, st)
}

// setKeys saves keys for one action (nil resets it) and rebuilds the keymap; a conflict is refused.
func (m Model) setKeys(a keyAction, keys []string) Model {
	user := map[string][]string{}
	for k, v := range m.km.user {
		user[k] = v
	}
	if slices.Equal(keys, a.def) {
		keys = nil
	}
	if len(keys) == 0 {
		delete(user, actionID(a))
	} else {
		user[actionID(a)] = keys
	}
	nk, err := newKeymap(user)
	if err != nil {
		m.ke.note = err.Error()
		return m
	}
	if m.deps.SaveKeys == nil {
		m.ke.note = "saving is unavailable"
		return m
	}
	if err := m.deps.SaveKeys(a.ctx, a.id, keys); err != nil {
		m.ke.note = err.Error()
		return m
	}
	m.km = nk
	if len(keys) == 0 {
		m.ke.note = "reset " + actionID(a)
	} else {
		m.ke.note = fmt.Sprintf("saved %s = %s", actionID(a), strings.Join(keys, ", "))
	}
	return m
}

func (m Model) updateKeyEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	acts := m.keRows()
	e := m.ke
	e.sel = max(0, min(e.sel, len(acts)-1))
	if e.typing {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			e.q, e.typing = "", false
		case "enter":
			e.typing = false
		case "down":
			e.sel = min(e.sel+1, len(acts)-1)
		case "up":
			e.sel = max(0, e.sel-1)
		case "backspace":
			if r := []rune(e.q); len(r) > 0 {
				e.q, e.sel = string(r[:len(r)-1]), 0
			}
		case "ctrl+u":
			e.q, e.sel = "", 0
		default:
			if msg.Text != "" {
				e.q, e.sel = e.q+clean(msg.Text), 0
			}
		}
		return m, nil
	}
	if e.resetAll {
		switch msg.String() {
		case "y":
			// Reset all actions by clearing user bindings
			user := map[string][]string{}
			for id := range m.km.user {
				parts := strings.Split(id, ".")
				if len(parts) != 2 {
					continue
				}
				if err := m.deps.SaveKeys(parts[0], parts[1], nil); err != nil {
					e.note = err.Error()
					break
				}
			}
			if e.note == "" {
				m.km, _ = newKeymap(user)
				e.note = fmt.Sprintf("cleared all %d custom shortcuts", len(m.km.user))
			}
			e.resetAll = false
		case "n", "esc":
			e.note = ""
			e.resetAll = false
		}
		return m, nil
	}
	if len(acts) == 0 { // nothing matches: only searching or leaving makes sense
		switch msg.String() {
		case "/":
			e.typing = true
		case "ctrl+c":
			return m, tea.Quit
		default:
			e.q = ""
		}
		return m, nil
	}
	a := acts[e.sel]
	if e.cap > 0 {
		key := msg.String()
		cap := e.cap
		e.cap = 0
		switch {
		case key == "esc":
			e.note = ""
		case cap == 1:
			m = m.setKeys(a, []string{key})
		case slices.Contains(m.km.keys(a), key):
			e.note = key + " is already bound to " + actionID(a)
		default:
			m = m.setKeys(a, append(slices.Clone(m.km.keys(a)), key))
		}
		return m, nil
	}
	e.note = ""
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "/":
		e.typing = true
	case "esc", "q":
		if e.q != "" && msg.String() == "esc" {
			e.q, e.sel = "", 0 // first esc clears the filter
			break
		}
		m.ke = nil
	case "j", "down":
		e.sel = min(e.sel+1, len(acts)-1)
	case "k", "up":
		e.sel = max(0, e.sel-1)
	case "pgdown":
		e.sel = min(e.sel+10, len(acts)-1)
	case "pgup":
		e.sel = max(0, e.sel-10)
	case "g", "home":
		e.sel = 0
	case "G", "end":
		e.sel = len(acts) - 1
	case "enter":
		e.cap = 1
	case "a":
		e.cap = 2
	case "backspace", "delete", "d":
		m = m.setKeys(a, nil)
	case "R":
		e.resetAll = true
	}
	return m, nil
}
