package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/scrape"
)

func exitCode(err error) int {
	var ee *exitError
	if err == nil {
		return 0
	}
	if errors.As(err, &ee) {
		return ee.code
	}
	return -1
}

func setup(t *testing.T, src, name string, d *scrape.Detail) (cfg config.Config, path, cache string, detail detailFunc) {
	t.Helper()
	ws := t.TempDir()
	cfg = config.Default()
	cfg.Workspace = ws
	dir := filepath.Join(ws, "1", "A")
	os.MkdirAll(dir, 0o755)
	path = filepath.Join(dir, name)
	os.WriteFile(path, []byte(src), 0o644)
	detail = func(context.Context, int, string) (*scrape.Detail, error) { return d, nil }
	return cfg, path, t.TempDir(), detail
}

var sumDetail = &scrape.Detail{TimeLimitMS: 2000, MemoryLimitMB: 256, Samples: []scrape.Sample{{Input: "1 2\n", Output: "3\n"}, {Input: "5 5\n", Output: "10\n"}}}

func TestVerdTestACAndWA(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 missing")
	}
	cfg, path, cache, detail := setup(t, "a,b=map(int,input().split())\nprint(a+b)\n", "main.py", sumDetail)
	var out bytes.Buffer
	if err := testCmd(context.Background(), &out, cfg, cache, detail, nil, path); err != nil {
		t.Fatalf("all AC must exit 0: %v\n%s", err, out.String())
	}
	for _, want := range []string{"1A  python", "sample-1", "sample-2", "AC", "2/2 AC"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	// samples were materialized as files
	if b, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "tests", "sample-2.ans")); string(b) != "10\n" {
		t.Errorf("sample-2.ans = %q", b)
	}
	// wrong answer: non-zero exit, mismatch reported
	os.WriteFile(path, []byte("a,b=map(int,input().split())\nprint(a+b+(1 if a==5 else 0))\n"), 0o644)
	out.Reset()
	err := testCmd(context.Background(), &out, cfg, cache, detail, nil, path)
	if exitCode(err) != 1 || !strings.Contains(out.String(), `line 1 col 1: want "10" got "11"`) || !strings.Contains(out.String(), "1/2 AC") {
		t.Fatalf("WA: code %d\n%s", exitCode(err), out.String())
	}
}

func TestVerdTestCompileErrorAndCache(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("g++ missing")
	}
	cfg, path, cache, detail := setup(t, "int main(){ return nope; }", "main.cpp", sumDetail)
	var out bytes.Buffer
	err := testCmd(context.Background(), &out, cfg, cache, detail, nil, path)
	if exitCode(err) != 1 || !strings.Contains(out.String(), "compile: FAILED") || !strings.Contains(out.String(), "nope") {
		t.Fatalf("CE: code %d\n%s", exitCode(err), out.String())
	}
	os.WriteFile(path, []byte("#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<\"\\n\";}"), 0o644)
	out.Reset()
	if err := testCmd(context.Background(), &out, cfg, cache, detail, nil, path); err != nil || !strings.Contains(out.String(), "compile: ok\n") {
		t.Fatalf("%v\n%s", err, out.String())
	}
	out.Reset()
	testCmd(context.Background(), &out, cfg, cache, detail, nil, path)
	if !strings.Contains(out.String(), "compile: ok (cached)") {
		t.Fatalf("second run should be cached:\n%s", out.String())
	}
}

func TestVerdTestRefusals(t *testing.T) {
	cfg, path, cache, _ := setup(t, "", "main.py", nil)
	inter := func(context.Context, int, string) (*scrape.Detail, error) {
		return &scrape.Detail{Interactive: true}, nil
	}
	var out bytes.Buffer
	if err := testCmd(context.Background(), &out, cfg, cache, inter, nil, path); exitCode(err) != 2 || !strings.Contains(err.Error(), "interactive") {
		t.Fatalf("interactive: %v", err)
	}
	if err := testCmd(context.Background(), &out, cfg, cache, inter, nil, "/etc/hostname"); exitCode(err) != 2 {
		t.Fatalf("outside workspace: %v", err)
	}
	empty := func(context.Context, int, string) (*scrape.Detail, error) { return &scrape.Detail{}, nil }
	if err := testCmd(context.Background(), &out, cfg, cache, empty, nil, path); exitCode(err) != 2 || !strings.Contains(err.Error(), "no tests") {
		t.Fatalf("no tests: %v", err)
	}
}

func TestVerdTestHonorsSavedModeAndHint(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 missing")
	}
	d := &scrape.Detail{TimeLimitMS: 2000, MemoryLimitMB: 256, Hint: "float", Samples: []scrape.Sample{{Input: "\n", Output: "0.3333333333\n"}}}
	cfg, path, cache, detail := setup(t, "print('0.333333')\n", "main.py", d)
	detail = func(context.Context, int, string) (*scrape.Detail, error) { return d, nil }
	var out bytes.Buffer
	// hint -> float: within eps, AC
	if err := testCmd(context.Background(), &out, cfg, cache, detail, nil, path); err != nil || !strings.Contains(out.String(), "float") {
		t.Fatalf("hint should select float: %v\n%s", err, out.String())
	}
	// saved mode beats the hint: tokens -> WA
	out.Reset()
	saved := func(int, string) string { return "tokens" }
	if err := testCmd(context.Background(), &out, cfg, cache, detail, saved, path); exitCode(err) != 1 || !strings.Contains(out.String(), "tokens") {
		t.Fatalf("saved mode should win: %v\n%s", err, out.String())
	}
}
