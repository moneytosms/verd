// Package tui is the root Bubble Tea model.
package tui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
)

var tabs = []string{"Problems", "Contests", "Stats", "Picker"}

type Model struct {
	Problems []cf.Problem
	Offline  string // non-empty = status note shown in the footer
	cursor   int
	height   int
}

func New(ps []cf.Problem, note string) Model { return Model{Problems: ps, Offline: note, height: 24} }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyPressMsg:
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
		}
	}
	return m, nil
}

// clean drops control characters (ESC etc.) so server-supplied text can't inject terminal escapes.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func (m Model) page() int { return max(1, m.height-4) }

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(" " + strings.Join(tabs, "  |  ") + "\n\n")
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
	footer := fmt.Sprintf("%d problems  q quit", len(m.Problems))
	if m.Offline != "" {
		footer += "  [" + clean(m.Offline) + "]"
	}
	b.WriteString("\n" + footer)
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
