package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/tui"
)

func TestDaily(t *testing.T) {
	var d tui.Data
	if err := dailyCmd(&bytes.Buffer{}, d, time.Now()); err == nil {
		t.Fatal("no Problems should error")
	}
	for i := 0; i < 20; i++ {
		d.Problems = append(d.Problems, cf.Problem{ContestID: 1000 + i, Index: "A", Name: "P", Rating: 800 + i*20, Tags: []string{"math"}})
	}
	d.Statuses = map[string]store.Status{"1000A": store.StatusSolved}
	day := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	var a, b bytes.Buffer
	if err := dailyCmd(&a, d, day); err != nil {
		t.Fatal(err)
	}
	dailyCmd(&b, d, day.Add(10*time.Hour))
	if a.String() != b.String() || !strings.Contains(a.String(), "codeforces.com") || strings.HasPrefix(a.String(), "1000A") {
		t.Fatalf("same day must give the same unsolved pick:\n%s\n%s", a.String(), b.String())
	}
}
