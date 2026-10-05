package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/stress"
)

func TestStressPanelOverlaySaveAndCancel(t *testing.T) {
	ch := make(chan stress.Event, 8)
	var saved [2]string
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p>hi</p></div>`, TimeLimitMS: 1000}, nil
		},
		Stress: func(context.Context, cf.Problem, *scrape.Detail, RunOpts) (<-chan stress.Event, error) {
			return ch, nil
		},
		SaveCounterexample: func(p cf.Problem, in, want string) (string, error) {
			saved = [2]string{in, want}
			return "custom-3", nil
		},
	}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A", Name: "N"}}, "", deps), "enter")
	m, _ = step(t, m, cmd)

	// progress updates per event
	ch <- stress.Event{Iter: 7, Seed: 7}
	m, cmd = send(m, "S")
	m, cmd = step(t, m, cmd)
	if out := plain(m); !strings.Contains(out, "running...") || !strings.Contains(out, "iteration 7") {
		t.Fatalf("progress missing:\n%s", out)
	}

	// esc cancels the run, not the Problem
	m, _ = send(m, "esc")
	if m.open == nil || m.strs.running || !strings.Contains(plain(m), "cancelled") {
		t.Fatalf("esc should cancel stress only:\n%s", plain(m))
	}

	// a mismatch opens the diff overlay; w saves brute output as the answer
	ch2 := make(chan stress.Event, 2)
	deps.Stress = func(context.Context, cf.Problem, *scrape.Detail, RunOpts) (<-chan stress.Event, error) {
		return ch2, nil
	}
	m.deps = deps
	m, cmd = send(m, "S")
	ch2 <- stress.Event{Done: true, Result: stress.Result{Kind: stress.Mismatch, Seed: 9, Iterations: 9, Input: "3\n", Want: "6\n", Got: "7\n",
		Mismatch: &runner.Mismatch{Line: 1, Col: 1, Want: "6", Got: "7"}}}
	m, _ = step(t, m, cmd)
	out := plain(m)
	for _, want := range []string{"Diff: stress seed 9", "expected", "actual", "w save as Custom Test"} {
		if !strings.Contains(out, want) {
			t.Fatalf("overlay missing %q:\n%s", want, out)
		}
	}
	m, _ = send(m, "w")
	if saved != [2]string{"3\n", "6\n"} || !strings.Contains(plain(m), "saved as custom-3") {
		t.Fatalf("save: %q\n%s", saved, plain(m))
	}
	m, _ = send(m, "esc")
	if m.strs.diff || m.open == nil || !strings.Contains(plain(m), "saved as custom-3") {
		t.Fatalf("esc closes overlay only:\n%s", plain(m))
	}
}
