package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/theme"
)

// settingDef describes one editable config key.
type settingDef struct {
	group, key, label, desc string
	kind                    string // enum, bool or text
	live                    bool   // takes effect immediately; otherwise on the next start
}

var settingDefs = []settingDef{
	{"Appearance", "theme", "Theme", "Colors for the whole interface. Changes apply as you move through the list.", "enum", true},
	{"Appearance", "background", "Background", "auto follows your terminal; force dark or light if the colors look wrong.", "enum", true},
	{"Appearance", "border", "Borders", "Box frame style: rounded, square, heavy, double, ascii, or none.", "enum", true},
	{"Layout", "header", "Show header", "Toggle the tab bar and rule beneath it. Tab keys (1-5, tab) still work.", "bool", true},
	{"Layout", "footer", "Show footer", "Toggle the footer with key hints and status badges.", "bool", true},
	{"Layout", "tabs", "Tab order", "Visible tabs in order (comma-separated): problems,contests,stats,picker,settings. Settings cannot be hidden.", "text", true},
	{"General", "handle", "Handle", "Your Codeforces handle: where solved marks, stats and Submissions come from.", "text", false},
	{"General", "workspace", "Workspace", "Where Solutions and tests live: <workspace>/<contest>/<index>/. A relative path is resolved against where verd starts; verd --here does that for one run.", "text", false},
	{"General", "default_lang", "Default language", "Language for new Solutions. Switch per Problem with {problem.language}.", "enum", true},
	{"Testing", "autotest", "Autotest", "Run the tests whenever you save the Solution.", "bool", true},
	{"Testing", "time_multiplier", "Time multiplier", "Scales every time limit for local runs. Use 2.0 on a slow machine.", "text", false},
	{"Testing", "float_eps", "Float tolerance", "Absolute and relative tolerance for the float Comparison Mode.", "text", false},
	{"Sources", "source_cf", "Codeforces", "Show Codeforces Problems in the list, the picker and search.", "bool", true},
	{"Sources", "source_cses", "CSES", "Show the CSES Problem Set (400 tasks, grouped by topic). Press {app.refresh} to load it after turning it on. Mark tasks solved with {problems.mark_solved}, or pull your solved list in with {problems.sync_cses} (after verd sync cses once).", "bool", true},
	{"Editor", "editor", "Editor", "Your editor command: nvim (default), vim, hx, nano, micro, emacs, code and so on. Any command works from config.toml. Only Neovim reuses its pane on the next e.", "enum", true},
	{"Editor", "split", "Editor split", "How the editor opens: auto, tmux, herdr, embedded (inside verd) or suspend.", "enum", false},
	{"Editor", "embed_ratio", "Embedded share", "Share of the window verd keeps when the editor is embedded (0.1 to 0.9).", "text", false},
	{"Editor", "embed_side", "Embedded side", "Which column the embedded editor takes: left or right. {editor.focus_left} / {editor.focus_right} move the keyboard to the left / right column.", "enum", true},
	{"Editor", "embed_zoom", "Start zoomed", "Start the editor zoomed (fullscreen) when it opens.", "bool", true},
	{"Editor", "embed_focus_key", "Focus key", "Hands the keyboard between verd and the embedded editor.", "text", false},
	{"Reading", "reading_width", "Statement width", "Max width for statement text (0 = use the whole pane; 40-200 to cap it). Text is left-aligned inside the pane.", "text", true},
	{"Reading", "reading_margin", "Left margin", "Left margin in columns (0-8).", "text", true},
	{"Reading", "reading_spacing", "Paragraph spacing", "Blank lines between paragraphs: compact, normal or relaxed.", "enum", true},
	{"Reading", "reading_headings", "Section headings", "Style for Input/Output/Note headings: plain, bold, bar or underline.", "enum", true},
	{"Reading", "reading_math", "Math rendering", "TeX rendering: unicode (to Unicode symbols) or raw (show TeX source).", "enum", true},
	{"Reading", "reading_emphasis", "Emphasis", "Render italics and bold; off = plain text for emphasis.", "bool", true},
	{"Mouse", "mouse", "Mouse reporting", "Enable mouse support. Off allows terminal-native selection everywhere.", "bool", true},
	{"Mouse", "wheel_lines", "Wheel lines", "Lines scrolled per wheel notch (1-20).", "text", true},
	{"Mouse", "mouse_select", "Drag to select", "Enable drag-to-select-lines in the Problem view.", "bool", true},
	{"Keys", "keys", "Shortcuts", "Rebind every shortcut: enter opens the editor. Changes apply at once and are saved under [keys.*] in config.toml, e.g. [keys.problem] run_tests = \"ctrl+t\" (a list gives several keys). ctrl+c always quits.", "keys", true},
	{"Snippets", "snippets", "Template", "The starting text of a new Solution, one per language. ←/→ picks the language, enter edits its Template file. Available variables are listed below; saving the file applies to the next new Solution.", "snippets", true},
	{"Submit", "submit_mode", "Submit mode", "browser copies the Solution and opens Codeforces (safe). direct posts it from verd with your saved session: experimental, account risk. See docs/submit.md.", "enum", false},
}

