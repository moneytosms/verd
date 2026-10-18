package session

import (
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

func sub(contest int, idx, verdict string, at int64) cf.Submission {
	x := cf.Submission{Verdict: verdict, Created: at}
	x.Problem.ContestID, x.Problem.Index = contest, idx
	return x
}

func TestScoreICPC(t *testing.T) {
	s := Session{ContestID: 7, Mode: Virtual, Start: 1000, End: 1000 + 7200}
	subs := []cf.Submission{
		sub(7, "A", "OK", 400),                    // before the session: never counts
		sub(7, "A", "WRONG_ANSWER", 1000+60),      // wrong 1
		sub(7, "A", "COMPILATION_ERROR", 1000+90), // free
		sub(7, "A", "OK", 1000+600),               // solved at minute 10
		sub(7, "A", "WRONG_ANSWER", 1000+900),     // after the solve: ignored
		sub(7, "B", "WRONG_ANSWER", 1000+100),     // never solved: no penalty
		sub(7, "B", "TIME_LIMIT_EXCEEDED", 1000+200),
		sub(7, "C", "OK", 1000+7200+1), // after the deadline
		sub(8, "A", "OK", 1000+30),     // other contest
		sub(7, "D", "OK", 1000+119),    // minute 1 (floor)
	}
	r := Score(s, []string{"A", "B", "C", "D"}, subs)
	if r.Solved != 2 || r.Penalty != 10+PenaltyMinutes+1 {
		t.Fatalf("%+v", r)
	}
	if a := r.Rows[0]; !a.Solved || a.Minute != 10 || a.Wrong != 1 {
		t.Fatalf("A: %+v", a)
	}
	if b := r.Rows[1]; b.Solved || b.Wrong != 2 {
		t.Fatalf("B: %+v", b)
	}
	if c := r.Rows[2]; c.Solved {
		t.Fatalf("C solved after the deadline: %+v", c)
	}
}

func TestPracticeHasNoPenaltyOrDeadline(t *testing.T) {
	s := Session{ContestID: 7, Mode: Practice, Start: 1000}
	r := Score(s, []string{"A"}, []cf.Submission{sub(7, "A", "WRONG_ANSWER", 1100), sub(7, "A", "OK", 999999)})
	if r.Solved != 1 || r.Penalty != 0 {
		t.Fatalf("%+v", r)
	}
	if s.Over(time.Unix(1<<40, 0)) || s.Left(time.Now()) != 0 {
		t.Fatal("practice never ends")
	}
}

func TestOverAndLeft(t *testing.T) {
	s := Session{Mode: Virtual, Start: 100, End: 200}
	if s.Over(time.Unix(199, 0)) || !s.Over(time.Unix(200, 0)) {
		t.Fatal("deadline boundary")
	}
	if s.Left(time.Unix(150, 0)) != 50*time.Second || s.Left(time.Unix(300, 0)) != 0 {
		t.Fatal("left")
	}
}
