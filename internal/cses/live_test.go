//go:build live

package cses

import (
	"context"
	"testing"
)

// go test -tags live ./internal/cses: checks the parsers against the real site.
func TestLive(t *testing.T) {
	c := New()
	ps, err := c.Problems(context.Background())
	if err != nil || len(ps) < 300 {
		t.Fatalf("%d problems: %v", len(ps), err)
	}
	bad := 0
	for _, p := range ps[:12] {
		d, err := c.Page(context.Background(), p.ContestID)
		if err != nil || d.TimeLimitMS == 0 || len(d.Samples) == 0 {
			t.Errorf("%s: %v %+v", p.Name, err, d)
			bad++
		}
	}
	t.Logf("%d problems, first %+v, %d bad of 12", len(ps), ps[0], bad)
}
