package tui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// keyAction is one rebindable shortcut. def are its default keys, written the way the handlers
// match them; a user's i-th key stands in for def[min(i, len-1)], so the handlers never change.
type keyAction struct {
	ctx, id, desc string
	def           []string
}

func ks(k ...string) []string { return k }

var keyActions = []keyAction{
	{"common", "help", "toggle help", ks("?")},
	{"common", "dismiss_toast", "dismiss a notification", ks("x")},
	{"tabs", "tab_1", "switch tab: Problems", ks("1")},
	{"tabs", "tab_2", "switch tab: Contests", ks("2")},
	{"tabs", "tab_3", "switch tab: Stats", ks("3")},
	{"tabs", "tab_4", "switch tab: Picker", ks("4")},
	{"tabs", "tab_5", "switch tab: Settings", ks("5")},
	{"app", "refresh", "refresh from the network", ks("ctrl+r")},
	{"app", "next_tab", "next tab", ks("tab")},

	{"problems", "quit", "quit", ks("q")},
	{"problems", "up", "move up", ks("up", "k")},
	{"problems", "down", "move down", ks("down", "j")},
	{"problems", "page_up", "page up", ks("pgup")},
	{"problems", "page_down", "page down", ks("pgdown")},
	{"problems", "open", "open Problem", ks("enter")},
	{"problems", "filters", "filters modal (rating, status, sort, tags)", ks("f")},
	{"problems", "filter_expr", "filter expression: 800-1200 +dp -graphs unsolved", ks(":")},
	{"problems", "clear_filters", "clear all filters", ks("X")},
	{"problems", "search", "live fuzzy search", ks("/")},
	{"problems", "mark_solved", "mark a CSES task solved or not", ks("m")},
	{"problems", "sync_cses", "sync solved marks from CSES", ks("Y")},

	{"contests", "quit", "quit", ks("q")},
	{"contests", "up", "move up", ks("up", "k")},
	{"contests", "down", "move down", ks("down", "j")},
	{"contests", "page_up", "page up", ks("pgup")},
	{"contests", "page_down", "page down", ks("pgdown")},
	{"contests", "open", "list the contest's Problems", ks("enter")},

	{"contest", "close", "close", ks("q", "esc")},
	{"contest", "up", "move up", ks("up", "k")},
	{"contest", "down", "move down", ks("down", "j")},
	{"contest", "open", "open the Problem", ks("enter")},

	{"stats", "quit", "quit", ks("q")},
	{"stats", "down", "scroll down", ks("down", "j")},
	{"stats", "up", "scroll up", ks("up", "k")},
	{"stats", "page_down", "page down", ks("pgdown")},
	{"stats", "page_up", "page up", ks("pgup")},
	{"stats", "next_problem", "select next attempted Problem", ks("n")},
	{"stats", "prev_problem", "select previous attempted Problem", ks("N")},
	{"stats", "open", "open the selected Problem", ks("enter")},
	{"stats", "picker", "Problem Picker with the weak-topics preset", ks("p")},

	{"picker", "quit", "quit", ks("q")},
	{"picker", "reroll", "re-roll", ks("r", "space")},
	{"picker", "open", "open the Problem", ks("enter")},
	{"picker", "filters", "filters: 800-1200 +dp -graphs", ks("f")},
	{"picker", "weak_topics", "weak-topics preset", ks("w")},

	{"settings", "quit", "quit", ks("q")},
	{"settings", "down", "move down", ks("down", "j")},
	{"settings", "up", "move up", ks("up", "k")},
	{"settings", "top", "first setting", ks("home", "g")},
	{"settings", "bottom", "last setting", ks("end", "G")},
	{"settings", "next_value", "next value / toggle", ks("right", "l", "space")},
	{"settings", "prev_value", "previous value", ks("left", "h")},
	{"settings", "edit", "edit a text value / open the shortcut editor", ks("enter")},
	{"settings", "login", "paste a browser session (direct submit)", ks("L")},
	{"settings", "edit_config", "open config.toml in your editor", ks("e")},

	{"problem", "quit", "quit", ks("q")},
	{"problem", "back", "back (also cancels a stress run)", ks("esc")},
	{"problem", "pane_next", "next pane: statement, tests, detail", ks("tab")},
	{"problem", "pane_prev", "previous pane", ks("shift+tab")},
	{"problem", "up", "scroll up / previous test", ks("up", "k")},
	{"problem", "down", "scroll down / next test", ks("down", "j")},
	{"problem", "page_up", "page up", ks("pgup")},
	{"problem", "page_down", "page down", ks("pgdown")},
	{"problem", "top", "jump to the top", ks("home", "g")},
	{"problem", "bottom", "jump to the bottom", ks("end", "G")},
	{"problem", "edit", "edit the Solution in your editor", ks("e")},
	{"problem", "notes", "edit the Problem's notes.md", ks("N")},
	{"problem", "copy", "copy the focused pane", ks("y")},
	{"problem", "manage_tests", "manage tests: view, add, edit, copy, delete", ks("T")},
	{"problem", "add_test", "add a Custom Test", ks("a")},
	{"problem", "language", "switch language", ks("l")},
	{"problem", "submit", "submit the Solution", ks("s")},
	{"problem", "run_tests", "run tests", ks("t")},
	{"problem", "stress", "stress test", ks("S")},
	{"problem", "stress_save", "save the stress counterexample as a test", ks("w")},
	{"problem", "cycle_mode", "cycle Comparison Mode", ks("c")},
	{"problem", "next_test", "select next test", ks("n")},
	{"problem", "prev_test", "select previous test", ks("p")},
	{"problem", "diff", "diff the selected failing test", ks("d")},
	{"problem", "refetch", "refetch statement", ks("r")},
	{"problem", "open_browser", "open in the browser", ks("o")},
	{"problem", "mark_solved", "mark a CSES task solved or not", ks("m")},
	{"problem", "toggle_tags", "show or hide the tags", ks("v")},
	{"problem", "widen", "widen the right column", ks(">", ".")},
	{"problem", "narrow", "narrow the right column", ks("<", ",")},
	{"problem", "grow_tests", "grow the test list", ks("+", "=")},
	{"problem", "shrink_tests", "shrink the test list", ks("-")},

	{"testmgr", "close", "close", ks("q", "esc")},
	{"testmgr", "down", "move down", ks("down", "j")},
	{"testmgr", "up", "move up", ks("up", "k")},
	{"testmgr", "add", "add a Custom Test here", ks("a")},
	{"testmgr", "edit", "edit the selected Custom Test", ks("e", "enter")},
	{"testmgr", "copy", "copy the selected test into a new Custom Test", ks("c")},
	{"testmgr", "delete", "delete the selected Custom Test", ks("d", "x")},
	{"testmgr", "add_in_editor", "add a Custom Test in your editor", ks("E")},
	{"testmgr", "run_tests", "run tests", ks("t")},

	{"submission", "close", "close the Submission modal", ks("q", "esc", "enter")},

	{"help", "close", "close help", ks("q", "esc")},
	{"help", "page_next", "next page", ks("right", "l", "tab")},
	{"help", "page_prev", "previous page", ks("left", "h", "shift+tab")},
	{"help", "scroll_down", "scroll down", ks("down", "j")},
	{"help", "scroll_up", "scroll up", ks("up", "k")},
	{"help", "page_1", "page: This screen", ks("1")},
	{"help", "page_2", "page: Everywhere", ks("2")},
	{"help", "page_3", "page: Mouse", ks("3")},
	{"help", "page_4", "page: Guide", ks("4")},

	{"editor", "focus", "toggle the keyboard between verd and the embedded editor", ks("ctrl+\\")},
	{"editor", "focus_left", "keyboard to the left column", ks("alt+h")},
	{"editor", "focus_right", "keyboard to the right column", ks("alt+l")},
	{"editor", "zoom", "hide or show verd (editor fullscreen)", ks("alt+z")},
	{"editor", "wider", "grow the editor column", ks("alt+]")},
	{"editor", "narrower", "shrink the editor column", ks("alt+[")},
	{"editor", "swap_side", "move editor between left and right", ks("alt+s")},
}

