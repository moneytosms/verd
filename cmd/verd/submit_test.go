package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/ipc"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/submit"
	"github.com/moneytosms/verd/internal/tui"
)

// fakeCF serves user.status: first call (baseline) has only the old submission, then the new one
// goes TESTING -> final.
func fakeCF(t *testing.T, final string) *httptest.Server {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		old := `{"id":100,"creationTimeSeconds":1,"programmingLanguage":"C++","verdict":"WRONG_ANSWER","passedTestCount":1,"problem":{"contestId":1900,"index":"A"}}`
		var items []string
		switch {
		case n == 1:
			items = []string{old}
		case n == 2:
			items = []string{`{"id":101,"creationTimeSeconds":2,"programmingLanguage":"C++","verdict":"TESTING","passedTestCount":2,"problem":{"contestId":1900,"index":"A"}}`, old}
		default:
			items = []string{fmt.Sprintf(`{"id":101,"creationTimeSeconds":2,"programmingLanguage":"C++","verdict":%q,"passedTestCount":5,"timeConsumedMillis":15,"memoryConsumedBytes":3145728,"problem":{"contestId":1900,"index":"A"}}`, final), old}
		}
		fmt.Fprintf(w, `{"status":"OK","result":[%s]}`, strings.Join(items, ","))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type ran struct {
	argv  []string
	stdin string
}

func testSubmitter(t *testing.T, srv *httptest.Server, calls *[]ran) *submitter {
	cfg := config.Default()
	cfg.Workspace, cfg.Handle = t.TempDir(), "tourist"
	dir := filepath.Join(cfg.Workspace, "1900", "A")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "main.cpp"), []byte("int main(){}\n"), 0o644)
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	client := cf.New(srv.URL)
	client.Interval = 0
	s := newSubmitter(cfg, client, st)
	s.interval, s.timeout = 5*time.Millisecond, 2*time.Second
	s.env = submit.Env{GOOS: "linux", Getenv: func(k string) string { return map[string]string{"WAYLAND_DISPLAY": "w"}[k] },
		LookPath: func(b string) (string, error) { return "/usr/bin/" + b, nil }}
	s.run = func(argv []string, stdin []byte) error {
		*calls = append(*calls, ran{argv, string(stdin)})
		return nil
	}
	return s
}

func TestSubmitFlowAccepted(t *testing.T) {
	var calls []ran
	s := testSubmitter(t, fakeCF(t, "OK"), &calls)
	st, err := s.begin(context.Background(), cf.Problem{ContestID: 1900, Index: "A"}, "cpp")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0].argv[0] != "wl-copy" || calls[0].stdin != "int main(){}\n" || calls[1].argv[0] != "xdg-open" || calls[1].argv[1] != "https://codeforces.com/contest/1900/submit/A" {
		t.Fatalf("copy then open expected: %+v", calls)
	}
	if st.Text != "int main(){}\n" || !strings.Contains(strings.Join(st.Notes, "|"), "paste and submit at https://codeforces.com/contest/1900/submit/A") {
		t.Fatalf("%+v", st)
	}
	var out bytes.Buffer
	if err := printSubmit(&out, st); err != nil {
		t.Fatalf("Accepted must exit 0: %v", err)
	}
	for _, want := range []string{"Testing on test 3", "Accepted", "15 ms  3.0 MB"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	// the Submission was upserted: the Problem now counts as solved
	if m, _ := s.store.Statuses(); m["1900A"] != store.StatusSolved {
		t.Fatalf("submission not stored: %v", m)
	}
}

func TestSubmitFlowWrongAnswerExitsNonZero(t *testing.T) {
	var calls []ran
	s := testSubmitter(t, fakeCF(t, "WRONG_ANSWER"), &calls)
	st, err := s.begin(context.Background(), cf.Problem{ContestID: 1900, Index: "A"}, "cpp")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printSubmit(&out, st); exitCode(err) != 1 || !strings.Contains(out.String(), "Wrong answer on test 6") {
		t.Fatalf("WA: code %d\n%s", exitCode(err), out.String())
	}
}

func TestSubmitOfflineFailsBeforeCopyOrOpen(t *testing.T) {
	var calls []ran
	srv := fakeCF(t, "OK")
	s := testSubmitter(t, srv, &calls)
	srv.Close()
	_, err := s.begin(context.Background(), cf.Problem{ContestID: 1900, Index: "A"}, "cpp")
	if err == nil || !strings.Contains(err.Error(), "offline") || len(calls) != 0 {
		t.Fatalf("offline must fail up front with nothing copied or opened: err=%v calls=%v", err, calls)
	}
	// missing Solution
	s = testSubmitter(t, fakeCF(t, "OK"), &calls)
	if _, err := s.begin(context.Background(), cf.Problem{ContestID: 1, Index: "Z"}, "cpp"); err == nil || !strings.Contains(err.Error(), "no Solution") {
		t.Fatalf("%v", err)
	}
}

func TestToolFailuresAreNotesNotErrors(t *testing.T) {
	var calls []ran
	s := testSubmitter(t, fakeCF(t, "OK"), &calls)
	s.run = func(argv []string, stdin []byte) error { return fmt.Errorf("%s: exec failed", argv[0]) }
	st, err := s.begin(context.Background(), cf.Problem{ContestID: 1900, Index: "A"}, "cpp")
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(st.Notes, "|")
	if !strings.Contains(notes, "clipboard tool failed") || !strings.Contains(notes, "could not open the browser") || !strings.Contains(notes, "paste and submit at") {
		t.Fatalf("notes should explain and give the URL: %v", st.Notes)
	}
	for range st.Updates {
	}
}

func TestDelegatedSubmitMatchesAndAttaches(t *testing.T) {
	var calls []ran
	s := testSubmitter(t, fakeCF(t, "OK"), &calls)
	sock := shortSock(t)
	srv, err := ipc.Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	attached := make(chan tui.ExternalSubmit, 1)
	go srv.Serve(dispatch(map[string]ipc.Handler{"submit": submitHandler(s, func(e tui.ExternalSubmit) { attached <- e })}))

	var out bytes.Buffer
	handled, err := delegateSubmit(context.Background(), &out, sock, filepath.Join(s.cfg.Workspace, "1900", "A", "main.cpp"))
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v\n%s", handled, err, out.String())
	}
	for _, want := range []string{"paste and submit at", "Testing on test 3", "Accepted"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("delegated output missing %q:\n%s", want, out.String())
		}
	}
	e := <-attached
	if e.Problem.ContestID != 1900 || e.Start.Text != "int main(){}\n" {
		t.Fatalf("TUI should receive the submit: %+v", e)
	}
	var texts []string
	for u := range e.Start.Updates {
		texts = append(texts, u.Text)
	}
	if len(texts) == 0 || texts[len(texts)-1] != "Accepted" {
		t.Fatalf("TUI updates: %v", texts)
	}
	// unknown commands are rejected cleanly
	if _, err := delegateTest(context.Background(), &out, sock, "x"); exitCode(err) != 2 {
		t.Fatalf("unknown cmd on this server: %v", err)
	}
}
