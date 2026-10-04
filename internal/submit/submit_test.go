package submit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

func env(goos string, vars map[string]string, bins ...string) Env {
	have := map[string]bool{}
	for _, b := range bins {
		have[b] = true
	}
	return Env{
		GOOS:   goos,
		Getenv: func(k string) string { return vars[k] },
		LookPath: func(b string) (string, error) {
			if have[b] {
				return "/usr/bin/" + b, nil
			}
			return "", errors.New("not found")
		},
	}
}

func TestClipboardAndOpenerSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  Env
		clip string
		open string
	}{
		{"macOS", env("darwin", nil, "pbcopy"), "pbcopy", "open"},
		{"wayland", env("linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, "wl-copy", "xclip", "xdg-open"), "wl-copy", "xdg-open"},
		{"x11 xclip", env("linux", map[string]string{"DISPLAY": ":0"}, "xclip", "xdg-open"), "xclip -selection clipboard", "xdg-open"},
		{"x11 xsel only", env("linux", map[string]string{"DISPLAY": ":0"}, "xsel", "xdg-open"), "xsel --clipboard --input", "xdg-open"},
		{"WSL with wslview", env("linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu", "DISPLAY": ":0"}, "clip.exe", "wslview", "explorer.exe", "xdg-open"), "clip.exe", "wslview"},
		{"WSL explorer fallback", env("linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, "clip.exe", "explorer.exe"), "clip.exe", "explorer.exe"},
		{"headless ssh", env("linux", nil), "", ""},
		{"wayland var but no tool", env("linux", map[string]string{"WAYLAND_DISPLAY": "w"}, "xdg-open"), "", "xdg-open"},
	} {
		got := tc.env.ClipboardCommand()
		gotStr := ""
		if len(got) > 0 {
			gotStr = got[0]
			for _, a := range got[1:] {
				gotStr += " " + a
			}
		}
		if gotStr != tc.clip {
			t.Errorf("%s clipboard: got %q want %q", tc.name, gotStr, tc.clip)
		}
		open := tc.env.OpenCommand("https://x")
		if tc.open == "" && open != nil || tc.open != "" && (len(open) != 2 || open[0] != tc.open || open[1] != "https://x") {
			t.Errorf("%s opener: got %v want %q", tc.name, open, tc.open)
		}
	}
}

func TestURLAndDescribe(t *testing.T) {
	if URL(1900, "A") != "https://codeforces.com/contest/1900/submit/A" {
		t.Fatal(URL(1900, "A"))
	}
	for _, tc := range []struct {
		s    cf.Submission
		want string
	}{
		{cf.Submission{Verdict: "TESTING", PassedTests: 2}, "Testing on test 3"},
		{cf.Submission{Verdict: ""}, "Testing on test 1"},
		{cf.Submission{Verdict: "OK", PassedTests: 9}, "Accepted"},
		{cf.Submission{Verdict: "WRONG_ANSWER", PassedTests: 1}, "Wrong answer on test 2"},
		{cf.Submission{Verdict: "TIME_LIMIT_EXCEEDED", PassedTests: 6}, "Time limit exceeded on test 7"},
		{cf.Submission{Verdict: "COMPILATION_ERROR"}, "Compilation error"},
		{cf.Submission{Verdict: "SOMETHING_NEW"}, "something new"},
	} {
		if got := Describe(tc.s); got != tc.want {
			t.Errorf("%+v: got %q want %q", tc.s, got, tc.want)
		}
	}
}

// fakeStatus serves a scripted sequence of user.status responses, newest first.
type fakeStatus struct {
	mu    sync.Mutex
	steps [][]cf.Submission
	calls int
	err   error
}

func (f *fakeStatus) fetch(context.Context, int) ([]cf.Submission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	i := min(f.calls, len(f.steps)-1)
	f.calls++
	return f.steps[i], nil
}

func sub(id int64, contest int, index, verdict string, passed int) cf.Submission {
	s := cf.Submission{ID: id, Verdict: verdict, PassedTests: passed}
	s.Problem.ContestID, s.Problem.Index = contest, index
	return s
}

func fast(f *fakeStatus) Tracker {
	return Tracker{Fetch: f.fetch, Interval: 5 * time.Millisecond, Timeout: 2 * time.Second}
}

func drain(ch <-chan Update) []Update {
	var out []Update
	for u := range ch {
		out = append(out, u)
	}
	return out
}

func TestTrackTestingToOK(t *testing.T) {
	old := sub(100, 1900, "A", "WRONG_ANSWER", 1) // an earlier attempt
	f := &fakeStatus{steps: [][]cf.Submission{
		{old}, // baseline
		{old}, // submitted nothing yet
		{sub(101, 1900, "A", "TESTING", 0), old},
		{sub(101, 1900, "A", "TESTING", 0), old}, // unchanged: no update
		{sub(101, 1900, "A", "TESTING", 3), old},
		{sub(101, 1900, "A", "OK", 10), old},
	}}
	tr := fast(f)
	base, err := tr.Baseline(context.Background(), 1900, "A")
	if err != nil || base != 100 {
		t.Fatalf("baseline %d %v", base, err)
	}
	ups := drain(tr.Track(context.Background(), 1900, "A", base))
	var texts []string
	for _, u := range ups {
		texts = append(texts, u.Text)
	}
	want := []string{"Testing on test 1", "Testing on test 4", "Accepted"}
	if len(texts) != 3 || texts[0] != want[0] || texts[1] != want[1] || texts[2] != want[2] {
		t.Fatalf("got %v want %v", texts, want)
	}
	if !ups[2].Final || ups[2].Submission.ID != 101 || ups[0].Final {
		t.Fatalf("finality wrong: %+v", ups)
	}
}

func TestTrackIgnoresOldAndOtherProblems(t *testing.T) {
	old := sub(100, 1900, "A", "OK", 10)
	other := sub(105, 1900, "B", "OK", 10) // newer id, but another Problem
	f := &fakeStatus{steps: [][]cf.Submission{{other, old}}}
	tr := Tracker{Fetch: f.fetch, Interval: 5 * time.Millisecond, Timeout: 100 * time.Millisecond}
	ups := drain(tr.Track(context.Background(), 1900, "A", 100))
	if len(ups) != 1 || !ups[0].Final || !errors.Is(ups[0].Err, ErrTimeout) {
		t.Fatalf("old/other-problem submissions must be ignored until timeout: %+v", ups)
	}
}

func TestTrackSurvivesPollErrorsAndStopsOnCancel(t *testing.T) {
	f := &fakeStatus{err: errors.New("network down"), steps: [][]cf.Submission{{}}}
	tr := fast(f)
	ctx, cancel := context.WithCancel(context.Background())
	ch := tr.Track(ctx, 1, "A", 0)
	u := <-ch
	if u.Err == nil || u.Final {
		t.Fatalf("poll error should be reported, non-final: %+v", u)
	}
	f.mu.Lock()
	f.err = nil
	f.steps = [][]cf.Submission{{sub(5, 1, "A", "OK", 3)}}
	f.mu.Unlock()
	var final Update
	for u := range ch {
		if u.Final {
			final = u
		}
	}
	if final.Text != "Accepted" {
		t.Fatalf("tracking should recover after errors: %+v", final)
	}
	cancel()
	// cancelled before any result: channel closes without a final update
	ch = tr.Track(ctx, 1, "A", 0)
	if ups := drain(ch); len(ups) != 0 {
		t.Fatalf("cancelled tracker must emit nothing: %+v", ups)
	}
}

func TestBaselineOfflineErrors(t *testing.T) {
	f := &fakeStatus{err: cf.ErrNetwork}
	if _, err := fast(f).Baseline(context.Background(), 1, "A"); !errors.Is(err, cf.ErrNetwork) {
		t.Fatalf("offline must surface: %v", err)
	}
}

func TestTrackDefaults(t *testing.T) {
	d := Tracker{}.defaults()
	if d.Interval != 2*time.Second || d.Timeout != 5*time.Minute {
		t.Fatalf("%+v", d)
	}
}
