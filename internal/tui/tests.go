package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/runner"
)

// testRun is the state of the latest Test Run for the open Problem.
type testRun struct {
	id       int
	cancel   context.CancelFunc
	running  bool
	compile  string // "", "compiling...", "ok", "ok (cached)", or compiler output
	results  []runner.Result
	overall  string
	diff     bool
	diffOff  int
	diffInit bool
}

type testEventMsg struct {
	id     int
	ev     runner.Event
	ch     <-chan runner.Event
	closed bool
}

func listen(id int, ch <-chan runner.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return testEventMsg{id: id, ev: ev, ch: ch, closed: !ok}
	}
}

// startTests begins a Test Run (cancelling any previous one) and returns the cmd that streams its events.
func (m Model) startTests() (Model, tea.Cmd) {
	switch {
	case m.deps.Tests == nil || m.detail == nil:
		return m, nil
	case m.detail.Interactive:
		m.errMsg = "interactive Problem: local run unsupported"
		return m, nil
	}
	if m.run != nil && m.run.cancel != nil {
		m.run.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.deps.Tests(ctx, *m.open, m.detail, RunOpts{Lang: m.lang(), Mode: runner.Mode(m.mode())})
	if err != nil {
		cancel()
		m.errMsg = "tests: " + err.Error()
		return m, nil
	}
	id := 1
	if m.run != nil {
		id = m.run.id + 1
	}
	m.errMsg = ""
	m.run = &testRun{id: id, cancel: cancel, running: true}
	m = m.loadCases()
	return m, listen(id, ch)
}

// attachRun shows a Test Run started elsewhere (the server owns its lifetime).
func (m Model) attachRun(ch <-chan runner.Event) (Model, tea.Cmd) {
	if m.run != nil && m.run.cancel != nil {
		m.run.cancel()
	}
	id := 1
	if m.run != nil {
		id = m.run.id + 1
	}
	m.errMsg = ""
	m.run = &testRun{id: id, running: true}
	return m, listen(id, ch)
}

func (m Model) stopTests() Model {
	if m.run != nil && m.run.cancel != nil {
		m.run.cancel()
	}
	m.run = nil
	return m
}

func (m Model) onTestEvent(msg testEventMsg) (Model, tea.Cmd) {
	r := m.run
	if r == nil || msg.id != r.id {
		return m, nil // stale: that run was replaced or the Problem closed
	}
	if msg.closed {
		r.running = false
		return m, nil
	}
	switch ev := msg.ev; ev.Kind {
	case runner.CompileStarted:
		r.compile = "compiling..."
	case runner.CompileFinished:
		switch {
		case ev.Err != "":
			r.compile = ev.Err
		case ev.Interpreted:
			r.compile = "ok (interpreted)"
		case ev.Cached:
			r.compile = "ok (cached)"
		default:
			r.compile = "ok"
		}
	case runner.TestFinished:
		r.results = append(r.results, ev.Result)
	case runner.Done:
		r.overall = ev.Verdict
	}
	return m, listen(msg.id, msg.ch)
}

func (m Model) updateTests(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	r := m.run
	switch msg.String() {
	case "t":
		nm, cmd := m.startTests()
		return nm, cmd, true
	case "n":
		m.tsel = min(m.tsel+1, max(0, len(m.rows())-1))
		return m, nil, true
	case "p":
		m.tsel = max(0, m.tsel-1)
		return m, nil, true
	case "d":
		if row, ok := m.selected(); ok && r != nil && row.Res != nil && row.Res.Verdict != runner.AC {
			r.diff, r.diffInit = true, false
		}
		return m, nil, r != nil
	}
	return m, nil, false
}

// tag is "lang, mode" (just the mode when no language is configured).
func (m Model) tag() string {
	if l := m.lang(); l != "" {
		return l + ", " + m.mode()
	}
	return m.mode()
}

// testsPanel renders the Tests panel lines.
func (m Model) testsPanel() []string {
	st := m.styles()
	r := m.run
	if r == nil {
		return []string{st.Dim.Render(m.kx(fmt.Sprintf("Tests [%s]: press {problem.run_tests} to run (samples and custom tests), {problem.cycle_mode} mode, {problem.language} language", m.tag())))}
	}
	head := fmt.Sprintf("Tests [%s]", m.tag())
	switch {
	case r.running:
		head += "  running..."
	case r.overall != "":
		head += "  " + m.verdictStyle(r.overall)
	}
	lines := []string{st.Accent.Render(head)}
	if r.compile != "" {
		for i, l := range strings.Split(strings.TrimRight(r.compile, "\n"), "\n") {
			if i == 0 && (r.compile == "compiling..." || strings.HasPrefix(r.compile, "ok")) {
				lines = append(lines, "  compile: "+r.compile)
				break
			}
			if i == 0 {
				lines = append(lines, "  compile: "+m.verdictStyle("CE"))
			}
			lines = append(lines, "    "+clean(l))
		}
	}
	var maxMS int
	var maxMB float64
	passed := 0
	for i, res := range r.results {
		cur := "  "
		if i == m.tsel {
			cur = st.Accent.Render("> ")
		}
		line := fmt.Sprintf("%s%-10s %s %6d ms %7.1f MB", cur, res.Name, m.verdictStyle(res.Verdict), res.TimeMS, res.MemoryMB)
		if res.Mismatch != nil {
			line += fmt.Sprintf("  line %d col %d: want %q got %q", res.Mismatch.Line, res.Mismatch.Col, clean(clip(res.Mismatch.Want, 20)), clean(clip(res.Mismatch.Got, 20)))
		} else if res.Note != "" {
			line += "  " + clean(res.Note)
		}
		lines = append(lines, line)
		maxMS, maxMB = max(maxMS, res.TimeMS), max(maxMB, res.MemoryMB)
		if res.Verdict == runner.AC {
			passed++
		}
	}
	if !r.running && len(r.results) > 0 {
		lines = append(lines, st.Dim.Render(fmt.Sprintf("  %d/%d AC  max %d ms  max %.1f MB   n/p select  d diff", passed, len(r.results), maxMS, maxMB)))
	}
	return lines
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func (m Model) verdictStyle(v string) string {
	st := m.styles()
	switch v {
	case runner.AC:
		return st.Good.Render(fmt.Sprintf("%-3s", v))
	case runner.Unk:
		return st.Dim.Render(fmt.Sprintf("%-3s", v))
	}
	return st.Bad.Render(fmt.Sprintf("%-3s", v))
}

// cell returns line i of ls, padded or clipped to w columns.
func cell(ls []string, i, w int) string {
	s := ""
	if i < len(ls) {
		s = ls[i]
	}
	r := []rune(s)
	if len(r) > w {
		r = append(r[:w-1], '…')
	}
	return string(r) + strings.Repeat(" ", max(0, w-len(r)))
}
