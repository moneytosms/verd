package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/stress"
)

// stressRun is the state of the latest stress run for the open Problem.
type stressRun struct {
	id       int
	cancel   context.CancelFunc
	running  bool
	iter     int
	res      *stress.Result
	saved    string // name of the Custom Test the counterexample was saved as
	diff     bool
	diffOff  int
	diffInit bool
}

type stressEventMsg struct {
	id     int
	ev     stress.Event
	ch     <-chan stress.Event
	closed bool
}

func listenStress(id int, ch <-chan stress.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		return stressEventMsg{id: id, ev: ev, ch: ch, closed: !ok}
	}
}

func (m Model) startStress() (Model, tea.Cmd) {
	switch {
	case m.deps.Stress == nil || m.detail == nil:
		return m, nil
	case m.detail.Interactive:
		m.errMsg = "interactive Problem: stress unsupported"
		return m, nil
	}
	m = m.stopStress()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.deps.Stress(ctx, *m.open, m.detail, RunOpts{Lang: m.lang(), Mode: runner.Mode(m.mode())})
	if err != nil {
		cancel()
		m.errMsg = "stress: " + err.Error()
		return m, nil
	}
	m.errMsg = ""
	m.strs = &stressRun{id: m.stressID, cancel: cancel, running: true}
	return m, listenStress(m.stressID, ch)
}

// attachStress shows a stress run started elsewhere (the server owns its lifetime).
func (m Model) attachStress(ch <-chan stress.Event) (Model, tea.Cmd) {
	m = m.stopStress()
	m.errMsg = ""
	m.strs = &stressRun{id: m.stressID, running: true}
	return m, listenStress(m.stressID, ch)
}

func (m Model) stopStress() Model {
	if m.strs != nil && m.strs.cancel != nil {
		m.strs.cancel()
	}
	m.stressID++ // late events of the old run are stale
	m.strs = nil
	return m
}

func (m Model) onStressEvent(msg stressEventMsg) (Model, tea.Cmd) {
	s := m.strs
	if s == nil || msg.id != s.id {
		return m, nil
	}
	if msg.closed {
		s.running = false
		return m, nil
	}
	s.iter = msg.ev.Iter
	if msg.ev.Done {
		res := msg.ev.Result
		s.running, s.res, s.iter = false, &res, res.Iterations
		s.diff = res.Kind == stress.Mismatch // the counterexample opens in the diff overlay
	}
	return m, listenStress(msg.id, msg.ch)
}

// counterexample is the failing case as a test result, so the diff overlay can show it.
func (s *stressRun) counterexample() runner.Result {
	r := s.res
	return runner.Result{Name: fmt.Sprintf("stress seed %d", r.Seed), Verdict: runner.WA, Expected: r.Want, Output: r.Got, Mismatch: r.Mismatch}
}

// saveStress stores the counterexample (brute output as the answer) as the next Custom Test.
func (m Model) saveStress() Model {
	s := m.strs
	if s == nil || s.res == nil || s.res.Kind != stress.Mismatch || s.saved != "" || m.deps.SaveCounterexample == nil {
		return m
	}
	name, err := m.deps.SaveCounterexample(*m.open, s.res.Input, s.res.Want)
	if err != nil {
		m.errMsg = "save: " + err.Error()
		return m
	}
	s.saved, m.errMsg = name, ""
	return m
}

// updateStressOverlay handles keys while the counterexample diff is open.
func (m Model) updateStressOverlay(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	s := m.strs
	switch msg.String() {
	case "esc", "d":
		s.diff = false
	case "w":
		m = m.saveStress()
	case "j", "down":
		s.diffOff++
	case "k", "up":
		s.diffOff = max(0, s.diffOff-1)
	case "pgdown":
		s.diffOff += m.page()
	case "pgup":
		s.diffOff = max(0, s.diffOff-m.page())
	case "q", "ctrl+c":
		return m.stopStress().stopTests(), tea.Quit
	}
	return m, nil
}

func (m Model) updateStress(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	s := m.strs
	switch msg.String() {
	case "S":
		nm, cmd := m.startStress()
		return nm, cmd, true
	case "esc":
		if s != nil && s.running {
			if s.cancel != nil {
				s.cancel()
			}
			s.running = false // an attached run is cancelled by its server; just stop watching
			m.stressID++
			m.strs.res = &stress.Result{Kind: stress.Cancelled, Iterations: s.iter}
			return m, nil, true
		}
	case "w":
		if s != nil && s.res != nil && s.res.Kind == stress.Mismatch {
			return m.saveStress(), nil, true
		}
	}
	return m, nil, false
}

func (m Model) stressPanel() []string {
	s := m.strs
	if s == nil {
		return nil
	}
	st := m.styles()
	head := fmt.Sprintf("Stress [%s]", m.tag())
	if s.running {
		return []string{st.Accent.Render(head + "  running..."), fmt.Sprintf("  iteration %d   esc cancel", s.iter)}
	}
	r := s.res
	if r == nil {
		return []string{st.Accent.Render(head), st.Dim.Render("  stopped")}
	}
	lines := []string{st.Accent.Render(head)}
	switch r.Kind {
	case stress.Limit:
		lines = append(lines, st.Good.Render(fmt.Sprintf("  no counterexample in %d iterations", r.Iterations)))
	case stress.Cancelled:
		lines = append(lines, st.Dim.Render(fmt.Sprintf("  cancelled after %d iterations", r.Iterations)))
	case stress.Error:
		for _, l := range strings.Split(strings.TrimRight(r.Err, "\n"), "\n") {
			lines = append(lines, "  "+st.Bad.Render(clean(l)))
		}
	case stress.Mismatch:
		lines = append(lines, st.Bad.Render(fmt.Sprintf("  mismatch at seed %d", r.Seed)))
		if s.saved != "" {
			lines = append(lines, "  saved as "+s.saved)
		} else {
			lines = append(lines, st.Dim.Render("  w save as Custom Test"))
		}
	default:
		lines = append(lines, st.Bad.Render(fmt.Sprintf("  %s at seed %d", r.Kind, r.Seed)))
	}
	return lines
}

func (s *stressRun) saveHint() string {
	if s.saved != "" {
		return "  saved as " + s.saved
	}
	return "  w save as Custom Test"
}