// keyGroups lists the action groups live in each context, innermost first. Keys in one chain
// must be unique; ctrl+c always quits and cannot be rebound.
var keyGroups = map[string][]string{
	"problems":   {"problems", "app", "tabs", "common"},
	"contests":   {"contests", "app", "tabs", "common"},
	"contest":    {"contest", "app", "tabs", "common"},
	"stats":      {"stats", "app", "tabs", "common"},
	"picker":     {"picker", "app", "tabs", "common"},
	"settings":   {"settings", "app", "tabs", "common"},
	"problem":    {"problem", "tabs", "common"},
	"testmgr":    {"testmgr"},
	"submission": {"submission"},
	"help":       {"help", "common"},
	"editor":     {"editor"},
}

// keyGroupOrder is how the shortcut editor lists the groups.
var keyGroupOrder = []string{"common", "tabs", "app", "problems", "contests", "contest", "stats", "picker", "settings", "problem", "testmgr", "submission", "help", "editor"}

var keyGroupTitle = map[string]string{
	"common": "Everywhere", "tabs": "Tabs", "app": "Tab screens", "problems": "Problems", "contests": "Contests",
	"contest": "Contest modal", "stats": "Stats", "picker": "Picker", "settings": "Settings", "problem": "Problem view",
	"testmgr": "Test manager", "submission": "Submission modal", "help": "Help", "editor": "Embedded editor",
}