// settingEdit is a text edit in the Settings tab. cred is non-zero while pasting a browser session
// for direct submit: 1 the cookie header (shown masked), 2 the user-agent.
type settingEdit struct {
	text   string
	cred   int
	cookie string
}

// snippetVars are the variables a Template can use (see workspace.TemplateVars).
var snippetVars = [][2]string{
	{"{{.Problem.ID}}", "CF 1900A / CSES1068"},
	{"{{.Problem.Name}}", "problem name"},
	{"{{.Problem.URL}}", "problem page"},
	{"{{.Contest}} {{.Index}}", "contest id and problem index"},
	{"{{.Handle}}", "your handle"},
	{"{{.Date}}", "2026-10-10"},
	{"{{.Time}}", "20:50"},
	{"{{.DateTime}}", "2026-10-10 20:50"},
	{"{{cursor}}", "where the editor cursor starts"},
}

// snippetLang is the language the Snippets setting is showing.
func (m Model) snippetLang() string {
	if m.snipLang != "" {
		return m.snipLang
	}
	return m.deps.DefaultLang
}

// options are the allowed values of an enum setting.
func (m Model) options(key string) []string {
	switch key {
	case "theme":
		names := theme.Names()
		out := []string{"terminal"}
		for _, n := range names {
			if n != "terminal" {
				out = append(out, n)
			}
		}
		return out
	case "default_lang":
		return m.deps.Langs
	case "editor":
		out := []string{"nvim", "vim", "hx", "nano", "micro", "emacs", "kak", "code", "subl", "zed"}
		if cur := m.setting("editor"); cur != "" && !slices.Contains(out, cur) {
			out = append(out, cur) // a custom command from config.toml stays selectable
		}
		return out
	}
	return config.Options(key)
}

func (m Model) setting(key string) string {
	if v, ok := m.cfgVals[key]; ok {
		return v
	}
	return ""
}

// apply saves a setting and applies it live where it can be.
func (m Model) apply(d settingDef, value string) Model {
	m.setNote = ""
	if m.deps.SaveSetting != nil {
		if err := m.deps.SaveSetting(d.key, value); err != nil {
			m.setNote = err.Error()
			return m
		}
	}
	if m.cfgVals == nil {
		m.cfgVals = map[string]string{}
	}
	m.cfgVals[d.key] = value
	m = m.applyLive(d.key, value)
	// source_cses special case
	if value == "true" && d.key == "source_cses" {
		m.setNote = m.kx("saved: press {app.refresh} on a list to load the CSES tasks")
		return m
	}
	if d.key == "submit_mode" && value == "direct" && m.deps.SaveCreds != nil && (m.deps.HasCreds == nil || !m.deps.HasCreds()) {
		m.setEdit = &settingEdit{cred: 1} // direct needs a saved session: ask for it now
		m.setNote = "paste your browser session (see docs/submit.md); esc skips"
		return m
	}
	if !d.live {
		m.setNote = "saved: applies the next time verd starts"
	} else {
		m.setNote = "saved"
	}
	return m
}

