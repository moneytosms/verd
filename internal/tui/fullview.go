package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// fullBox is the selected test (input, expected, stdout, stderr) in a near full-screen modal.
// stderr is only shown, never compared.
func (m Model) fullBox() []string {
	w, h := m.modalSize(m.width, m.height)
	title := "Test"
	var body []string
	if lines, t := m.detailLines(w - 4); len(lines) > 0 {
		title = t
		for _, l := range lines {
			body = append(body, strings.Split(xansi.Hardwrap(l, w-4, true), "\n")...)
		}
	}
	rows := max(1, h-2)
	off := min(m.fullOff, max(0, len(body)-rows))
	end := min(len(body), off+rows)
	note := m.kx(fmt.Sprintf("%d-%d/%d  {problem.down}/{problem.up} scroll  {problem.back} close", min(off+1, len(body)), end, len(body)))
	return box(title, body[off:end], note, w, h, true, m.styles(), m.borderStyle())
}

// updateFull handles keys while the full-screen test view is open.
func (m Model) updateFull(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "f", "q":
		m.full, m.fullOff = false, 0
	case "j", "down":
		m.fullOff++
	case "k", "up":
		m.fullOff = max(0, m.fullOff-1)
	case "pgdown":
		m.fullOff += m.page()
	case "pgup":
		m.fullOff = max(0, m.fullOff-m.page())
	case "home", "g":
		m.fullOff = 0
	case "end", "G":
		m.fullOff = 1 << 30 // clamped when drawn
	case "ctrl+c":
		return m.stopTests(), tea.Quit
	}
	return m, nil
}
