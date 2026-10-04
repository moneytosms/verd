// Package tui is the root Bubble Tea model.
package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
)

var tabs = []string{"Problems", "Contests", "Stats", "Picker"}

// Deps are the side effects the model needs; nil funcs are no-ops.
type Deps struct {
	// Load returns a Problem's detail, cache-first unless force.
	Load    func(p cf.Problem, force bool) (*scrape.Detail, error)
	OpenURL func(url string) error
	Now     func() time.Time // defaults to time.Now
	// Refresh stages run in order in the background; each syncs stale data from the network and
	// returns the data now in the cache (valid even when err != nil, e.g. offline). A network
	// failure (cf.ErrNetwork) skips the remaining stages.
	Refresh []func() (Data, error)
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
	open    *cf.Problem
	detail  *scrape.Detail
	body    []string // rendered statement lines
	scroll  int
	loading bool
	errMsg  string
}

func New(ps []cf.Problem, note string, deps Deps) Model {
	return Model{Problems: ps, visible: ps, Notice: note, deps: deps, width: 80, height: 24, syncing: len(deps.Refresh) > 0}
}

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
		return nil
	}
	return m.refresh(0)
}

type detailMsg struct {
	p    cf.Problem
	d    *scrape.Detail
	body string
	err  error
}

func (m Model) load(p cf.Problem, force bool) tea.Cmd {
	load, width := m.deps.Load, m.width
	return func() tea.Msg {
		if load == nil {
			return detailMsg{p: p, err: errors.New("loading unavailable")}
		}
		d, err := load(p, force)
		if err != nil {
			return detailMsg{p: p, err: err}
		}
		body, err := scrape.Render(d.Statement, width-4)
		return detailMsg{p: p, d: d, body: body, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
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
			m.errMsg, m.detail, m.scroll = "", msg.d, 0
			m.body = strings.Split(msg.body, "\n")
		}
	case tea.KeyPressMsg:
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
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.open, m.detail, m.body, m.errMsg, m.loading = nil, nil, nil, "", false
	case "r":
		if m.offline {
			m.errMsg = "offline: refetch needs network (ctrl+r on a list retries the connection)"
			break
		}
		m.loading, m.errMsg = true, ""
		return m, m.load(*m.open, true)
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
	var b strings.Builder
	names := make([]string, len(tabs))
	for i, t := range tabs {
		names[i] = t
		if i == m.tab {
			names[i] = "[" + t + "]"
		}
	}
	b.WriteString(" " + strings.Join(names, "  ") + "\n\n")
	footer := ""
	switch {
	case m.open != nil:
		footer = m.viewProblem(&b)
	case m.tab == 0:
		footer = m.viewList(&b)
	case m.tab == 1:
		footer = m.viewContests(&b)
	default:
		b.WriteString("  " + tabs[m.tab] + ": coming soon\n")
		footer = "1-4 tabs  q quit"
	}
	if m.Notice != "" {
		footer += "  [" + clean(m.Notice) + "]"
	}
	switch {
	case m.syncing:
		footer += "  [syncing...]"
	case m.offline && m.syncedAt.IsZero():
		footer += "  [offline, never synced]"
	case m.offline:
		footer += fmt.Sprintf("  [offline, synced %s ago]", ago(m.clock().Sub(m.syncedAt)))
	}
	b.WriteString("\n" + footer)
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) viewProblem(b *strings.Builder) string {
	lines := m.content()
	if m.loading {
		lines = append(lines, "", "loading...")
	}
	if m.errMsg != "" {
		lines = append(lines, "", clean(m.errMsg))
	}
	end := min(len(lines), m.scroll+m.page())
	for _, l := range lines[min(m.scroll, end):end] {
		b.WriteString(" " + l + "\n")
	}
	return "esc back  j/k scroll  r refetch  o browser  q quit"
}

func (m Model) viewList(b *strings.Builder) string {
	b.WriteString(fmt.Sprintf("    %-8s %-40s %6s %7s  %s\n", "ID", "Name", "Rating", "Solved", "Tags"))
	rows := m.page() - 1
	start := max(0, min(m.cursor-rows/2, len(m.visible)-rows))
	for i := start; i < min(start+rows, len(m.visible)); i++ {
		p := m.visible[i]
		cur := " "
		if i == m.cursor {
			cur = ">"
		}
		mark, rating := markOf(m.statusOf(p)), ratingStr(p)
		b.WriteString(fmt.Sprintf("%s %s %-8s %-40.40s %6s %7d  %s\n", cur, mark, fmt.Sprintf("%d%s", p.ContestID, clean(p.Index)), clean(p.Name), rating, p.SolvedCount, clean(strings.Join(p.Tags, ", "))))
	}
	if m.input != nil {
		b.WriteString(fmt.Sprintf("\n%c %s_", m.input.kind, m.input.text))
		if m.inputErr != "" {
			b.WriteString("   " + m.inputErr)
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
	return count + "  " + strings.Join(active, "  ") + "  f filter  / search  enter open  q quit"
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