// applyLive applies the live effects of a config key change (for hot reload).
func (m Model) applyLive(key, value string) Model {
	switch key {
	case "theme":
		m.theme, _ = theme.Get(value)
	case "background":
		m.bgMode = value
		if value != "auto" {
			m.dark = value == "dark"
		}
	case "embed_side":
		m.deps.EmbedSide = value
		m.resizeEmbed()
	case "embed_zoom":
		m.deps.EmbedZoom = value == "true"
	case "autotest":
		m.deps.Autotest = value == "true"
	case "default_lang":
		m.deps.DefaultLang = value
	case "source_cf", "source_cses":
		if m.filter.Source != "" && !m.sourceOn(m.filter.Source) {
			m.filter.Source = ""
		}
		m.cursor = 0
		m = m.refilter()
	case "reading_width", "reading_margin", "reading_spacing", "reading_headings", "reading_math", "reading_emphasis":
		// Invalidate bodyW to force re-render with new reading preferences
		if m.detail != nil {
			m.bodyW = 0
		}
	}
	return m
}

// step moves an enum or bool setting by dir (+1/-1).
func (m Model) step(d settingDef, dir int) Model {
	switch d.kind {
	case "keys":
		return m.openKeyEditor()
	case "snippets":
		langs := m.deps.Langs
		if len(langs) == 0 {
			return m
		}
		cur := m.snippetLang()
		i := slices.Index(langs, cur)
		m.snipLang = langs[((i+dir)%len(langs)+len(langs))%len(langs)]
		return m
	case "bool":
		return m.apply(d, strconv.FormatBool(m.setting(d.key) != "true"))
	case "enum":
		opts := m.options(d.key)
		if len(opts) == 0 {
			return m
		}
		i := 0
		for j, o := range opts {
			if o == m.setting(d.key) {
				i = j
			}
		}
		return m.apply(d, opts[(i+dir+len(opts))%len(opts)])
	}
	return m
}

func (m Model) updateSettings(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.setEdit != nil && m.setEdit.cred > 0 {
		return m.updateCred(msg)
	}
	if m.setEdit != nil {
		d := settingDefs[m.setSel]
		switch msg.String() {
		case "esc":
			m.setEdit = nil
		case "enter":
			text := strings.TrimSpace(m.setEdit.text)
			m.setEdit = nil
			return m.apply(d, text), nil
		case "backspace":
			if r := []rune(m.setEdit.text); len(r) > 0 {
				m.setEdit.text = string(r[:len(r)-1])
			}
		case "ctrl+u":
			m.setEdit.text = ""
		case "ctrl+c":
			return m, tea.Quit
		default:
			if msg.Text != "" {
				m.setEdit.text += clean(msg.Text)
			}
		}
		return m, nil
	}
	d := settingDefs[m.setSel]
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.setSel = min(m.setSel+1, len(settingDefs)-1)
	case "k", "up":
		m.setSel = max(0, m.setSel-1)
	case "g", "home":
		m.setSel = 0
	case "G", "end":
		m.setSel = len(settingDefs) - 1
	case "right", "l", " ", "space":
		m = m.step(d, 1)
		if d.kind == "text" && msg.String() != " " && msg.String() != "space" {
			break
		}
	case "left", "h":
		m = m.step(d, -1)
	case "enter":
		if d.kind == "snippets" && m.deps.EditTemplate != nil {
			cmd, err := m.deps.EditTemplate(m.snippetLang())
			return m.afterOpen(cmd, err, "template")
		}
		if d.kind == "text" {
			m.setEdit = &settingEdit{text: m.setting(d.key)}
		} else {
			m = m.step(d, 1)
		}
	case "L":
		if d.key == "submit_mode" && m.deps.SaveCreds != nil {
			m.setEdit, m.setNote = &settingEdit{cred: 1}, "paste your browser session (see docs/submit.md); esc cancels"
		}
	case "e":
		if m.deps.EditConfig != nil {
			cmd, err := m.deps.EditConfig()
			return m.afterOpen(cmd, err, "config")
		}
	}
	return m, nil
}

