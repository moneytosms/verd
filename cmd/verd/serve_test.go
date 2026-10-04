package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/moneytosms/verd/internal/ipc"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/tui"
)

var timing = regexp.MustCompile(`\d+ ms|\d+\.\d MB`)

var spaces = regexp.MustCompile(`\s+`)

// norm hides run-to-run numbers (times, memory) and the padding they cause.
func norm(s string) string { return spaces.ReplaceAllString(timing.ReplaceAllString(s, "<n>"), " ") }

func shortSock(t *testing.T) string {
	dir, err := os.MkdirTemp("", "vs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir + "/v.sock"
}

// The same run gives the same output and exit code whether it is delegated to a running TUI or run headless.
func TestDelegatedMatchesHeadless(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 missing")
	}
	d := &scrape.Detail{TimeLimitMS: 2000, MemoryLimitMB: 256, Samples: []scrape.Sample{{Input: "1 2\n", Output: "3\n"}, {Input: "5 5\n", Output: "10\n"}}}
	cfg, path, cache, detail := setup(t, "a,b=map(int,input().split())\nprint(a+b+(a==5))\n", "main.py", d) // sample-2 is WA
	detail = func(context.Context, int, string) (*scrape.Detail, error) { return d, nil }

	var headless bytes.Buffer
	hErr := testCmd(context.Background(), &headless, cfg, cache, detail, nil, path)

	sock := shortSock(t)
	srv, err := ipc.Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	attached := make(chan tui.ExternalRun, 1)
	go srv.Serve(testHandler(cfg, cache, detail, nil, func(r tui.ExternalRun) { attached <- r }))

	var delegated bytes.Buffer
	handled, dErr := delegateTest(context.Background(), &delegated, sock, path)
	if !handled {
		t.Fatal("a listening TUI must handle the request")
	}
	if exitCode(hErr) != 1 || exitCode(dErr) != 1 {
		t.Fatalf("both must exit 1 on WA: headless=%v delegated=%v", hErr, dErr)
	}
	if norm(headless.String()) != norm(delegated.String()) {
		t.Fatalf("output differs\n--- headless\n%s--- delegated\n%s", headless.String(), delegated.String())
	}

	// the same run was attached to the TUI pane, with its events
	r := <-attached
	if r.Problem.ContestID != 1 || r.Problem.Index != "A" {
		t.Fatalf("attached %+v", r.Problem)
	}
	var kinds []runner.EventKind
	for ev := range r.Events {
		kinds = append(kinds, ev.Kind)
	}
	if len(kinds) < 4 || kinds[len(kinds)-1] != runner.Done {
		t.Fatalf("pane should receive the whole run: %v", kinds)
	}
}

func TestDelegateErrorsAndFallback(t *testing.T) {
	cfg, path, cache, _ := setup(t, "", "main.py", nil)
	// nobody listening: not handled, so the caller runs headless
	if handled, err := delegateTest(context.Background(), &bytes.Buffer{}, shortSock(t), path); handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	// server-side failure (interactive Problem) becomes exit code 2 with the message
	sock := shortSock(t)
	srv, _ := ipc.Listen(sock)
	defer srv.Close()
	inter := func(context.Context, int, string) (*scrape.Detail, error) {
		return &scrape.Detail{Interactive: true}, nil
	}
	go srv.Serve(testHandler(cfg, cache, inter, nil, func(tui.ExternalRun) { t.Error("must not attach a refused run") }))
	_, err := delegateTest(context.Background(), &bytes.Buffer{}, sock, path)
	if exitCode(err) != 2 || err.Error() == "" {
		t.Fatalf("want exit 2 with a message, got %v", err)
	}
}
