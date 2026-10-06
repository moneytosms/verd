package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/workspace"
)

func need(t *testing.T, bin string) {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s not installed", bin)
	}
}

type fixture struct {
	t     *testing.T
	dir   string
	cache string
}

func newFixture(t *testing.T) *fixture {
	return &fixture{t: t, dir: t.TempDir(), cache: filepath.Join(t.TempDir(), "build")}
}

func (f *fixture) write(name, content string) string {
	p := filepath.Join(f.dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) test(name, in, ans string) workspace.Test {
	return workspace.Test{Name: name, In: f.write(name+".in", in), Ans: f.write(name+".ans", ans)}
}

func (f *fixture) run(src string, lang string, tests []workspace.Test, mod func(*Spec)) (results []Result, events []Event) {
	spec := Spec{Solution: src, Lang: config.Default().Lang[lang], CacheDir: f.cache, Tests: tests, TimeLimitMS: 2000, MemoryMB: 256}
	if mod != nil {
		mod(&spec)
	}
	for ev := range Run(context.Background(), spec) {
		events = append(events, ev)
		if ev.Kind == TestFinished {
			results = append(results, ev.Result)
		}
	}
	return
}

const cppSum = `#include <iostream>
int main(){ long long a,b; std::cin>>a>>b; std::cout<<a+b<<"\n"; }`

func TestCppACWACached(t *testing.T) {
	need(t, "g++")
	f := newFixture(t)
	src := f.write("main.cpp", cppSum)
	tests := []workspace.Test{f.test("sample-1", "1 2\n", "3\n"), f.test("sample-2", "5 5\n", "11\n")}
	res, evs := f.run(src, "cpp", tests, nil)
	if len(res) != 2 || res[0].Verdict != AC || res[1].Verdict != WA {
		t.Fatalf("%+v", res)
	}
	if m := res[1].Mismatch; m == nil || m.Line != 1 || m.Col != 1 || m.Want != "11" || m.Got != "10" {
		t.Fatalf("mismatch %+v", m)
	}
	if evs[1].Kind != CompileFinished || evs[1].Cached {
		t.Fatalf("first run must compile: %+v", evs[1])
	}
	if last := evs[len(evs)-1]; last.Kind != Done || last.Verdict != WA {
		t.Fatalf("overall should be first failure: %+v", last)
	}
	if res[0].MemoryMB <= 0 || res[0].TimeMS < 0 {
		t.Fatalf("stats not measured: %+v", res[0])
	}

	// unchanged source: no recompile
	_, evs = f.run(src, "cpp", tests, nil)
	if !evs[1].Cached {
		t.Fatal("second run of unchanged source must hit the compile cache")
	}
	// changed source: recompiles
	f.write("main.cpp", cppSum+"\n// changed\n")
	_, evs = f.run(src, "cpp", tests, nil)
	if evs[1].Cached {
		t.Fatal("changed source must recompile")
	}
	// changed compile argv: recompiles
	_, evs = f.run(src, "cpp", tests, func(s *Spec) { s.Lang.Compile = append(append([]string{}, s.Lang.Compile...), "-DX=1") })
	if evs[1].Cached {
		t.Fatal("changed compile flags must recompile")
	}
}

func TestPythonAC(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "a,b=map(int,input().split())\nprint(a+b)\n")
	res, evs := f.run(src, "python", []workspace.Test{f.test("sample-1", "1 2\n", "3\n")}, nil)
	if len(res) != 1 || res[0].Verdict != AC {
		t.Fatalf("%+v", res)
	}
	if evs[len(evs)-1].Verdict != AC {
		t.Fatal("overall AC")
	}
}

func TestCompileError(t *testing.T) {
	need(t, "g++")
	f := newFixture(t)
	src := f.write("main.cpp", "int main(){ return nope; }")
	res, evs := f.run(src, "cpp", []workspace.Test{f.test("sample-1", "", "")}, nil)
	if len(res) != 0 {
		t.Fatal("no tests should run after CE")
	}
	if ev := evs[1]; ev.Kind != CompileFinished || !strings.Contains(ev.Err, "nope") {
		t.Fatalf("compiler stderr should surface: %+v", ev)
	}
	if evs[len(evs)-1].Verdict != CE {
		t.Fatal("overall CE")
	}
	// a failed build must not look cached
	_, evs = f.run(src, "cpp", nil, nil)
	if evs[1].Err == "" {
		t.Fatal("CE must repeat, not be cached as success")
	}
}