// onConfigChanged reloads the config file when it changes.
func (m Model) onConfigChanged() (tea.Model, tea.Cmd) {
	if m.deps.LoadConfig == nil {
		return m, nil
	}
	settings, keys, err := m.deps.LoadConfig()
	if err != nil {
		m.Notice = "config reload failed: " + err.Error()
		return m, nil
	}
	// Apply the new settings
	for key, value := range settings {
		if m.cfgVals[key] != value {
			m.cfgVals[key] = value
			m = m.applyLive(key, value)
		}
	}
	// Reload keys if they changed
	if !mapsEqual(m.deps.Keys, keys) {
		m.deps.Keys = keys
		km, err := newKeymap(keys)
		if err != nil {
			m.Notice = "keys: " + err.Error()
			return m, nil
		}
		m.km = km
	}
	m.Notice = "config reloaded"
	return m, nil
}

func mapsEqual(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if u, ok := b[k]; !ok || len(u) != len(v) {
			return false
		} else {
			for i, x := range v {
				if x != u[i] {
					return false
				}
			}
		}
	}
	return true
}

// settingLine is one row of the settings list; idx is -1 for a group heading.
type settingLine struct {
	text string
	idx  int
}

func (m Model) settingLines(w int) []settingLine {
	st := m.styles()
	var out []settingLine
	group := ""
	for i, d := range settingDefs {
		if d.group != group {
			group = d.group
			if len(out) > 0 {
				out = append(out, settingLine{"", -1})
			}
			out = append(out, settingLine{st.Accent2.Render(strings.ToUpper(group)), -1})
		}
		val := m.setting(d.key)
		var shown string
		switch d.kind {
		case "keys":
			shown = st.Dim.Render(fmt.Sprintf("enter to edit (%d changed)", len(m.km.user)))
		case "snippets":
			shown = st.Dim.Render("‹ ") + st.Accent.Render(m.snippetLang()) + st.Dim.Render(" ›  enter to edit")
		case "enum":
			shown = st.Dim.Render("‹ ") + st.Accent.Render(val) + st.Dim.Render(" ›")
		case "bool":
			if val == "true" {
				shown = st.Good.Render("● on")
			} else {
				shown = st.Dim.Render("○ off")
			}
		default:
			shown = val
			if m.setEdit != nil && i == m.setSel {
				shown = m.setEdit.text + st.Accent.Render("▏")
			} else if val == "" {
				shown = st.Dim.Render("(not set)")
			}
		}
		if e := m.setEdit; e != nil && e.cred > 0 && i == m.setSel {
			if e.cred == 1 {
				shown = st.Accent.Render("cookie ") + strings.Repeat("•", min(len([]rune(e.text)), 24)) + st.Accent.Render("▏")
			} else {
				shown = st.Accent.Render("user-agent ") + e.text + st.Accent.Render("▏")
			}
		}
		label := fit(d.label, 18)
		note := ""
		if !d.live {
			note = st.Dim.Render(" ↻")
		}
		out = append(out, settingLine{"  " + label + shown + note, i})
	}
	return out
}

