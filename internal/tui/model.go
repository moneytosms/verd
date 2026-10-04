// Package tui is the root Bubble Tea model.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/theme"
)

var tabs = []string{"Problems", "Contests", "Stats", "Picker"}

// Deps are the side effects the model needs; nil funcs are no-ops.
type Deps struct {
	// Load returns a Problem's detail, cache-first unless force.
	Load    func(p cf.Problem, force bool) (*scrape.Detail, error)
	OpenURL func(url string) error
	Now     func() time.Time // defaults to time.Now
	// Edit creates the Problem's Solution if needed and opens it in the editor.
	// A nil cmd with a nil error means the editor opened in a split pane (see EditorAlive).
	Edit func(p cf.Problem, lang string) (*exec.Cmd, error)
	// AddCustom creates the next Custom Test files and opens them in the editor (like Edit).
	AddCustom func(p cf.Problem) (*exec.Cmd, error)
	// Ensure creates the Problem's Solution in lang from its Template if absent.
	Ensure func(p cf.Problem, lang string) error
	// Langs are the configured language keys (sorted); DefaultLang is used until the user picks one.
	Langs       []string
	DefaultLang string
	// EditorAlive reports whether the split-pane editor is still open; polled every 500 ms.
	EditorAlive func() bool
	// Tests starts a Test Run of the Problem's Solution and streams its events.
	Tests func(ctx context.Context, p cf.Problem, d *scrape.Detail, o RunOpts) (<-chan runner.Event, error)
	// LoadState and SaveState persist per-Problem choices (Comparison Mode).
	LoadState func(p cf.Problem) ProblemState
	SaveState func(p cf.Problem, s ProblemState) error
	// Refresh stages run in order in the background; each syncs stale data from the network and
	// returns the data now in the cache (valid even when err != nil, e.g. offline). A network
	// failure (cf.ErrNetwork) skips the remaining stages.
	Refresh []func() (Data, error)
}

// ProblemState is the user's per-Problem choices; empty fields mean "use the default".
type ProblemState struct{ Lang, Mode string }

// RunOpts are the per-run choices handed to Deps.Tests.
type RunOpts struct {
	Lang string
	Mode runner.Mode
}

// Data is everything the UI renders from the cache.
type Data struct {
	Handle   string
	Rating   int // latest rating, 0 = unrated
	Problems []cf.Problem
	Contests []cf.Contest
	Statuses map[string]store.Status
	SyncedAt time.Time // zero = never fully synced
}

type Model struct {
	theme    theme.Theme
	dark     bool // terminal background; dark until detected otherwise
	help     bool
	handle   string
	rating   int
	syncing  bool
	offline  bool
	syncedAt time.Time

	Problems []cf.Problem
	Notice   string // transient note shown in the footer
	deps     Deps
	status   map[string]store.Status

	tab           int
	contests      []cf.Contest
	upcoming      int // contests[:upcoming] are not finished
	contestCursor int
	contestOpen   *cf.Contest
	cpCursor      int

	// list filtering
	filter     Filter
	filterExpr string
	visible    []cf.Problem // Problems that pass filter
	input      *input       // active prompt, if any
	inputErr   string
	cursor     int
	width      int
	height     int

	// Problem view
	pstate     ProblemState
	run        *testRun
	editorOpen bool // a split-pane editor is open; a tick is polling it
	open       *cf.Problem
	detail     *scrape.Detail
	body       []string // rendered statement lines
	scroll     int
	loading    bool
	errMsg     string
}

func New(ps []cf.Problem, note string, deps Deps) Model {
	return Model{Problems: ps, visible: ps, Notice: note, deps: deps, width: 80, height: 24, syncing: len(deps.Refresh) > 0, dark: true, theme: mustTheme("terminal")}
}

func mustTheme(name string) theme.Theme { t, _ := theme.Get(name); return t }

// WithTheme selects a named theme; an unknown name keeps the default and sets a notice.
func (m Model) WithTheme(name string) Model {
	t, ok := theme.Get(name)
	m.theme = t
	if !ok {
		m.Notice = fmt.Sprintf("unknown theme %q (have: %s)", clean(name), strings.Join(theme.Names(), ", "))
	}
	return m
}

func (m Model) styles() theme.Styles { return m.theme.Styles(m.dark) }

// WithData replaces everything rendered from the cache, keeping view state.
func (m Model) WithData(d Data) Model {
	m.handle, m.rating, m.syncedAt = d.Handle, d.Rating, d.SyncedAt
	m.Problems, m.status = d.Problems, d.Statuses
	m = m.WithContests(d.Contests)
	return m.refilter()
}

