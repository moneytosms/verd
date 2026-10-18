package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/session"
	"github.com/moneytosms/verd/internal/store"
)

func sessSub(idx, verdict string, at int64) cf.Submission {
	x := cf.Submission{Verdict: verdict, Created: at}
	x.Problem.ContestID, x.Problem.Index = 7, idx
	return x
}

func TestVirtualSessionHidesAndScores(t *testing.T) {
	now := time.Unix(10_000, 0)
	ps := []cf.Problem{
		{ContestID: 7, Index: "A", Name: "Easy", Rating: 800, Tags: []string{"math"}, SolvedCount: 99999},
		{ContestID: 7, Index: "B", Name: "Hard", Rating: 2400, Tags: []string{"dp"}, SolvedCount: 12},
		{ContestID: 9, Index: "A", Name: "Other", Rating: 1500, Tags: []string{"greedy"}, SolvedCount: 5},
	}
	cs := []cf.Contest{{ID: 7, Name: "Round 7", Phase: "FINISHED", Start: 1, Duration: 7200}}
	var started session.Session
	var ended bool
	deps := Deps{
		Now: func() time.Time { return now },
		StartSession: func(c cf.Contest, mode string) (session.Session, error) {
			started = session.Session{ContestID: c.ID, Name: c.Name, Mode: mode, Start: now.Unix(), End: now.Unix() + c.Duration}
			return started, nil
		},
		EndSession: func() error { ended = true; return nil },
	}
	// the user solved 7A years ago: a replay must not show it
	data := Data{Problems: ps, Contests: cs, Statuses: map[string]store.Status{"7A": store.StatusSolved, "9A": store.StatusSolved}}
	m := New(ps, "", deps).WithData(data)
	m, _ = send(m, "2")
	m, _ = send(m, "enter")
	if !strings.Contains(plain(m), "2400") {
		t.Fatalf("ratings are visible before the session:\n%s", plain(m))
	}
	m, cmd := send(m, "s")
	if m.sess != nil || cmd != nil || !strings.Contains(plain(m), "Press the same key again") {
		t.Fatalf("first press only asks to confirm:\n%s", plain(m))
	}
	m, cmd = send(m, "s")
	if m.sess == nil || cmd == nil || started.Mode != session.Virtual {
		t.Fatalf("second press starts the session: %+v", m.sess)
	}
	out := plain(m)
	for _, hidden := range []string{"2400", "800"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("rating %s must be hidden:\n%s", hidden, out)
		}
	}
	for _, want := range []string{"virtual", "2:00:00 left", "solved 0/2", "penalty 0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("session view missing %q:\n%s", want, out)
		}
	}
	if m.statusOf(ps[0]) != store.StatusNone || m.statusOf(ps[2]) != store.StatusSolved {
		t.Fatalf("old marks hidden only inside the session contest: %v %v", m.statusOf(ps[0]), m.statusOf(ps[2]))
	}
	if blinded := m.Problems[0]; blinded.Rating != 0 || blinded.Tags != nil || blinded.SolvedCount != 0 || m.Problems[2].Rating != 1500 {
		t.Fatalf("blinding: %+v / %+v", blinded, m.Problems[2])
	}
	if raw := m.raw.Problems[0]; raw.Rating != 800 {
		t.Fatal("raw data must stay intact so ending restores it")
	}

	// a submission inside the window scores; a later reload shows it
	d := m.raw
	d.Subs = []cf.Submission{sessSub("A", "WRONG_ANSWER", now.Unix()+60), sessSub("A", "OK", now.Unix()+600), sessSub("B", "OK", now.Unix()-5)}
	m = m.WithData(d)
	out = plain(m)
	if !strings.Contains(out, "solved 1/2") || !strings.Contains(out, "penalty 30") || !strings.Contains(out, "10m +1") || m.statusOf(ps[1]) != store.StatusNone {
		t.Fatalf("score after a solve (10 min + 20 for one wrong):\n%s", out)
	}

	// time runs out: the summary opens, esc keeps the session, e ends it
	now = now.Add(2*time.Hour + time.Second)
	m2, _ := m.Update(sessTickMsg{})
	m = m2.(Model)
	if !m.sessSum || !strings.Contains(plain(m), "Session summary") || !strings.Contains(plain(m), "upsolve: B") {
		t.Fatalf("summary at the deadline:\n%s", plain(m))
	}
	m, _ = send(m, "e")
	if !ended || m.sess != nil || m.sessSum {
		t.Fatal("e must end the session")
	}
	if m.Problems[1].Rating != 2400 || m.statusOf(ps[0]) != store.StatusSolved {
		t.Fatal("ending restores ratings and old marks")
	}
}

func TestSessionRefusals(t *testing.T) {
	cs := []cf.Contest{{ID: 7, Name: "Live", Phase: "CODING", Start: 1}, {ID: 8, Name: "Done", Phase: "FINISHED", Start: 1, Duration: 60}}
	ps := []cf.Problem{{ContestID: 8, Index: "A", Name: "x"}}
	deps := Deps{
		StartSession: func(c cf.Contest, mode string) (session.Session, error) {
			return session.Session{ContestID: c.ID, Mode: mode, Start: 1, End: 61}, nil
		},
	}
	m := New(ps, "", deps).WithData(Data{Problems: ps, Contests: cs})
	m, _ = send(m, "2")
	m, _ = send(m, "enter") // the running contest sorts first
	m, _ = send(m, "s")
	if m.sess != nil || !strings.Contains(plain(m), "finished contests") {
		t.Fatalf("live contests are not sessions yet:\n%s", plain(m))
	}
	m, _ = send(m, "esc")
	m, _ = send(m, "j")
	m, _ = send(m, "enter")
	m, _ = send(m, "p")
	m, _ = send(m, "p")
	if m.sess == nil || m.sess.Mode != session.Practice {
		t.Fatalf("practice starts: %+v", m.sess)
	}
	m, _ = send(m, "esc")
	m, _ = send(m, "k")
	m, _ = send(m, "enter")
	m, _ = send(m, "s")
	if !strings.Contains(plain(m), "end the running session first") {
		t.Fatalf("one session at a time:\n%s", plain(m))
	}
}
