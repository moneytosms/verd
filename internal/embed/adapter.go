package embed

import (
	"sync"

	"github.com/moneytosms/verd/internal/mux"
)

// Messages the Adapter emits for the TUI (it forwards them with tea.Program.Send).
type (
	// OpenedMsg: a new embedded program started; the TUI should lay out a pane for Term.
	OpenedMsg struct{ Term *Term }
	// RedrawMsg: the screen changed or the program exited.
	RedrawMsg struct{}
	// FocusMsg: something asked for keyboard focus to move to the pane.
	FocusMsg struct{}
)

// Adapter is the third Mux adapter: it "opens a split" by starting the program in an embedded PTY
// that the TUI draws itself. Send must not block the caller (the TUI calls Controller.Open from
// its update loop), so it should hand messages to tea.Program.Send in a goroutine.
type Adapter struct {
	Send func(msg any)

	mu   sync.Mutex
	term *Term
}

var _ mux.Mux = (*Adapter)(nil)

func (a *Adapter) Name() string { return "embedded" }

// defaultSize is used until the TUI lays the pane out and resizes it.
const defaultW, defaultH = 80, 24

func (a *Adapter) OpenEditor(cwd string, argv []string) (mux.Pane, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.term != nil {
		a.term.Close()
	}
	t, err := Start(cwd, argv, defaultW, defaultH, func() { a.Send(RedrawMsg{}) })
	if err != nil {
		return mux.Pane{}, err
	}
	a.term = t
	a.Send(OpenedMsg{Term: t})
	return mux.Pane{ID: "embedded"}, nil
}

func (a *Adapter) Alive(mux.Pane) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.term != nil && a.term.Alive()
}

func (a *Adapter) Focus(mux.Pane) error {
	a.Send(FocusMsg{})
	return nil
}

func (a *Adapter) Close(mux.Pane) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.term != nil {
		a.term.Close()
	}
	return nil
}
