// Package submit hands a Solution to the user's browser and tracks the resulting Submission's Verdict.
package submit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

// URL is the Problem's submit page.
func URL(contest int, index string) string {
	return fmt.Sprintf("%s/contest/%d/submit/%s", cf.BaseURL, contest, index)
}

// Env is what platform detection looks at; fields are injected for tests.
type Env struct {
	GOOS     string
	Getenv   func(string) string
	LookPath func(string) (string, error)
}

func (e Env) has(bin string) bool { _, err := e.LookPath(bin); return err == nil }

func (e Env) wsl() bool { return e.Getenv("WSL_DISTRO_NAME") != "" }

// ClipboardCommand picks the tool that receives the Solution on stdin; nil if none is available
// (OSC 52 is still sent by the caller).
func (e Env) ClipboardCommand() []string {
	switch {
	case e.GOOS == "darwin" && e.has("pbcopy"):
		return []string{"pbcopy"}
	case e.wsl() && e.has("clip.exe"):
		return []string{"clip.exe"}
	case e.Getenv("WAYLAND_DISPLAY") != "" && e.has("wl-copy"):
		return []string{"wl-copy"}
	case e.Getenv("DISPLAY") != "" && e.has("xclip"):
		return []string{"xclip", "-selection", "clipboard"}
	case e.Getenv("DISPLAY") != "" && e.has("xsel"):
		return []string{"xsel", "--clipboard", "--input"}
	}
	return nil
}

// OpenCommand picks the browser opener for url; nil if none is available.
func (e Env) OpenCommand(url string) []string {
	switch {
	case e.GOOS == "darwin":
		return []string{"open", url}
	case e.wsl() && e.has("wslview"):
		return []string{"wslview", url}
	case e.wsl() && e.has("explorer.exe"):
		return []string{"explorer.exe", url}
	case e.has("xdg-open"):
		return []string{"xdg-open", url}
	}
	return nil
}

// Update is one observation of the Submission being tracked.
type Update struct {
	Submission cf.Submission
	Final      bool  // verdict decided
	Err        error // a poll failed (tracking continues) or, with Final, the timeout
	Text       string
}

// ErrTimeout ends tracking when no final Verdict arrived in time.
var ErrTimeout = errors.New("timed out waiting for a Verdict")

// Describe renders a Submission's state for humans: "Testing on test 3", "Accepted", "Wrong answer on test 2".
func Describe(s cf.Submission) string {
	test := s.PassedTests + 1
	switch s.Verdict {
	case "", "TESTING":
		return fmt.Sprintf("Testing on test %d", test)
	case "OK":
		return "Accepted"
	case "WRONG_ANSWER":
		return fmt.Sprintf("Wrong answer on test %d", test)
	case "TIME_LIMIT_EXCEEDED":
		return fmt.Sprintf("Time limit exceeded on test %d", test)
	case "MEMORY_LIMIT_EXCEEDED":
		return fmt.Sprintf("Memory limit exceeded on test %d", test)
	case "RUNTIME_ERROR":
		return fmt.Sprintf("Runtime error on test %d", test)
	case "COMPILATION_ERROR":
		return "Compilation error"
	case "IDLENESS_LIMIT_EXCEEDED":
		return fmt.Sprintf("Idleness limit exceeded on test %d", test)
	case "PRESENTATION_ERROR":
		return fmt.Sprintf("Presentation error on test %d", test)
	case "SKIPPED":
		return "Skipped"
	case "REJECTED":
		return "Rejected"
	}
	return strings.ToLower(strings.ReplaceAll(s.Verdict, "_", " "))
}

// Fetcher returns the user's newest Submissions, newest first (user.status from=1).
type Fetcher func(ctx context.Context, count int) ([]cf.Submission, error)

// Tracker follows the Submission a user is about to make for one Problem.
type Tracker struct {
	Fetch    Fetcher
	Interval time.Duration // default 2s
	Timeout  time.Duration // default 5m
}

func (t Tracker) defaults() Tracker {
	if t.Interval == 0 {
		t.Interval = 2 * time.Second
	}
	if t.Timeout == 0 {
		t.Timeout = 5 * time.Minute
	}
	return t
}

func newest(subs []cf.Submission, contest int, index string) (cf.Submission, bool) {
	var best cf.Submission
	found := false
	for _, s := range subs {
		if s.Problem.ContestID == contest && s.Problem.Index == index && (!found || s.ID > best.ID) {
			best, found = s, true
		}
	}
	return best, found
}

// Baseline records the newest existing Submission id for the Problem. Call it BEFORE the user
// submits: only Submissions with a higher id count as theirs, which needs no clock agreement
// with Codeforces. A fetch error (offline) is returned so the caller can say so up front.
func (t Tracker) Baseline(ctx context.Context, contest int, index string) (int64, error) {
	subs, err := t.defaults().Fetch(ctx, 20)
	if err != nil {
		return 0, err
	}
	b, _ := newest(subs, contest, index)
	return b.ID, nil
}

// Track polls until the new Submission has a final Verdict, the timeout passes, or ctx ends.
// The channel closes afterwards.
func (t Tracker) Track(ctx context.Context, contest int, index string, baseline int64) <-chan Update {
	t = t.defaults()
	out := make(chan Update, 4)
	go func() {
		defer close(out)
		deadline := time.NewTimer(t.Timeout)
		defer deadline.Stop()
		tick := time.NewTicker(t.Interval)
		defer tick.Stop()
		send := func(u Update) bool {
			select {
			case out <- u:
				return true
			case <-ctx.Done():
				return false
			}
		}
		var last cf.Submission
		for {
			select {
			case <-ctx.Done():
				return
			case <-deadline.C:
				send(Update{Final: true, Err: ErrTimeout, Text: ErrTimeout.Error()})
				return
			case <-tick.C:
			}
			subs, err := t.Fetch(ctx, 20)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case out <- Update{Err: err, Text: "poll failed: " + err.Error()}:
				default: // the reader is behind; the next poll reports again
				}
				continue
			}
			s, ok := newest(subs, contest, index)
			if !ok || s.ID <= baseline {
				continue // nothing new yet
			}
			final := s.Verdict != "" && s.Verdict != "TESTING"
			if final || s.ID != last.ID || s.PassedTests != last.PassedTests || s.Verdict != last.Verdict {
				last = s
				if !send(Update{Submission: s, Final: final, Text: Describe(s)}) {
					return
				}
			}
			if final {
				return
			}
		}
	}()
	return out
}
