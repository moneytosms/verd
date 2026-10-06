package main

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/tui"
)

// dailyCmd prints today's pick: an unsolved Codeforces Problem near your rating, from your weak
// topics when there are any. Seeded by the date, so it is the same all day.
func dailyCmd(out io.Writer, d tui.Data, now time.Time) error {
	if len(d.Problems) == 0 {
		return &exitError{1, "no Problems cached: start verd once to sync"}
	}
	status := func(p cf.Problem) store.Status { return d.Statuses[fmt.Sprintf("%d%s", p.ContestID, p.Index)] }
	rng := rand.New(rand.NewSource(int64(now.Year()*10000 + int(now.Month())*100 + now.Day())))
	f := tui.PickFilter(d.Stats.Rating)
	weak := false
	for _, t := range d.Stats.Weaknesses {
		f.AnyOf = append(f.AnyOf, t.Tag)
		weak = true
	}
	p, _, ok := tui.Pick(d.Problems, status, f, rng, nil)
	if !ok && weak { // nothing in the weak topics: any topic
		f.AnyOf, weak = nil, false
		p, _, ok = tui.Pick(d.Problems, status, f, rng, nil)
	}
	if !ok {
		return &exitError{1, "no unsolved Problem in your rating band"}
	}
	fmt.Fprintf(out, "%s  %s  (%d)\n", p.Code(), p.Name, p.Rating)
	if len(p.Tags) > 0 {
		fmt.Fprintln(out, "tags:", strings.Join(p.Tags, ", "))
	}
	if weak {
		fmt.Fprintln(out, "from your weak topics")
	}
	fmt.Fprintln(out, p.URL())
	return nil
}