type refreshedMsg struct {
	data  Data
	err   error
	stage int // index of the stage that just finished
}

func (m Model) refresh(stage int) tea.Cmd {
	r := m.deps.Refresh[stage]
	return func() tea.Msg {
		d, err := r()
		return refreshedMsg{d, err, stage}
	}
}

// WithStatuses sets the user's solved/attempted marks (keyed "<contest><index>").
func (m Model) WithStatuses(st map[string]store.Status) Model {
	m.status = st
	return m.refilter()
}

func (m Model) statusOf(p cf.Problem) store.Status {
	return m.status[fmt.Sprintf("%d%s", p.ContestID, p.Index)]
}

func (m Model) refilter() Model {
	m.visible = make([]cf.Problem, 0, len(m.Problems))
	for _, p := range m.Problems {
		if m.filter.Match(p, m.statusOf(p)) {
			m.visible = append(m.visible, p)
		}
	}
	m.cursor = min(m.cursor, max(0, len(m.visible)-1))
	return m
}

// input is a one-line prompt for the filter (`f`) or search (`/`).
type input struct {
	kind byte // 'f' or '/'
	text string
}

func (m Model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input, m.inputErr = nil, ""
	case "enter":
		if m.input.kind == '/' {
			m.filter.Search = m.input.text
		} else {
			f, err := ParseFilter(m.input.text)
			if err != nil {
				m.inputErr = err.Error()
				return m, nil
			}
			f.Search = m.filter.Search
			m.filter, m.filterExpr = f, m.input.text
		}
		m.input, m.inputErr = nil, ""
		m.cursor = 0
		return m.refilter(), nil
	case "backspace":
		if r := []rune(m.input.text); len(r) > 0 {
			m.input.text = string(r[:len(r)-1])
		}
	case "ctrl+u":
		m.input.text = ""
	case "ctrl+c":
		return m, tea.Quit
	default:
		if msg.Text != "" {
			m.input.text += clean(msg.Text)
		}
	}
	return m, nil
}

func (m Model) Init() tea.Cmd {
	if len(m.deps.Refresh) == 0 {
		return tea.RequestBackgroundColor
	}
	return tea.Batch(tea.RequestBackgroundColor, m.refresh(0))
}

type editorDoneMsg struct{ err error }

type editorTickMsg struct{}

func editorTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return editorTickMsg{} })
}

type detailMsg struct {
	p     cf.Problem
	d     *scrape.Detail
	body  string
	state ProblemState
	err   error
}