func (m Model) viewSettings(b *strings.Builder) string {
	st := m.styles()
	h := m.paneHeight()
	w := m.contentWidth()
	lw := min(max(46, w*3/5), w-30)
	rw := w - lw - 1
	lines := m.settingLines(lw - 4)
	inner := h - 2
	// keep the selected row in view
	selAt := 0
	for i, l := range lines {
		if l.idx == m.setSel {
			selAt = i
		}
	}
	start := max(0, min(selAt-inner/2, len(lines)-inner))
	var body []string
	for i := start; i < min(len(lines), start+inner); i++ {
		l := lines[i]
		if l.idx == m.setSel {
			body = append(body, paintRow(" "+l.text, lw-4, st.Selected))
		} else {
			body = append(body, " "+l.text)
		}
	}
	left := box("Settings", body, "", lw, h, true, st, m.borderStyle())

	d := settingDefs[m.setSel]
	var info []string
	info = append(info, st.Accent.Render(d.label), "")
	info = append(info, wrap(m.kx(d.desc), rw-4)...)
	info = append(info, "")
	if d.live {
		info = append(info, st.Good.Render("applies immediately"))
	} else {
		info = append(info, st.Warn.Render("applies the next time verd starts"))
	}
	switch d.kind {
	case "keys":
		info = append(info, st.Dim.Render("enter opens the editor"))
	case "snippets":
		info = append(info, st.Dim.Render("←/→ language · enter edits its Template"), "", st.Accent2.Render("VARIABLES"))
		for _, v := range snippetVars {
			info = append(info, "  "+st.Accent.Render(v[0]), "    "+st.Dim.Render(v[1]))
		}
		info = append(info, "", st.Dim.Render("Go text/template: {{if eq .Handle \"x\"}}…{{end}} works too"))
	case "enum", "bool":
		info = append(info, st.Dim.Render("←/→ or space change · saved on change"))
	default:
		info = append(info, st.Dim.Render("enter edits · enter saves · esc cancels"))
	}
	if d.key == "theme" {
		info = append(info, "", st.Accent2.Render("THEMES"))
		info = append(info, m.themeGallery(rw-4)...)
	}
	if m.setNote != "" {
		style := st.Good
		if !strings.HasPrefix(m.setNote, "saved") {
			style = st.Bad
		}
		info = append(info, "", style.Render(clean(m.setNote)))
	}
	if len(m.deps.LangSummary) > 0 && d.group != "Appearance" {
		info = append(info, "", st.Accent2.Render("LANGUAGES"), st.Dim.Render("edit [lang.*] in config.toml (e)"))
		for _, l := range m.deps.LangSummary {
			info = append(info, "  "+clean(l))
		}
	}
	right := box("About", info, "", rw, h, false, st, m.borderStyle())
	for _, l := range joinCols(left, lw, right, " ") {
		b.WriteString(l + "\n")
	}
	return "{settings.down}/{settings.up} move  {settings.prev_value}/{settings.next_value} change  {settings.edit} edit  {settings.login} paste login (submit mode)  {settings.edit_config} open config.toml  {common.help} help  {settings.quit} quit"
}

// themeGallery lists every theme with its palette swatch, the current one marked.
func (m Model) themeGallery(w int) []string {
	st := m.styles()
	var out []string
	for _, n := range m.options("theme") {
		t, _ := theme.Get(n)
		p := t.Swatch(m.dark)
		var sw string
		for _, c := range []lipgloss.Style{
			lipgloss.NewStyle().Foreground(p.Accent), lipgloss.NewStyle().Foreground(p.Accent2),
			lipgloss.NewStyle().Foreground(p.Good), lipgloss.NewStyle().Foreground(p.Bad),
			lipgloss.NewStyle().Foreground(p.Warn), lipgloss.NewStyle().Foreground(p.Dim),
		} {
			sw += c.Render("██")
		}
		mark := "  "
		name := st.Dim.Render(fit(n, 14))
		if n == m.setting("theme") || (m.setting("theme") == "" && n == "terminal") {
			mark, name = st.Accent.Render("▸ "), st.Accent.Render(fit(n, 14))
		}
		out = append(out, mark+name+" "+sw)
	}
	return out
}

// wrap breaks text into lines of at most w columns at word boundaries.
func wrap(text string, w int) []string {
	var out []string
	cur := ""
	for _, word := range strings.Fields(text) {
		if cur != "" && len([]rune(cur))+1+len([]rune(word)) > w {
			out = append(out, cur)
			cur = ""
		}
		if cur != "" {
			cur += " "
		}
		cur += word
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// updateCred collects the cookie header, then the user-agent, and saves them for direct submit.
func (m Model) updateCred(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	e := m.setEdit
	switch msg.String() {
	case "esc":
		m.setEdit, m.setNote = nil, "cancelled: direct submit needs a session (verd login, or L here)"
	case "enter":
		text := strings.TrimSpace(e.text)
		if text == "" {
			return m, nil
		}
		if e.cred == 1 {
			m.setEdit = &settingEdit{cred: 2, cookie: text}
			return m, nil
		}
		m.setEdit = nil
		where, err := m.deps.SaveCreds(e.cookie, text)
		if err != nil {
			m.setNote = "login: " + err.Error()
		} else {
			m.setNote = "saved session to " + where + "; direct applies the next time verd starts"
		}
	case "backspace":
		if r := []rune(e.text); len(r) > 0 {
			e.text = string(r[:len(r)-1])
		}
	case "ctrl+u":
		e.text = ""
	case "ctrl+c":
		return m, tea.Quit
	default:
		if msg.Text != "" {
			e.text += clean(msg.Text)
		}
	}
	return m, nil
}
