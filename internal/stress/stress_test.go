package stress

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/config"
)

func setup(t *testing.T, sol, brute string) Spec {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 missing")
	}
	dir := t.TempDir()
	w := func(n, c string) Program {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte(c), 0o644)
		return Program{Path: p, Lang: config.Default().Lang["python"]}
	}
	return Spec{
		Gen:      w("gen.py", "import sys,random\nr=random.Random(int(sys.argv[1]))\nprint(r.randint(1,100))\n"),
		Solution: w("main.py", sol), Brute: w("brute.py", brute),
		CacheDir: filepath.Join(dir, "b"), MaxIter: 40, Timeout: 20 * time.Second,
	}
}

func final(ch <-chan Event) (Result, int) {
	n := 0
	for e := range ch {
		if e.Done {
			return e.Result, n
		}
		n++
	}
	return Result{}, n
}

const good = "print(int(input())*2)\n"

func TestBuggySolutionGivesReproducibleSeed(t *testing.T) {
	s := setup(t, "n=int(input())\nprint(n*2 if n<=50 else 0)\n", good)
	s.MaxIter = 1000
	r, _ := final(Run(context.Background(), s))
	if r.Kind != Mismatch || r.Input == "" || r.Want == r.Got {
		t.Fatalf("%+v", r)
	}
	r2, _ := final(Run(context.Background(), s))
	if r2.Seed != r.Seed || r2.Input != r.Input {
		t.Fatalf("seed must reproduce: %d vs %d", r.Seed, r2.Seed)
	}
}

func TestCorrectHitsLimitWithProgress(t *testing.T) {
	r, n := final(Run(context.Background(), setup(t, good, good)))
	if r.Kind != Limit || r.Iterations != 40 || n != 40 {
		t.Fatalf("%+v events=%d", r, n)
	}
}

func TestCrashingBruteIsError(t *testing.T) {
	r, _ := final(Run(context.Background(), setup(t, good, "import sys\nsys.exit(3)\n")))
	if r.Kind != Error || r.Err == "" {
		t.Fatalf("%+v", r)
	}
}

func TestSolutionRETLEAndCancel(t *testing.T) {
	r, _ := final(Run(context.Background(), setup(t, "raise SystemExit(2)\n", good)))
	if r.Kind != SolutionRE {
		t.Fatalf("%+v", r)
	}
	s := setup(t, "while True: pass\n", good)
	s.TimeLimitMS = 200
	if r, _ := final(Run(context.Background(), s)); r.Kind != SolutionTLE {
		t.Fatalf("%+v", r)
	}
	s = setup(t, good, good)
	s.MaxIter = 1 << 30
	ctx, cancel := context.WithCancel(context.Background())
	ch := Run(ctx, s)
	<-ch
	cancel()
	if r, _ := final(ch); r.Kind != Cancelled {
		t.Fatalf("%+v", r)
	}
}