func (m Model) load(p cf.Problem, force bool) tea.Cmd {
	load, width, style, loadState := m.deps.Load, m.width, m.styles().Glamour, m.deps.LoadState
	return func() tea.Msg {
		if load == nil {
			return detailMsg{p: p, err: errors.New("loading unavailable")}
		}
		d, err := load(p, force)
		if err != nil {
			return detailMsg{p: p, err: err}
		}
		body, err := scrape.Render(d.Statement, width-4, style)
		var state ProblemState
		if loadState != nil {
			state = loadState(p)
		}
		return detailMsg{p: p, d: d, body: body, state: state, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
	case refreshedMsg:
		m = m.WithData(msg.data)
		m.offline = errors.Is(msg.err, cf.ErrNetwork)
		if msg.stage == 0 || msg.err != nil { // later stages must not erase an earlier stage's notice
			m.Notice = ""
			if msg.err != nil && !m.offline {
				m.Notice = "sync failed: " + msg.err.Error()
			}
		}
		if next := msg.stage + 1; !m.offline && next < len(m.deps.Refresh) {
			return m, m.refresh(next)
		}
		m.syncing = false
	case testEventMsg:
		return m.onTestEvent(msg)
	case editorTickMsg:
		if m.deps.EditorAlive != nil && m.deps.EditorAlive() {
			return m, editorTick()
		}
		m.editorOpen = false // the pane closed
	case editorDoneMsg:
		if msg.err != nil {
			m.errMsg = "editor: " + msg.err.Error()
		}
	case detailMsg:
		if m.open == nil || m.open.ContestID != msg.p.ContestID || m.open.Index != msg.p.Index {
			break // user already left or switched Problems
		}
		m.loading = false
		switch {
		case errors.Is(msg.err, cf.ErrNetwork):
			m.offline = true
			if m.detail == nil {
				m.errMsg = "offline: statement not cached. Press o to open in browser."
			} else {
				m.errMsg = "offline: could not refetch"
			}
		case errors.Is(msg.err, cf.ErrChallenge):
			m.errMsg = "Codeforces blocked the request (Cloudflare). Press o to open in browser."
		case msg.err != nil:
			m.errMsg = "load failed: " + msg.err.Error()
		default:
			m.errMsg, m.detail, m.scroll, m.pstate = "", msg.d, 0, msg.state
			m.body = strings.Split(msg.body, "\n")
		}
	case tea.KeyPressMsg:
		if m.help {
			switch msg.String() {
			case "?", "esc":
				m.help = false
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		if msg.String() == "?" && m.input == nil {
			m.help = true
			return m, nil
		}
		if m.open != nil {
			return m.updateProblem(msg)
		}
		if m.input != nil {
			return m.updateInput(msg)
		}
		switch k := msg.String(); k {
		case "ctrl+r":
			if len(m.deps.Refresh) == 0 || m.syncing {
				return m, nil
			}
			m.syncing = true
			return m, m.refresh(0)
		case "1", "2", "3", "4":
			m.tab = int(k[0] - '1')
			return m, nil
		case "tab":
			m.tab = (m.tab + 1) % len(tabs)
			return m, nil
		}
		switch m.tab {
		case 0:
			return m.updateList(msg)
		case 1:
			return m.updateContests(msg)
		}
		if k := msg.String(); k == "q" || k == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
	case "pgup":
		m.cursor = max(0, m.cursor-m.page())
	case "pgdown":
		m.cursor = min(len(m.visible)-1, m.cursor+m.page())
	case "f":
		m.input, m.inputErr = &input{kind: 'f', text: m.filterExpr}, ""
	case "/":
		m.input, m.inputErr = &input{kind: '/', text: m.filter.Search}, ""
	case "enter":
		if len(m.visible) == 0 {
			break
		}
		return m.openProblem(m.visible[m.cursor])
	}
	return m, nil
}

func (m Model) openProblem(p cf.Problem) (tea.Model, tea.Cmd) {
	m.open, m.detail, m.body, m.errMsg, m.scroll, m.loading = &p, nil, nil, "", 0, true
	return m, m.load(p, false)
}

func (m Model) updateProblem(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if r := m.run; r != nil && r.diff {
		switch msg.String() {
		case "esc", "d":
			r.diff = false
		case "j", "down":
			r.diffOff++
		case "k", "up":
			r.diffOff = max(0, r.diffOff-1)
		case "pgdown":
			r.diffOff += m.page()
		case "pgup":
			r.diffOff = max(0, r.diffOff-m.page())
		case "q", "ctrl+c":
			return m.stopTests(), tea.Quit
		}
		return m, nil
	}
	if nm, cmd, handled := m.updateTests(msg); handled {
		return nm, cmd
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m = m.stopTests()
		m.open, m.detail, m.body, m.errMsg, m.loading = nil, nil, nil, "", false
	case "r":
		if m.offline {
			m.errMsg = "offline: refetch needs network (ctrl+r on a list retries the connection)"
			break
		}
		m.loading, m.errMsg = true, ""
		return m, m.load(*m.open, true)
	case "c":
		if m.detail == nil {
			break
		}
		m.pstate.Mode = nextMode(m.mode())
		if m.deps.SaveState != nil {
			if err := m.deps.SaveState(*m.open, m.pstate); err != nil {
				m.errMsg = "saving mode: " + err.Error()
			}
		}
	case "l":
		if len(m.deps.Langs) == 0 || m.detail == nil {
			break
		}
		lang := nextOf(m.deps.Langs, m.lang())
		if m.deps.Ensure != nil {
			if err := m.deps.Ensure(*m.open, lang); err != nil {
				m.errMsg = "language " + lang + ": " + err.Error()
				break
			}
		}
		m.pstate.Lang = lang
		m.errMsg = ""
		if m.deps.SaveState != nil {
			if err := m.deps.SaveState(*m.open, m.pstate); err != nil {
				m.errMsg = "saving language: " + err.Error()
			}
		}
	case "e":
		if m.deps.Edit == nil {
			break
		}
		cmd, err := m.deps.Edit(*m.open, m.lang())
		return m.afterOpen(cmd, err, "edit")
	case "a":
		if m.deps.AddCustom == nil {
			break
		}
		cmd, err := m.deps.AddCustom(*m.open)
		return m.afterOpen(cmd, err, "add test")
	case "o":
		if open, p := m.deps.OpenURL, *m.open; open != nil {
			return m, func() tea.Msg {
				open(fmt.Sprintf("%s/problemset/problem/%d/%s", cf.BaseURL, p.ContestID, p.Index))
				return nil
			}
		}
	case "up", "k":
		m.scroll = max(0, m.scroll-1)
	case "down", "j":
		m.scroll = min(m.maxScroll(), m.scroll+1)
	case "pgup":
		m.scroll = max(0, m.scroll-m.page())
	case "pgdown":
		m.scroll = min(m.maxScroll(), m.scroll+m.page())
	}
	return m, nil
}

// content is the whole Problem view as lines: header, statement, Sample Tests.
func (m Model) content() []string {
	p := *m.open
	lines := []string{fmt.Sprintf("%d%s  %s", p.ContestID, clean(p.Index), clean(p.Name))}
	if m.detail != nil {
		d := m.detail
		lim := fmt.Sprintf("time %.4g s   memory %d MB", float64(d.TimeLimitMS)/1000, d.MemoryLimitMB)
		if d.Interactive {
			lim += "   [interactive: local run unsupported]"
		}
		lines = append(lines, lim, "")
		lines = append(lines, m.testsPanel()...)
		lines = append(lines, "")
		lines = append(lines, m.body...)
		for i, s := range d.Samples {
			lines = append(lines, "", fmt.Sprintf("Sample %d input:", i+1))
			lines = append(lines, cleanLines(s.Input)...)
			lines = append(lines, fmt.Sprintf("Sample %d output:", i+1))
			lines = append(lines, cleanLines(s.Output)...)
		}
	}
	return lines
}

var modes = []string{"tokens", "exact", "float", "none"}

func nextMode(cur string) string { return nextOf(modes, cur) }

// nextOf returns the entry after cur, wrapping; the first if cur is not in the list.
func nextOf(list []string, cur string) string {
	for i, x := range list {
		if x == cur {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

// lang is the language in effect: the user's choice if still configured, else the default.
func (m Model) lang() string {
	for _, l := range m.deps.Langs {
		if l == m.pstate.Lang {
			return l
		}
	}
	return m.deps.DefaultLang
}

// afterOpen handles the result of opening files in the editor: an error message, a split pane
// (nil cmd: poll until it closes), or a foreground editor to suspend for.
func (m Model) afterOpen(cmd *exec.Cmd, err error, what string) (tea.Model, tea.Cmd) {
	if err != nil {
		m.errMsg = what + ": " + err.Error()
		return m, nil
	}
	m.errMsg = ""
	if cmd == nil {
		if m.editorOpen || m.deps.EditorAlive == nil {
			return m, nil
		}
		m.editorOpen = true
		return m, editorTick()
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{err} })
}

// mode is the Comparison Mode in effect: the user's choice, else the Problem's parsed hint.
func (m Model) mode() string {
	if m.pstate.Mode != "" {
		return m.pstate.Mode
	}
	if m.detail != nil {
		return m.detail.DefaultMode()
	}
	return "tokens"
}

func (m Model) maxScroll() int { return max(0, len(m.content())-m.page()) }

// clean drops control characters (ESC etc.) so server-supplied text can't inject terminal escapes.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func cleanLines(s string) []string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range ls {
		ls[i] = clean(ls[i])
	}
	return ls
}

func (m Model) page() int { return max(1, m.height-4) }

func (m Model) View() tea.View {
	st := m.styles()
	var b strings.Builder
	names := make([]string, len(tabs))
	for i, t := range tabs {
		names[i] = t
		if i == m.tab {
			names[i] = st.Accent.Render("[" + t + "]")
		}
	}
	b.WriteString(" " + strings.Join(names, "  "))
	if m.handle != "" {
		who := clean(m.handle)
		if m.rating > 0 {
			who += fmt.Sprintf(" (%d)", m.rating)
		}
		b.WriteString("   " + st.Dim.Render(who))
	}
	b.WriteString("\n\n")
	footer := ""
	switch {
	case m.help:
		footer = m.viewHelp(&b)
	case m.open != nil:
		footer = m.viewProblem(&b)
	case m.tab == 0:
		footer = m.viewList(&b)
	case m.tab == 1:
		footer = m.viewContests(&b)
	default:
		b.WriteString("  " + tabs[m.tab] + ": coming soon\n")
		footer = "1-4 tabs  ? help  q quit"
	}
	footer = st.Dim.Render(footer)
	if m.Notice != "" {
		footer += "  " + st.Warn.Render("["+clean(m.Notice)+"]")
	}
	if m.editorOpen {
		footer += "  " + st.Dim.Render("[editor open]")
	}
	switch {
	case m.syncing:
		footer += "  " + st.Warn.Render("[syncing...]")
	case m.offline && m.syncedAt.IsZero():
		footer += "  " + st.Warn.Render("[offline, never synced]")
	case m.offline:
		footer += "  " + st.Warn.Render(fmt.Sprintf("[offline, synced %s ago]", ago(m.clock().Sub(m.syncedAt))))
	}
	b.WriteString("\n" + footer)
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// keys lists the current screen's key bindings for the help overlay.
func (m Model) keys() (screen string, keys [][2]string) {
	global := [][2]string{{"1-4 / tab", "switch tab"}, {"ctrl+r", "refresh from network"}, {"?", "toggle help"}, {"q", "quit"}}
	switch {
	case m.open != nil:
		return "Problem", [][2]string{{"j/k, pgup/pgdn", "scroll"}, {"e", "edit Solution in Neovim"}, {"a", "add Custom Test"}, {"l", "switch language"}, {"t", "run tests"}, {"c", "cycle Comparison Mode (tokens, exact, float, none)"}, {"n/p", "select test"}, {"d", "diff selected failing test"}, {"r", "refetch statement"}, {"o", "open in browser"}, {"esc", "back"}, {"?", "toggle help"}, {"q", "quit"}}
	case m.input != nil:
		return "Prompt", [][2]string{{"enter", "apply"}, {"esc", "cancel"}, {"ctrl+u", "clear"}}
	case m.tab == 0:
		return "Problems", append([][2]string{{"j/k, pgup/pgdn", "move"}, {"enter", "open Problem"}, {"f", "filter: 800-1200 +dp -graphs unsolved"}, {"/", "search by ID or name"}}, global...)
	case m.tab == 1 && m.contestOpen != nil:
		return "Contest", [][2]string{{"j/k", "move"}, {"enter", "open Problem"}, {"esc", "back to contests"}, {"?", "toggle help"}, {"q", "quit"}}
	case m.tab == 1:
		return "Contests", append([][2]string{{"j/k, pgup/pgdn", "move"}, {"enter", "list Problems"}}, global...)
	}
	return tabs[m.tab], global
}

func (m Model) viewHelp(b *strings.Builder) string {
	screen, keys := m.keys()
	st := m.styles()
	b.WriteString(" " + st.Accent.Render("Keys: "+screen) + "\n\n")
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("  %-16s %s\n", k[0], k[1]))
	}
	return "? or esc closes"
}

func (m Model) viewProblem(b *strings.Builder) string {
	if m.run != nil && m.run.diff {
		return m.diffView(b)
	}
	lines := m.content()
	if m.loading {
		lines = append(lines, "", "loading...")
	}
	if m.errMsg != "" {
		lines = append(lines, "", m.styles().Bad.Render(clean(m.errMsg)))
	}
	end := min(len(lines), m.scroll+m.page())
	for _, l := range lines[min(m.scroll, end):end] {
		b.WriteString(" " + l + "\n")
	}
	return "esc back  e edit  a add test  l lang  t test  c mode  j/k scroll  r refetch  o browser  ? help  q quit"
}

func (m Model) viewList(b *strings.Builder) string {
	st := m.styles()
	b.WriteString(st.Dim.Render(fmt.Sprintf("    %-8s %-40s %6s %7s  %s", "ID", "Name", "Rating", "Solved", "Tags")) + "\n")
	rows := m.page() - 1
	start := max(0, min(m.cursor-rows/2, len(m.visible)-rows))
	for i := start; i < min(start+rows, len(m.visible)); i++ {
		p := m.visible[i]
		cur := " "
		if i == m.cursor {
			cur = st.Accent.Render(">")
		}
		mark, rating := m.markOf(m.statusOf(p)), ratingStr(p)
		b.WriteString(fmt.Sprintf("%s %s %-8s %-40.40s %6s %7d  %s\n", cur, mark, fmt.Sprintf("%d%s", p.ContestID, clean(p.Index)), clean(p.Name), rating, p.SolvedCount, clean(strings.Join(p.Tags, ", "))))
	}
	if m.input != nil {
		b.WriteString(fmt.Sprintf("\n%c %s_", m.input.kind, m.input.text))
		if m.inputErr != "" {
			b.WriteString("   " + st.Bad.Render(m.inputErr))
		}
		return "enter apply  esc cancel"
	}
	count := fmt.Sprintf("%d problems", len(m.visible))
	if len(m.visible) != len(m.Problems) {
		count = fmt.Sprintf("%d/%d problems", len(m.visible), len(m.Problems))
	}
	var active []string
	if m.filterExpr != "" {
		active = append(active, "filter: "+clean(m.filterExpr))
	}
	if m.filter.Search != "" {
		active = append(active, "search: "+clean(m.filter.Search))
	}
	return count + "  " + strings.Join(active, "  ") + "  f filter  / search  enter open  ? help  q quit"
}

func ago(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
