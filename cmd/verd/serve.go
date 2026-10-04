package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/ipc"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/tui"
)

func socketPath() string { return filepath.Join(config.RuntimeDir(), "verd.sock") }

// delegateTest streams a Test Run from a running TUI. handled=false means no TUI is listening
// and the caller should run headless.
func delegateTest(ctx context.Context, out io.Writer, sock, file string) (handled bool, err error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return true, err
	}
	rep := &reporter{out: out}
	var srvErr string
	dialed, err := ipc.Call(ctx, sock, ipc.Request{Cmd: "test", File: abs}, func(m ipc.Message) {
		switch m.Kind {
		case "header":
			rep.header(m.Text)
		case "error":
			srvErr = m.Text
		default:
			if ev, ok := m.ToEvent(); ok {
				rep.event(ev)
			}
		}
	})
	switch {
	case !dialed:
		return false, nil
	case err != nil:
		return true, err
	case srvErr != "":
		return true, &exitError{2, srvErr}
	}
	return true, rep.finish()
}

// testHandler serves `test` requests inside the TUI process: it runs the tests, streams events to
// the client, and attaches the same run to the TUI so the verd pane shows it too.
func testHandler(cfg config.Config, cacheDir string, detail detailFunc, savedMode modeFunc, attach func(tui.ExternalRun)) ipc.Handler {
	return func(ctx context.Context, req ipc.Request, emit func(ipc.Message)) error {
		if req.Cmd != "test" {
			return errors.New("unknown command " + req.Cmd)
		}
		prep, err := prepareTest(ctx, cfg, cacheDir, detail, savedMode, req.File)
		if err != nil {
			return err
		}
		emit(ipc.Message{Kind: "header", Text: prep.header})
		shown := make(chan runner.Event, 256)
		attach(tui.ExternalRun{Problem: cf.Problem{ContestID: prep.ref.Contest, Index: prep.ref.Index}, Events: shown})
		defer close(shown)
		for ev := range runner.Run(ctx, prep.spec) {
			if m := ipc.FromEvent(ev); m.Kind != "" {
				emit(m)
			}
			select {
			case shown <- ev:
			default: // the pane is only a view; never stall the client for it
			}
		}
		return nil
	}
}
