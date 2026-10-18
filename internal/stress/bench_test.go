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

// BenchmarkOpts reports iters/s with --batch and --multi on trivial C++ programs.
func BenchmarkOpts(b *testing.B) {
	s := cppSpec(b, 2000)
	dir := filepath.Dir(s.Gen.Path)
	l := s.Gen.Lang
	w := func(n, c string) Program {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte(c), 0o644)
		return Program{Path: p, Lang: l}
	}
	const multi = "#include <cstdio>\nint main(){int t,n;scanf(\"%d\",&t);while(t--){scanf(\"%d\",&n);printf(\"%d\\n\",n*2);}}\n"
	batchGen := w("bgen.cpp", "#include <cstdio>\n#include <cstdlib>\nint main(int c,char**v){int s=atoi(v[1]),n=atoi(v[2]);for(int i=s;i<s+n;i++)printf(\"%d\\n---\\n\",i%100);}\n")
	for _, c := range []struct {
		name string
		s    Spec
	}{
		{"batch", func() Spec { x := s; x.Gen, x.Batch = batchGen, 100; return x }()},
		{"batch+multi", func() Spec {
			x := s
			x.Gen, x.Batch, x.Multi = batchGen, 100, 100
			x.Solution, x.Brute = w("m.cpp", multi), w("mb.cpp", multi)
			return x
		}()},
	} {
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				start := time.Now()
				r, _ := final(Run(context.Background(), c.s))
				if r.Kind != Limit {
					b.Fatalf("%+v", r)
				}
				b.ReportMetric(float64(r.Iterations)/time.Since(start).Seconds(), "iters/s")
			}
		})
	}
}
