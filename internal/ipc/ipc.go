// Package ipc is the Unix-socket protocol between a running verd TUI and `verd test`:
// newline-delimited JSON, one request per connection, events streamed back until Done.
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/moneytosms/verd/internal/runner"
)

// ErrRunning is returned by Listen when another verd already owns the socket.
var ErrRunning = errors.New("verd is already running")

// maxPath is the portable Unix socket path limit (macOS: 104 bytes including the NUL).
const maxPath = 100

type Request struct {
	Cmd  string `json:"cmd"` // "test" | "submit"
	File string `json:"file"`
}

// Message is one streamed event. Kind: header | compile | test | done | submit | error.
// A submit message carries a status line in Text; Final marks the last one.
type Message struct {
	Kind        string         `json:"kind"`
	Text        string         `json:"text,omitempty"` // header line or error message
	Cached      bool           `json:"cached,omitempty"`
	Interpreted bool           `json:"interpreted,omitempty"`
	Err         string         `json:"err,omitempty"` // compiler output on failure
	Result      *runner.Result `json:"result,omitempty"`
	Verdict     string         `json:"verdict,omitempty"`
	Final       bool           `json:"final,omitempty"` // submit: the Verdict is decided
}

// FromEvent converts a runner event to its wire form.
func FromEvent(ev runner.Event) Message {
	switch ev.Kind {
	case runner.CompileFinished:
		return Message{Kind: "compile", Cached: ev.Cached, Interpreted: ev.Interpreted, Err: ev.Err}
	case runner.TestFinished:
		r := ev.Result
		return Message{Kind: "test", Result: &r}
	case runner.Done:
		return Message{Kind: "done", Verdict: ev.Verdict}
	}
	return Message{} // CompileStarted carries nothing worth sending
}

// ToEvent is the inverse of FromEvent; ok is false for messages that are not runner events.
func (m Message) ToEvent() (runner.Event, bool) {
	switch m.Kind {
	case "compile":
		return runner.Event{Kind: runner.CompileFinished, Cached: m.Cached, Interpreted: m.Interpreted, Err: m.Err}, true
	case "test":
		if m.Result != nil {
			return runner.Event{Kind: runner.TestFinished, Result: *m.Result}, true
		}
	case "done":
		return runner.Event{Kind: runner.Done, Verdict: m.Verdict}, true
	}
	return runner.Event{}, false
}

// Handler serves one request, calling emit for each message. It should return when done or
// when ctx is cancelled (the client went away).
type Handler func(ctx context.Context, req Request, emit func(Message)) error

type Server struct {
	path string
	ln   net.Listener
	wg   sync.WaitGroup
}

// Listen claims the socket at path. A live owner yields ErrRunning; a stale socket file left by
// a dead process is removed.
func Listen(path string) (*Server, error) {
	if len(path) > maxPath {
		return nil, fmt.Errorf("socket path too long (%d > %d bytes): %s", len(path), maxPath, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		c.Close()
		return nil, fmt.Errorf("%w (socket %s)", ErrRunning, path)
	} else if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	os.Remove(path) // stale
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	os.Chmod(path, 0o600)
	return &Server{path: path, ln: ln}, nil
}

// Serve handles connections until Close. It blocks.
func (s *Server) Serve(h Handler) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer conn.Close()
			s.handle(conn, h)
		}()
	}
}

func (s *Server) handle(conn net.Conn, h Handler) {
	var req Request
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	enc := json.NewEncoder(conn)
	var mu sync.Mutex
	emit := func(m Message) {
		mu.Lock()
		defer mu.Unlock()
		if enc.Encode(m) != nil {
			cancel() // client gone: stop the run
		}
	}
	// a client that disconnects mid-run (Ctrl-C on `verd test`) cancels it
	go func() {
		buf := make([]byte, 1)
		conn.Read(buf)
		cancel()
	}()
	if err := h(ctx, req, emit); err != nil {
		emit(Message{Kind: "error", Text: err.Error()})
	}
}

// Close stops accepting, removes the socket and waits for in-flight handlers.
func (s *Server) Close() {
	s.ln.Close()
	os.Remove(s.path)
	s.wg.Wait()
}

// Call sends req to a running verd and returns its messages. dialed=false means nobody is
// listening (no TUI): the caller should fall back to running headless.
func Call(ctx context.Context, path string, req Request, onMsg func(Message)) (dialed bool, err error) {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false, nil
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return true, err
	}
	dec := json.NewDecoder(bufio.NewReader(conn))
	for {
		var m Message
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, os.ErrClosed) || ctx.Err() != nil {
				return true, ctx.Err()
			}
			return true, nil // server closed the stream
		}
		onMsg(m)
		if m.Kind == "done" || m.Kind == "error" || (m.Kind == "submit" && m.Final) {
			return true, nil
		}
	}
}