// keymap is the user's bindings over keyActions. user is "group.id" -> keys.
type keymap struct {
	user map[string][]string
	in   map[string]map[string]string // group -> user key -> canonical key
	dead map[string]map[string]bool   // group -> default key that its rebound action no longer answers to
}

func actionID(a keyAction) string { return a.ctx + "." + a.id }

func findAction(id string) (keyAction, bool) {
	for _, a := range keyActions {
		if actionID(a) == id {
			return a, true
		}
	}
	return keyAction{}, false
}

// newKeymap validates user and builds the lookup tables.
func newKeymap(user map[string][]string) (*keymap, error) {
	k := &keymap{user: map[string][]string{}, in: map[string]map[string]string{}, dead: map[string]map[string]bool{}}
	for id, keys := range user {
		a, ok := findAction(id)
		if !ok {
			return nil, fmt.Errorf("keys: unknown shortcut %q (see the Keyboard shortcuts editor in Settings)", id)
		}
		for _, key := range keys {
			if _, err := parseKey(key); err != nil {
				return nil, fmt.Errorf("keys.%s: %w", id, err)
			}
			if key == "ctrl+c" {
				return nil, fmt.Errorf("keys.%s: ctrl+c always quits and cannot be rebound", id)
			}
		}
		if len(keys) == 0 {
			continue
		}
		k.user[id] = keys
		if k.in[a.ctx] == nil {
			k.in[a.ctx], k.dead[a.ctx] = map[string]string{}, map[string]bool{}
		}
		for i, key := range keys {
			k.in[a.ctx][key] = a.def[min(i, len(a.def)-1)]
		}
		for _, d := range a.def {
			if !slices.Contains(keys, d) {
				k.dead[a.ctx][d] = true
			}
		}
	}
	for ctx, groups := range keyGroups {
		seen := map[string]string{}
		for _, a := range keyActions {
			if !slices.Contains(groups, a.ctx) {
				continue
			}
			for _, key := range k.keys(a) {
				if other, dup := seen[key]; dup && other != actionID(a) {
					return nil, fmt.Errorf("key %q is bound to both %s and %s (in %s)", key, other, actionID(a), keyGroupTitle[ctx])
				}
				seen[key] = actionID(a)
			}
		}
	}
	return k, nil
}

// keys are the action's current keys: the user's, else the defaults.
func (k *keymap) keys(a keyAction) []string {
	if u := k.user[actionID(a)]; len(u) > 0 {
		return u
	}
	return a.def
}

// translate maps a pressed key to the canonical key the handlers match. drop means the key was
// rebound away and now does nothing here.
func (k *keymap) translate(ctx, key string) (canon string, drop bool) {
	groups := keyGroups[ctx]
	for _, g := range groups {
		if c, ok := k.in[g][key]; ok {
			return c, false
		}
	}
	for _, g := range groups {
		if k.dead[g][key] {
			return "", true
		}
	}
	return key, false
}

// CheckKeys reports why a set of user bindings ("context.action" -> keys) is invalid, or nil.
func CheckKeys(user map[string][]string) error {
	_, err := newKeymap(user)
	return err
}

// keyCtx is the context a key press lands in right now; "" means a text field has the keys.
func (m Model) keyCtx() string {
	switch {
	case m.tm != nil:
		if m.tm.ed != nil || m.tm.confirmDel {
			return ""
		}
		return "testmgr"
	case m.fm != nil:
		return ""
	case m.subModal:
		return "submission"
	case m.help:
		return "help"
	case m.viewing():
		if m.confirm {
			return ""
		}
		return "problem"
	case m.input != nil:
		return ""
	case m.tab == 4 && m.setEdit != nil:
		return ""
	}
	return m.screenCtx()
}

