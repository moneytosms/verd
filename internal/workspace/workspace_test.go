package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/scrape"
)

func TestResolve(t *testing.T) {
	root := t.TempDir()
	w := Workspace{Root: root}
	langs := config.Default().Lang
	r, err := w.Resolve(filepath.Join(root, "1900", "A", "main.cpp"), langs)
	if err != nil || r.Contest != 1900 || r.Index != "A" || r.Lang != "cpp" {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := w.Resolve(filepath.Join(root, "4", "B1", "main.py"), langs); err != nil || r.Lang != "python" || r.Index != "B1" {
		t.Fatalf("%+v %v", r, err)
	}
	for name, p := range map[string]string{
		"outside":     "/etc/passwd",
		"too shallow": filepath.Join(root, "main.cpp"),
		"bad contest": filepath.Join(root, "abc", "A", "main.cpp"),
		"unknown ext": filepath.Join(root, "1", "A", "main.zig"),
	} {
		if _, err := w.Resolve(p, langs); err == nil {
			t.Errorf("%s should fail", name)
		}
	}
}

func TestSamplesAndTests(t *testing.T) {
	w := Workspace{Root: t.TempDir()}
	if err := w.WriteSamples(1, "A", []scrape.Sample{{Input: "1\n", Output: "2\n"}, {Input: "3\n", Output: "4\n"}}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.Dir(1, "A"), "tests")
	for _, n := range []string{"custom-2", "custom-10"} { // numeric, not lexical, order
		os.WriteFile(filepath.Join(dir, n+".in"), []byte("x"), 0o644)
		os.WriteFile(filepath.Join(dir, n+".ans"), []byte("y"), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "custom-3.in"), []byte("x"), 0o644) // no .ans: ignored
	ts, err := w.Tests(1, "A")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, x := range ts {
		names = append(names, x.Name)
	}
	want := []string{"sample-1", "sample-2", "custom-2", "custom-10"}
	if len(names) != len(want) {
		t.Fatalf("got %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v want %v", names, want)
		}
	}
	if b, _ := os.ReadFile(ts[1].Ans); string(b) != "4\n" {
		t.Fatalf("sample-2.ans = %q", b)
	}
	if ts, err := w.Tests(9, "Z"); err != nil || ts != nil {
		t.Fatal("missing tests dir is empty, not an error")
	}
}

func TestNextCustomSkipsExisting(t *testing.T) {
	w := Workspace{Root: t.TempDir()}
	n, in, ans, err := w.NextCustom(1, "A")
	if err != nil || n != 1 || filepath.Base(in) != "custom-1.in" || filepath.Base(ans) != "custom-1.ans" {
		t.Fatalf("%d %s %s %v", n, in, ans, err)
	}
	for _, p := range []string{in, ans} {
		if st, err := os.Stat(p); err != nil || st.Size() != 0 {
			t.Fatalf("%s should exist empty: %v", p, err)
		}
	}
	os.WriteFile(in, []byte("keep"), 0o644)
	dir := filepath.Dir(in)
	os.WriteFile(filepath.Join(dir, "custom-7.in"), nil, 0o644) // a lone .in still counts
	n, in2, _, err := w.NextCustom(1, "A")
	if err != nil || n != 8 {
		t.Fatalf("should skip past 7: %d %v", n, err)
	}
	if b, _ := os.ReadFile(in); string(b) != "keep" {
		t.Fatal("existing custom test touched")
	}
	_ = in2
	// created pairs are picked up by Tests, after the samples
	w.WriteSamples(1, "A", []scrape.Sample{{Input: "1\n", Output: "1\n"}})
	ts, _ := w.Tests(1, "A")
	var names []string
	for _, x := range ts {
		names = append(names, x.Name)
	}
	if strings.Join(names, ",") != "sample-1,custom-1,custom-8" {
		t.Fatalf("custom tests must run with samples: %v", names)
	}
}

func TestSaveCustomWritesNextPair(t *testing.T) {
	w := New(t.TempDir())
	for want := 1; want <= 2; want++ {
		name, err := w.SaveCustom(1, "A", "3\n", "6\n")
		if err != nil || name != fmt.Sprintf("custom-%d", want) {
			t.Fatalf("%q %v", name, err)
		}
	}
	tests, _ := w.Tests(1, "A")
	b, _ := os.ReadFile(tests[1].Ans)
	if len(tests) != 2 || string(b) != "6\n" {
		t.Fatalf("%+v %q", tests, b)
	}
}

func TestNewMakesRelativeRootAbsolute(t *testing.T) {
	wd, _ := os.Getwd()
	if got := New(".").Root; got != wd {
		t.Fatalf("%q want %q", got, wd)
	}
}
