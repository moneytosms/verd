package store

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

func TestProblemsetRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "verd.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if at, _ := s.ProblemsetSyncedAt(); !at.IsZero() {
		t.Fatalf("fresh db should be unsynced, got %v", at)
	}
	want := []cf.Problem{
		{ContestID: 2, Index: "A", Name: "B", Rating: 800, Tags: []string{"dp", "math"}, SolvedCount: 5},
		{ContestID: 1, Index: "A", Name: "A", Tags: []string{}},
	}
	at := time.Unix(1700000000, 0)
	if err := s.SaveProblemset(want, at); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// reopen: persisted, migration idempotent
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Problems()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v err %v", got, err)
	}
	if g, _ := s.ProblemsetSyncedAt(); !g.Equal(at) {
		t.Fatalf("synced at %v", g)
	}
	// replace semantics
	if err := s.SaveProblemset(want[:1], at); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Problems(); len(got) != 1 {
		t.Fatalf("expected replace, got %d", len(got))
	}
}

func TestProblemStatePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verd.db")
	s, _ := Open(path)
	if st, err := s.ProblemState(1, "A"); err != nil || st != (ProblemState{}) {
		t.Fatalf("unset state must be zero: %+v %v", st, err)
	}
	if err := s.SaveProblemState(1, "A", ProblemState{Lang: "python", Mode: "float"}, time.Unix(5, 0)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProblemState(1, "A", ProblemState{Lang: "python", Mode: "exact"}, time.Unix(6, 0)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, _ = Open(path)
	defer s.Close()
	if st, _ := s.ProblemState(1, "A"); st != (ProblemState{Lang: "python", Mode: "exact"}) {
		t.Fatalf("after reopen: %+v", st)
	}
	if st, _ := s.ProblemState(1, "B"); st != (ProblemState{}) {
		t.Fatal("state is per Problem")
	}
}

func TestAllSubmissionsRoundTrip(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	a := cf.Submission{ID: 2, Created: 20, Language: "C++", Verdict: "OK", PassedTests: 5, TimeMS: 15, MemoryBytes: 1 << 20}
	a.Problem.ContestID, a.Problem.Index = 1900, "A"
	b := a
	b.ID, b.Verdict = 1, "WRONG_ANSWER"
	if err := s.SaveSubmissions([]cf.Submission{a, b}); err != nil {
		t.Fatal(err)
	}
	got, err := s.AllSubmissions()
	if err != nil || len(got) != 2 || got[0].ID != 1 || !reflect.DeepEqual(got[1], a) {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestOtherSourcesSurviveCFSyncAndMarksShowAsSolved(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Unix(1000, 0)
	if err := s.SaveSource(cf.SourceCSES, []cf.Problem{{ContestID: 1068, Name: "Weird Algorithm", Tags: []string{"Introductory Problems"}}}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProblemset([]cf.Problem{{ContestID: 4, Index: "A", Name: "Watermelon", Tags: []string{}}}, now); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.Problems()
	if len(ps) != 2 {
		t.Fatalf("a Codeforces sync must keep CSES rows: %+v", ps)
	}
	if err := s.SaveSource(cf.SourceCSES, nil, now); err != nil {
		t.Fatal(err)
	}
	if ps, _ = s.Problems(); len(ps) != 1 || ps[0].Index != "A" {
		t.Fatalf("a CSES sync must keep Codeforces rows: %+v", ps)
	}
	s.SetMark(1068, cf.SourceCSES, true)
	if st, _ := s.Statuses(); st["1068cses"] != StatusSolved {
		t.Fatalf("mark: %v", st)
	}
	s.SetMark(1068, cf.SourceCSES, false)
	if st, _ := s.Statuses(); len(st) != 0 {
		t.Fatalf("cleared: %v", st)
	}
}
