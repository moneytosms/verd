package stress

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/runner"
)

func cppSpec(b *testing.B, iters int) Spec {
	if _, err := exec.LookPath("g++"); err != nil {
		b.Skip("g++ missing")
	}
	dir := b.TempDir()
	l := config.Default().Lang["cpp"]
	l.Compile = append([]string{"g++"}, l.Compile[1:]...)
	w := func(n, c string) Program {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte(c), 0o644)
		return Program{Path: p, Lang: l}
	}
	const twice = "#include <cstdio>\nint main(){int n;scanf(\"%d\",&n);printf(\"%d\\n\",n*2);}\n"
	return Spec{
		Gen:      w("gen.cpp", "#include <cstdio>\n#include <cstdlib>\nint main(int c,char**v){printf(\"%d\\n\",atoi(v[1])%100);}\n"),
		Solution: w("main.cpp", twice), Brute: w("brute.cpp", twice),
		CacheDir: filepath.Join(dir, "b"), MaxIter: iters, Timeout: 10 * time.Minute,
	}
}

// BenchmarkIterations reports stress iterations/sec with trivial C++ programs.
func BenchmarkIterations(b *testing.B) {
	s := cppSpec(b, 500)
	for i := 0; i < b.N; i++ {
		start := time.Now()
		r, _ := final(Run(context.Background(), s))
		if r.Kind != Limit {
			b.Fatalf("%+v", r)
		}
		b.ReportMetric(float64(r.Iterations)/time.Since(start).Seconds(), "iters/s")
	}
}

// BenchmarkSpawn reports the cost of one trivial process spawn (exec1).
func BenchmarkSpawn(b *testing.B) {
	s := cppSpec(b, 1)
	argv, _, err := runner.Build(context.Background(), s.Solution.Lang, s.Solution.Path, s.CacheDir)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		exec1(context.Background(), argv, "5\n", 10*time.Second)
	}
}