func TestRuntimeError(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "import sys\nsys.exit(3)\n")
	res, _ := f.run(src, "python", []workspace.Test{f.test("sample-1", "", "")}, nil)
	if res[0].Verdict != RE || res[0].ExitCode != 3 {
		t.Fatalf("%+v", res[0])
	}
}

func TestTLEIsKilledWithinHardLimit(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "while True: pass\n")
	start := time.Now()
	res, evs := f.run(src, "python", []workspace.Test{f.test("sample-1", "", "")}, func(s *Spec) { s.TimeLimitMS = 200 })
	if res[0].Verdict != TLE || evs[len(evs)-1].Verdict != TLE {
		t.Fatalf("%+v", res[0])
	}
	// hard kill at max(2*TL, TL+1s) = 1.2s
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("runaway not killed in time: %v", d)
	}
	if res[0].TimeMS < 1000 {
		t.Fatalf("TLE should report time up to the hard kill, got %d ms", res[0].TimeMS)
	}
}

func TestTimeMultiplier(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "import time\ntime.sleep(0.4)\nprint(1)\n")
	tests := []workspace.Test{f.test("sample-1", "", "1\n")}
	if res, _ := f.run(src, "python", tests, func(s *Spec) { s.TimeLimitMS = 200 }); res[0].Verdict != TLE {
		t.Fatalf("0.4s with TL 0.2s should be TLE: %+v", res[0])
	}
	if res, _ := f.run(src, "python", tests, func(s *Spec) { s.TimeLimitMS = 200; s.Multiplier = 5 }); res[0].Verdict != AC {
		t.Fatalf("multiplier 5 should allow it: %+v", res[0])
	}
}

func TestOutputCap(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "while True: print('x'*1000)\n")
	start := time.Now()
	res, _ := f.run(src, "python", []workspace.Test{f.test("sample-1", "", "")}, func(s *Spec) { s.OutputCap = 10_000 })
	if res[0].Verdict != RE || !strings.Contains(res[0].Note, "output limit") {
		t.Fatalf("%+v", res[0])
	}
	if len(res[0].Output) > 10_000 || time.Since(start) > 3*time.Second {
		t.Fatalf("output must be capped and the process killed quickly: %d bytes in %v", len(res[0].Output), time.Since(start))
	}
}

func TestSoftMemoryLimit(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "x = bytearray(150*1024*1024)\nprint(len(x) > 0)\n")
	res, _ := f.run(src, "python", []workspace.Test{f.test("sample-1", "", "True\n")}, func(s *Spec) { s.MemoryMB = 50 })
	if res[0].Verdict != MLE || res[0].MemoryMB < 100 {
		t.Fatalf("%+v", res[0])
	}
}

func TestModeNoneAndFloat(t *testing.T) {
	need(t, "python3")
	f := newFixture(t)
	src := f.write("main.py", "print('0.3333333')\n")
	tests := []workspace.Test{f.test("sample-1", "", "0.3333333333\n")}
	if res, _ := f.run(src, "python", tests, func(s *Spec) { s.Mode = Float }); res[0].Verdict != AC {
		t.Fatalf("float: %+v", res[0])
	}
	if res, _ := f.run(src, "python", tests, nil); res[0].Verdict != WA {
		t.Fatalf("tokens: %+v", res[0])
	}
	if res, _ := f.run(src, "python", tests, func(s *Spec) { s.Mode = None }); res[0].Verdict != Unk {
		t.Fatalf("none: %+v", res[0])
	}
}

// A big parent must not make a small child look big (children inherit the parent's peak RSS).
func TestMemoryNotInheritedFromParent(t *testing.T) {
	need(t, "g++")
	ballast := make([]byte, 150<<20)
	for i := 0; i < len(ballast); i += 4096 {
		ballast[i] = 1
	}
	f := newFixture(t)
	src := f.write("main.cpp", cppSum)
	res, _ := f.run(src, "cpp", []workspace.Test{f.test("sample-1", "1 2\n", "3\n")}, func(s *Spec) { s.MemoryMB = 64 })
	if res[0].Verdict != AC || res[0].MemoryMB > 64 {
		t.Fatalf("%+v", res[0])
	}
	_ = ballast[len(ballast)-1]
}