// screenCtx is the screen under any modal.
func (m Model) screenCtx() string {
	switch {
	case m.viewing():
		return "problem"
	case m.tab == 0:
		return "problems"
	case m.tab == 1 && m.contestOpen != nil:
		return "contest"
	case m.tab == 1:
		return "contests"
	case m.tab == 2:
		return "stats"
	case m.tab == 3:
		return "picker"
	}
	return "settings"
}

// remapKey rewrites a key press so the handlers see the canonical key; ok=false drops it.
func (m Model) remapKey(msg tea.KeyPressMsg) (tea.KeyPressMsg, bool) {
	if m.km == nil || m.ke != nil || len(m.km.user) == 0 {
		return msg, true
	}
	ctx := m.keyCtx()
	if ctx == "" {
		return msg, true
	}
	key := msg.String()
	canon, drop := m.km.translate(ctx, key)
	switch {
	case drop:
		return msg, false
	case canon == key:
		return msg, true
	}
	out, err := parseKey(canon)
	if err != nil {
		return msg, true
	}
	return out, true
}

var keyNames = map[string]rune{
	"enter": tea.KeyEnter, "tab": tea.KeyTab, "backspace": tea.KeyBackspace, "esc": tea.KeyEscape,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"delete": tea.KeyDelete, "insert": tea.KeyInsert, "home": tea.KeyHome, "end": tea.KeyEnd,
	"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
}

// parseKey builds the key press whose String() is s: "x", "G", "enter", "ctrl+r", "alt+z", "shift+tab", "space".
func parseKey(s string) (tea.KeyPressMsg, error) {
	var k tea.Key
	rest := s
	for {
		switch {
		case strings.HasPrefix(rest, "ctrl+") && len(rest) > 5:
			k.Mod |= tea.ModCtrl
			rest = rest[5:]
			continue
		case strings.HasPrefix(rest, "alt+") && len(rest) > 4:
			k.Mod |= tea.ModAlt
			rest = rest[4:]
			continue
		case strings.HasPrefix(rest, "shift+") && len(rest) > 6:
			k.Mod |= tea.ModShift
			rest = rest[6:]
			continue
		}
		break
	}
	if code, ok := keyNames[rest]; ok {
		k.Code = code
	} else if rest == "space" {
		k.Code = tea.KeySpace
		if k.Mod == 0 {
			k.Text = " "
		}
	} else if r := []rune(rest); len(r) == 1 {
		k.Code = r[0]
		if k.Mod == 0 {
			k.Text = rest
		}
	} else {
		return tea.KeyPressMsg{}, fmt.Errorf("unknown key %q (use x, enter, esc, tab, space, up, ctrl+x, alt+x, shift+tab ...)", s)
	}
	return tea.KeyPressMsg(k), nil
}

var keyRef = regexp.MustCompile(`\{([a-z]+\.[a-z_0-9]+)\}`)

// kx replaces {context.action} in s with that action's current first key, so hints and messages
// follow the user's bindings.
func (m Model) kx(s string) string {
	if !strings.Contains(s, "{") {
		return s
	}
	return keyRef.ReplaceAllStringFunc(s, func(ref string) string {
		a, ok := findAction(ref[1 : len(ref)-1])
		if !ok || m.km == nil {
			return ref
		}
		return m.km.keys(a)[0]
	})
}

// KeyInfo describes one rebindable shortcut for `verd keys`.
type KeyInfo struct {
	Context, Action, Desc string
	Default, Current      []string
}

// KeyActions lists every shortcut with its default and current keys (user overrides applied).
func KeyActions(user map[string][]string) []KeyInfo {
	var out []KeyInfo
	for _, a := range editableActions() {
		cur := a.def
		if u := user[actionID(a)]; len(u) > 0 {
			cur = u
		}
		out = append(out, KeyInfo{a.ctx, a.id, a.desc, a.def, cur})
	}
	return out
}

// KeyContexts lists the shortcut contexts with a one-line title each.
func KeyContexts() [][2]string {
	var out [][2]string
	for _, g := range keyGroupOrder {
		out = append(out, [2]string{g, keyGroupTitle[g]})
	}
	return out
}
