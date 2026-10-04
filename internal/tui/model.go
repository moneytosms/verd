// Package tui is the root Bubble Tea model.
package tui

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

var tabs = []string{"Problems", "Contests", "Stats", "Picker"}

// Deps are the side effects the model needs; nil funcs are no-ops.
type Deps struct {
	// Load returns a Problem's detail, cache-first unless force.
	Load    func(p cf.Problem, force bool) (*scrape.Detail, error)
	OpenURL func(url string) error
}

type Model struct {
	Problems []cf.Problem
	Offline  string // non-empty = status note shown in the footer
	deps     Deps
	cursor   int
	width    int
	height   int

	// Problem view
	open    *cf.Problem
	detail  *scrape.Detail
	body    []string // rendered statement lines
	scroll  int
	loading bool
	errMsg  string
}

func New(ps []cf.Problem, note string, deps Deps) Model {
	return Model{Problems: ps, Offline: note, deps: deps, width: 80, height: 24}
}

func (m Model) Init() tea.Cmd { return nil }

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
	case detailMsg:
		if m.open == nil || m.open.ContestID != msg.p.ContestID || m.open.Index != msg.p.Index {
			break // user already left or switched Problems
		}
		m.loading = false
		switch {
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
		return m.updateList(msg)
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
		if m.cursor < len(m.Problems)-1 {
			m.cursor++
		}
	case "pgup":
		m.cursor = max(0, m.cursor-m.page())
	case "pgdown":
		m.cursor = min(len(m.Problems)-1, m.cursor+m.page())
	case "enter":
		if len(m.Problems) == 0 {
			break
		}
		p := m.Problems[m.cursor]
		m.open, m.detail, m.body, m.errMsg, m.scroll, m.loading = &p, nil, nil, "", 0, true
		return m, m.load(p, false)
	}
	return m, nil
}

func (m Model) updateProblem(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.open, m.detail, m.body, m.errMsg, m.loading = nil, nil, nil, "", false
	case "r":
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
	b.WriteString(" " + strings.Join(tabs, "  |  ") + "\n\n")
	footer := ""
	if m.open != nil {
		footer = m.viewProblem(&b)
	} else {
		footer = m.viewList(&b)
	}
	if m.Offline != "" {
		footer += "  [" + clean(m.Offline) + "]"
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
	b.WriteString(fmt.Sprintf("  %-8s %-40s %6s %7s  %s\n", "ID", "Name", "Rating", "Solved", "Tags"))
	rows := m.page() - 1
	start := max(0, min(m.cursor-rows/2, len(m.Problems)-rows))
	for i := start; i < min(start+rows, len(m.Problems)); i++ {
		p := m.Problems[i]
		cur := "  "
		if i == m.cursor {
			cur = "> "
		}
		rating := "-"
		if p.Rating > 0 {
			rating = fmt.Sprint(p.Rating)
		}
		b.WriteString(fmt.Sprintf("%s%-8s %-40.40s %6s %7d  %s\n", cur, fmt.Sprintf("%d%s", p.ContestID, clean(p.Index)), clean(p.Name), rating, p.SolvedCount, clean(strings.Join(p.Tags, ", "))))
	}
	return fmt.Sprintf("%d problems  enter open  q quit", len(m.Problems))
}
