package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/submit"
)

// SubmitStart is a submit that Deps.Submit has handed to the browser and is now tracking.
type SubmitStart struct {
	Text    string   // the Solution, for an OSC 52 clipboard copy
	Notes   []string // non-fatal: clipboard/opener problems, where to paste
	Direct  bool     // already sent to Codeforces: no paste step, and nothing to copy
	Updates <-chan submit.Update
}

// ExternalSubmit attaches a submit started elsewhere (`verd submit` over the socket).
type ExternalSubmit struct {
	Problem cf.Problem
	Start   SubmitStart
}

type subState struct {
	problem cf.Problem
	status  string
	notes   []string
	final   bool   // the Verdict is in
	ok      bool   // ...and it is Accepted
	detail  string // time and memory of the final Verdict
}

type toast struct {
	id   int
	text string
	bad  bool
}

type (
	submitStartedMsg struct {
		p     cf.Problem
		start SubmitStart
		err   error
	}
	subUpdateMsg struct {
		u      submit.Update
		ch     <-chan submit.Update
		closed bool
	}
	toastClearMsg struct{ id int }
	reloadMsg     struct{ data Data }
)

func listenSub(ch <-chan submit.Update) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		return subUpdateMsg{u: u, ch: ch, closed: !ok}
	}
}

// localTestsFailing reports a finished Test Run that was not all AC.
func (m Model) localTestsFailing() (string, bool) {
	r := m.run
	if r == nil || r.running || r.overall == "" || r.overall == runner.AC {
		return "", false
	}
	return r.overall, true
}

// trySubmit starts a submit for the open Problem, asking first if local tests are failing.
func (m Model) trySubmit() (Model, tea.Cmd) {
	if m.deps.Submit == nil {
		return m, nil
	}
	if m.offline {
		m.errMsg = "offline: submitting needs a connection to Codeforces"
		return m, nil
	}
	if _, bad := m.localTestsFailing(); bad {
		m.confirm = true
		return m, nil
	}
	return m.startSubmit()
}

func (m Model) startSubmit() (Model, tea.Cmd) {
	p, lang, submit := *m.open, m.lang(), m.deps.Submit
	m.confirm, m.errMsg = false, ""
	m.sub, m.subModal = &subState{problem: p, status: "preparing..."}, true
	return m, func() tea.Msg {
		st, err := submit(context.Background(), p, lang)
		return submitStartedMsg{p: p, start: st, err: err}
	}
}

// begin shows a started submit and returns the cmds that copy the Solution and follow the updates.
func (m Model) beginTracking(p cf.Problem, st SubmitStart) (Model, tea.Cmd) {
	status := "waiting for your submission on Codeforces..."
	if st.Direct {
		status = "submitted, waiting for the Verdict..."
	}
	m.sub, m.subModal = &subState{problem: p, status: status, notes: st.Notes}, true
	cmds := []tea.Cmd{listenSub(st.Updates)}
	if st.Text != "" && !st.Direct {
		cmds = append(cmds, tea.SetClipboard(st.Text))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) onSubmitMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case submitStartedMsg:
		if msg.err != nil {
			m.sub, m.subModal = nil, false
			m.errMsg = "submit: " + msg.err.Error()
			return m, nil, true
		}
		nm, cmd := m.beginTracking(msg.p, msg.start)
		return nm, cmd, true
	case ExternalSubmit:
		nm, cmd := m.beginTracking(msg.Problem, msg.Start)
		return nm, cmd, true
	case subUpdateMsg:
		if msg.closed || m.sub == nil {
			return m, nil, true
		}
		m.sub.status = msg.u.Text
		if !msg.u.Final {
			return m, listenSub(msg.ch), true
		}
		m.sub.final = true
		if s := msg.u.Submission; msg.u.Err == nil {
			m.sub.ok = s.Verdict == "OK"
			if s.TimeMS > 0 || s.MemoryBytes > 0 {
				m.sub.detail = fmt.Sprintf("%d ms  %.1f MB", s.TimeMS, float64(s.MemoryBytes)/(1<<20))
			}
		}
		id := fmt.Sprintf("%d%s", m.sub.problem.ContestID, m.sub.problem.Index)
		reload := m.reload()
		m.toastSeq++
		t := &toast{id: m.toastSeq}
		if s := msg.u.Submission; msg.u.Err == nil && s.Verdict == "OK" {
			t.text = fmt.Sprintf("✓ %s %s  %d ms  %.1f MB", msg.u.Text, id, s.TimeMS, float64(s.MemoryBytes)/(1<<20))
			m.toast = t
			return m, tea.Batch(reload, tea.Tick(6*time.Second, func(time.Time) tea.Msg { return toastClearMsg{t.id} })), true
		}
		t.text, t.bad = fmt.Sprintf("✗ %s (%s)  x dismiss", msg.u.Text, id), true
		m.toast = t // sticky: failures stay until dismissed
		return m, reload, true
	case reloadMsg:
		return m.WithData(msg.data), nil, true
	case toastClearMsg:
		if m.toast != nil && m.toast.id == msg.id {
			m.toast = nil
		}
		return m, nil, true
	}
	return m, nil, false
}

// reload re-reads the cache so solved/attempted marks include the new Submission.
func (m Model) reload() tea.Cmd {
	r := m.deps.Reload
	if r == nil {
		return nil
	}
	return func() tea.Msg {
		d, err := r()
		if err != nil {
			return nil
		}
		return reloadMsg{d}
	}
}

func (m Model) viewToast() string {
	if m.toast == nil {
		return ""
	}
	st := m.styles()
	if m.toast.bad {
		return st.Bad.Render(clean(m.toast.text))
	}
	return st.Good.Render(clean(m.toast.text))
}

// submissionLines is the Problem view's Submission status, shown while one is in flight or just ended.
func (m Model) submissionLines() []string {
	s := m.sub
	if s == nil || m.open == nil || s.problem.ContestID != m.open.ContestID || s.problem.Index != m.open.Index {
		return nil
	}
	lines := []string{m.styles().Accent.Render("Submission") + "  " + clean(s.status)}
	for _, n := range s.notes {
		lines = append(lines, "  "+m.styles().Dim.Render(clean(n)))
	}
	return lines
}

func (m Model) confirmText() string {
	v, _ := m.localTestsFailing()
	return fmt.Sprintf("local tests failing (%s); submit anyway? y/n", strings.TrimSpace(v))
}
