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

// Higher failing seeds finish first; the lowest one must still be reported.
func TestParallelReportsLowestFailingSeed(t *testing.T) {
	s := setup(t, "import time\nn=int(input())\ntime.sleep((13-n)*0.03 if n<13 else 0)\nprint(n*2+(1 if n in (5,9,12) else 0))\n", good)
	s.Gen.Path = filepath.Join(filepath.Dir(s.Gen.Path), "seed.py")
	os.WriteFile(s.Gen.Path, []byte("import sys\nprint(sys.argv[1])\n"), 0o644)
	s.Jobs = 8
	for range 3 {
		r, _ := final(Run(context.Background(), s))
		if r.Kind != Mismatch || r.Seed != 5 || r.Iterations != 5 {
			t.Fatalf("%+v", r)
		}
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

// Batch gen: one run prints N cases; seeds, Iterations and the lowest failing seed match the per-seed path.
func TestBatchGen(t *testing.T) {
	s := setup(t, "n=int(input())\nprint(n*2+(1 if n==37 else 0))\n", good)
	s.Gen.Path = filepath.Join(filepath.Dir(s.Gen.Path), "batch.py")
	os.WriteFile(s.Gen.Path, []byte("import sys\ns,n=int(sys.argv[1]),int(sys.argv[2])\nfor i in range(s,s+n): print(i); print('---')\n"), 0o644)
	s.Batch, s.MaxIter = 8, 100
	r, _ := final(Run(context.Background(), s))
	if r.Kind != Mismatch || r.Seed != 37 || r.Input != "37\n" {
		t.Fatalf("%+v", r)
	}
	s.MaxIter = 20
	if r, _ := final(Run(context.Background(), s)); r.Kind != Limit || r.Iterations != 20 {
		t.Fatalf("%+v", r)
	}
	s.Batch = 5 // gen still prints 5 per 5: fine; a wrong count is an Error
	s.Gen.Path = filepath.Join(filepath.Dir(s.Gen.Path), "short.py")
	os.WriteFile(s.Gen.Path, []byte("print(1)\n"), 0o644)
	if r, _ := final(Run(context.Background(), s)); r.Kind != Error || r.Err == "" {
		t.Fatalf("%+v", r)
	}
}

// Multi: Solution and brute read t then t cases; failures fall back to per-seed runs so the seed is exact.
func TestMulti(t *testing.T) {
	multi := func(f string) string {
		return "t=int(input())\nfor _ in range(t):\n    n=int(input())\n    print(" + f + ")\n"
	}
	s := setup(t, multi("n*2+(1 if n==37 else 0)"), multi("n*2"))
	s.Gen.Path = filepath.Join(filepath.Dir(s.Gen.Path), "seed.py")
	os.WriteFile(s.Gen.Path, []byte("import sys\nprint(sys.argv[1])\n"), 0o644)
	s.Multi, s.MaxIter, s.Jobs = 8, 100, 4
	for range 3 {
		r, _ := final(Run(context.Background(), s))
		if r.Kind != Mismatch || r.Seed != 37 || r.Iterations != 37 || r.Input != "1\n37\n" {
			t.Fatalf("%+v", r)
		}
	}
	s.Solution.Path = filepath.Join(filepath.Dir(s.Gen.Path), "ok.py")
	os.WriteFile(s.Solution.Path, []byte(multi("n*2")), 0o644)
	s.MaxIter = 20 // not a multiple of Multi
	r, n := final(Run(context.Background(), s))
	if r.Kind != Limit || r.Iterations != 20 || n != 20 {
		t.Fatalf("%+v events=%d", r, n)
	}
}
