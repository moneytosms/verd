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
