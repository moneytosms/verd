package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/runner"
)

func sockPath(t *testing.T) string {
	// t.TempDir() paths can exceed the socket limit; use a short dir
	dir, err := os.MkdirTemp("", "vi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "v.sock")
}

func TestRoundTripStreamsEvents(t *testing.T) {
	path := sockPath(t)
	srv, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(func(ctx context.Context, req Request, emit func(Message)) error {
		if req.Cmd != "test" || req.File != "/w/1/A/main.cpp" {
			return errors.New("bad request")
		}
		emit(Message{Kind: "header", Text: "1A cpp"})
		emit(FromEvent(runner.Event{Kind: runner.CompileFinished, Cached: true}))
		emit(FromEvent(runner.Event{Kind: runner.TestFinished, Result: runner.Result{Name: "sample-1", Verdict: "WA", Mismatch: &runner.Mismatch{Line: 2, Col: 3, Want: "a", Got: "b"}}}))
		emit(FromEvent(runner.Event{Kind: runner.Done, Verdict: "WA"}))
		return nil
	})
	var got []Message
	dialed, err := Call(context.Background(), path, Request{Cmd: "test", File: "/w/1/A/main.cpp"}, func(m Message) { got = append(got, m) })
	if !dialed || err != nil {
		t.Fatalf("dialed=%v err=%v", dialed, err)
	}
	if len(got) != 4 || got[0].Text != "1A cpp" || got[3].Verdict != "WA" {
		t.Fatalf("%+v", got)
	}
	ev, ok := got[2].ToEvent()
	if !ok || ev.Result.Mismatch == nil || ev.Result.Mismatch.Got != "b" || ev.Result.Name != "sample-1" {
		t.Fatalf("result must survive the wire: %+v", ev)
	}
	if ev, ok := got[1].ToEvent(); !ok || !ev.Cached {
		t.Fatal("compile event")
	}
	// handler errors reach the client as an error message
	srv2path := sockPath(t)
	srv2, _ := Listen(srv2path)
	defer srv2.Close()
	go srv2.Serve(func(context.Context, Request, func(Message)) error { return errors.New("no such Problem") })
	got = nil
	Call(context.Background(), srv2path, Request{Cmd: "test"}, func(m Message) { got = append(got, m) })
	if len(got) != 1 || got[0].Kind != "error" || got[0].Text != "no such Problem" {
		t.Fatalf("%+v", got)
	}
}

func TestNoServerMeansNotDialed(t *testing.T) {
	dialed, err := Call(context.Background(), sockPath(t), Request{Cmd: "test"}, func(Message) { t.Fatal("no messages") })
	if dialed || err != nil {
		t.Fatalf("dialed=%v err=%v", dialed, err)
	}
}

func TestSecondListenRefusedAndStaleCleaned(t *testing.T) {
	path := sockPath(t)
	srv, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(path); !errors.Is(err, ErrRunning) || !strings.Contains(err.Error(), path) {
		t.Fatalf("second instance must be refused with the socket path: %v", err)
	}
	srv.Close()
	if _, err := os.Stat(path); err == nil {
		t.Fatal("Close must remove the socket")
	}

	// stale socket file from a crashed process: nobody listening, file still there
	ln, _ := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	ln.SetUnlinkOnClose(false)
	ln.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("test setup: stale socket should exist")
	}
	srv, err = Listen(path)
	if err != nil {
		t.Fatalf("stale socket must be cleaned up, got %v", err)
	}
	srv.Close()
}

func TestClientDisconnectCancelsRun(t *testing.T) {
	path := sockPath(t)
	srv, _ := Listen(path)
	defer srv.Close()
	cancelled := make(chan struct{})
	go srv.Serve(func(ctx context.Context, req Request, emit func(Message)) error {
		emit(Message{Kind: "header", Text: "start"})
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	go Call(ctx, path, Request{Cmd: "test"}, func(Message) {})
	time.Sleep(100 * time.Millisecond)
	cancel() // like Ctrl-C on `verd test`
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("server run was not cancelled when the client left")
	}
}

func TestPathTooLong(t *testing.T) {
	if _, err := Listen("/tmp/" + strings.Repeat("x", 120) + "/v.sock"); err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("%v", err)
	}
}
