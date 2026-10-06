package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/creds"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/submit"
	"github.com/moneytosms/verd/internal/workspace"
)

// submitter holds what a submit flow needs; fields are injectable for tests.
type submitter struct {
	cfg      config.Config
	client   *cf.Client
	store    *store.Store
	env      submit.Env
	run      func(argv []string, stdin []byte) error // copy tool / opener
	interval time.Duration
	timeout  time.Duration
	// direct posts the Solution with the saved session; any error means "use the browser instead".
	direct func(ctx context.Context, contest int, index string, compilerID int, src []byte) error
}

func credStore() creds.Store {
	return creds.Store{File: filepath.Join(config.Dir(), "credentials.json")}
}

// directSubmit is the real direct path: saved session over a Chrome TLS client.
func directSubmit(ctx context.Context, contest int, index string, compilerID int, src []byte) error {
	c, err := credStore().Load()
	if err != nil {
		return err
	}
	doer, err := submit.NewChromeDoer()
	if err != nil {
		return err
	}
	return submit.Direct{Doer: doer, Cookie: c.Cookie, UserAgent: c.UserAgent}.Submit(ctx, contest, index, compilerID, src)
}

func newSubmitter(cfg config.Config, client *cf.Client, s *store.Store) *submitter {
	return &submitter{
		cfg: cfg, client: client, store: s, direct: directSubmit,
		env: submit.Env{GOOS: runtime.GOOS, Getenv: os.Getenv, LookPath: exec.LookPath},
		run: func(argv []string, stdin []byte) error {
			cmd := exec.Command(argv[0], argv[1:]...)
			if stdin != nil {
				cmd.Stdin = bytes.NewReader(stdin)
				return cmd.Run()
			}
			return cmd.Start() // openers (xdg-open etc.) may outlive us; do not wait
		},
	}
}

// Started is a submit in progress: the Solution text (for an OSC 52 copy by the caller),
// non-fatal notes (copy/open problems), and the live updates.
type Started struct {
	Problem cf.Problem
	Text    string
	Notes   []string
	Direct  bool // sent straight to Codeforces; nothing was copied or opened
	Updates <-chan submit.Update
}

// begin hands the Solution to the user's browser and starts tracking the Submission.
// Offline is detected first (before anything is copied or opened). Every update's Submission is
// upserted into the store.
func (s *submitter) begin(ctx context.Context, p cf.Problem, lang string) (*Started, error) {
	ref, err := solutionRef(s.cfg, p, lang)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(ref.Path)
	if err != nil {
		return nil, fmt.Errorf("no Solution at %s", ref.Path)
	}
	if p.Source() != cf.SourceCF {
		// No Verdict feed without a login: hand the Solution over, and the user marks it solved (m).
		st := &Started{Problem: p, Text: string(src)}
		s.handoff(st, src, submit.URL(p.ContestID, p.Index))
		st.Notes = append(st.Notes, "verd cannot see the verdict here: press m on the Problem to mark it solved")
		done := make(chan submit.Update)
		close(done)
		st.Updates = done
		return st, nil
	}
	tr := submit.Tracker{Interval: s.interval, Timeout: s.timeout, Fetch: func(ctx context.Context, count int) ([]cf.Submission, error) {
		return s.client.UserStatus(ctx, s.cfg.Handle, 1, count)
	}}
	base, err := tr.Baseline(ctx, p.ContestID, p.Index)
	if err != nil {
		if errors.Is(err, cf.ErrNetwork) {
			return nil, errors.New("offline: submitting needs a connection to Codeforces")
		}
		return nil, err
	}
	st := &Started{Problem: p, Text: string(src)}
	sent := false
	if s.cfg.SubmitMode == "direct" {
		if err := s.direct(ctx, p.ContestID, p.Index, s.cfg.Lang[lang].CFCompilerID, src); err != nil {
			st.Notes = append(st.Notes, "direct submit failed, using the browser: "+err.Error())
		} else {
			sent, st.Direct = true, true
			st.Notes = append(st.Notes, "submitted directly")
		}
	}
	if sent {
		return s.track(ctx, st, tr, base), nil
	}
	s.handoff(st, src, submit.URL(p.ContestID, p.Index))
	return s.track(ctx, st, tr, base), nil
}

// handoff copies the Solution and opens the submit page, noting what went wrong.
func (s *submitter) handoff(st *Started, src []byte, url string) {
	if clip := s.env.ClipboardCommand(); clip != nil {
		if err := s.run(clip, src); err != nil {
			st.Notes = append(st.Notes, "clipboard tool failed: "+err.Error())
		}
	}
	if open := s.env.OpenCommand(url); open != nil {
		if err := s.run(open, nil); err != nil {
			st.Notes = append(st.Notes, "could not open the browser: "+err.Error())
		}
	} else {
		st.Notes = append(st.Notes, "no browser opener found")
	}
	st.Notes = append(st.Notes, "paste and submit at "+url)
}

// track follows the new Submission, upserting each update into the store.
func (s *submitter) track(ctx context.Context, st *Started, tr submit.Tracker, base int64) *Started {
	p := st.Problem
	raw := tr.Track(ctx, p.ContestID, p.Index, base)
	out := make(chan submit.Update, 4)
	go func() {
		defer close(out)
		for u := range raw {
			if u.Submission.ID != 0 {
				s.store.SaveSubmissions([]cf.Submission{u.Submission})
			}
			select {
			case out <- u:
			case <-ctx.Done():
				return
			}
		}
	}()
	st.Updates = out
	return st
}

// osc52 is the clipboard escape sequence; it works over tmux/ssh where terminals allow it.
func osc52(text string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
}

// printSubmit runs a submit headless: notes, then each status until the final one.
// Exit 0 only if the Submission was Accepted.
func printSubmit(out io.Writer, st *Started) error {
	for _, n := range st.Notes {
		fmt.Fprintln(out, n)
	}
	var final submit.Update
	for u := range st.Updates {
		fmt.Fprintln(out, u.Text)
		if u.Final {
			final = u
		}
	}
	switch {
	case final.Err != nil:
		return &exitError{1, ""}
	case final.Submission.Verdict == "OK":
		fmt.Fprintf(out, "%d ms  %.1f MB\n", final.Submission.TimeMS, float64(final.Submission.MemoryBytes)/(1<<20))
		return nil
	case !final.Final:
		return &exitError{2, "tracking ended without a Verdict"}
	}
	return &exitError{1, ""}
}

func absSolutionProblem(cfg config.Config, path string) (cf.Problem, string, error) {
	ref, err := workspace.New(cfg.Workspace).Resolve(path, cfg.Lang)
	if err != nil {
		return cf.Problem{}, "", err
	}
	return cf.Problem{ContestID: ref.Contest, Index: ref.Index}, ref.Lang, nil
}
