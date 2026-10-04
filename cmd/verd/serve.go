package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/ipc"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/submit"
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

// submitHandler serves `submit` requests: it runs the same flow as the `s` key, streams status
// lines to the client, and attaches the Submission to the TUI so its pane and toast show it.
func submitHandler(sub *submitter, attach func(tui.ExternalSubmit)) ipc.Handler {
	return func(ctx context.Context, req ipc.Request, emit func(ipc.Message)) error {
		p, lang, err := absSolutionProblem(sub.cfg, req.File)
		if err != nil {
			return err
		}
		st, err := sub.begin(ctx, p, lang)
		if err != nil {
			return err
		}
		for _, n := range st.Notes {
			emit(ipc.Message{Kind: "submit", Text: n})
		}
		shown := make(chan submit.Update, 8)
		attach(tui.ExternalSubmit{Problem: p, Start: tui.SubmitStart{Text: st.Text, Notes: st.Notes, Updates: shown}})
		defer close(shown)
		for u := range st.Updates {
			emit(ipc.Message{Kind: "submit", Text: u.Text, Final: u.Final, Verdict: u.Submission.Verdict, Err: errText(u.Err)})
			select {
			case shown <- u:
			default:
			}
		}
		return nil
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// dispatch routes requests by command.
func dispatch(handlers map[string]ipc.Handler) ipc.Handler {
	return func(ctx context.Context, req ipc.Request, emit func(ipc.Message)) error {
		h, ok := handlers[req.Cmd]
		if !ok {
			return errors.New("unknown command " + req.Cmd)
		}
		return h(ctx, req, emit)
	}
}

// delegateSubmit runs `verd submit` through a running TUI. handled=false: nobody listening.
func delegateSubmit(ctx context.Context, out io.Writer, sock, file string) (handled bool, err error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return true, err
	}
	var srvErr string
	var final ipc.Message
	dialed, err := ipc.Call(ctx, sock, ipc.Request{Cmd: "submit", File: abs}, func(m ipc.Message) {
		switch m.Kind {
		case "error":
			srvErr = m.Text
		case "submit":
			fmt.Fprintln(out, m.Text)
			if m.Final {
				final = m
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
	case final.Verdict == "OK":
		return true, nil
	case !final.Final:
		return true, &exitError{2, "tracking ended without a Verdict"}
	}
	return true, &exitError{1, ""}
}
