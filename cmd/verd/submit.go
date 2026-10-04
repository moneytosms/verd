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
	"runtime"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
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
}

func newSubmitter(cfg config.Config, client *cf.Client, s *store.Store) *submitter {
	return &submitter{
		cfg: cfg, client: client, store: s,
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
	if clip := s.env.ClipboardCommand(); clip != nil {
		if err := s.run(clip, src); err != nil {
			st.Notes = append(st.Notes, "clipboard tool failed: "+err.Error())
		}
	}
	url := submit.URL(p.ContestID, p.Index)
	if open := s.env.OpenCommand(url); open != nil {
		if err := s.run(open, nil); err != nil {
			st.Notes = append(st.Notes, "could not open the browser: "+err.Error())
		}
	} else {
		st.Notes = append(st.Notes, "no browser opener found")
	}
	st.Notes = append(st.Notes, "paste and submit at "+url)

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
	return st, nil
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
