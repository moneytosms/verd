package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplain(t *testing.T) {
	asan := "==1==ERROR: AddressSanitizer: heap-buffer-overflow on address 0x1\n    #0 0x1 in main /home/u/1/A/main.cpp:12:9\n"
	py := "Traceback (most recent call last):\n  File \"/w/main.py\", line 4, in <module>\n    a[9]\nIndexError: list index out of range\n"
	for _, c := range []struct {
		name string
		r    Result
		hint string
		loc  string
	}{
		{"asan", Result{Verdict: RE, Stderr: asan}, "past the end of a heap", "/home/u/1/A/main.cpp:12"},
		{"python", Result{Verdict: RE, Stderr: py}, "List index", "/w/main.py:4"},
		{"segv", Result{Verdict: RE, Signal: "SIGSEGV"}, "Invalid memory access", ""},
		{"fpe", Result{Verdict: RE, Signal: "SIGFPE"}, "division or modulo by zero", ""},
		{"exit", Result{Verdict: RE, ExitCode: 3}, "code 3", ""},
		{"tle", Result{Verdict: TLE}, "Too slow", ""},
		{"wa", Result{Verdict: WA, Stderr: asan}, "", ""},
	} {
		got := Explain(c.r)
		if !strings.Contains(got.Hint, c.hint) || got.Loc != c.loc || (c.hint == "" && got.Hint != "") {
			t.Errorf("%s: got %+v", c.name, got)
		}
	}
}

// testdata/*.err are real g++ 16 runs under the sanitizer build (paths shortened to /w).
func TestExplainSanitizerOutput(t *testing.T) {
	for _, c := range []struct{ file, hint, loc, trace string }{
		{"dbg", "Index 4, but the container has size 4 (valid 0..3): one past the end", "/w/dbg.cpp:5", "dfs:5 ← dfs:6 ×4 ← main:8"},
		{"lb", "Binary search", "/w/lb.cpp:2", ""},
		{"rec", "Stack overflow", "/w/rec.cpp:2", "f:2 ×247"},
		{"uaf", "vector that has since grown", "/w/uaf.cpp:2", ""},
		{"glob", "Index 5, but the array has size 4", "/w/glob.cpp:3", ""},
		{"ub", "2147483647 + 1 overflows int", "/w/ub.cpp:2", ""},
		{"cmp", "strict weak order", "/w/cmp.cpp:2", ""},
	} {
		b, err := os.ReadFile("testdata/" + c.file + ".err")
		if err != nil {
			t.Fatal(err)
		}
		x := Explain(Result{Verdict: RE, Stderr: string(b)})
		if !strings.Contains(x.Hint, c.hint) || x.Loc != c.loc || x.Trace != c.trace {
			t.Errorf("%s: got %+v", c.file, x)
		}
	}
}

func TestIndexHint(t *testing.T) {
	for _, c := range []struct{ idx, n, want string }{
		{"18446744073709551615", "5", "Index -1, but the container has size 5 (valid 0..4): a negative index"},
		{"0", "0", "it is empty"},
		{"9", "5", "far past the end"},
	} {
		if got := indexHint(c.idx, c.n, "container"); !strings.Contains(got, c.want) {
			t.Errorf("%s/%s: %q", c.idx, c.n, got)
		}
	}
	if got := memHint("4", "0", "after", "16", "heap array"); !strings.Contains(got, "Index 4, but the heap array has size 4") {
		t.Errorf("memHint after: %q", got)
	}
	if got := memHint("4", "4", "before", "16", "heap array"); !strings.Contains(got, "Index -1") {
		t.Errorf("memHint before: %q", got)
	}
}

func TestSourceLine(t *testing.T) {
	f := filepath.Join(t.TempDir(), "main.cpp")
	os.WriteFile(f, []byte("int main() {\n    vis[v] = 1;\n}\n"), 0o644)
	if got := sourceLine(f + ":2"); got != "vis[v] = 1;" {
		t.Errorf("got %q", got)
	}
	if got := sourceLine(f + ":9"); got != "" {
		t.Errorf("past the end: %q", got)
	}
}

func TestFold(t *testing.T) {
	b, _ := os.ReadFile("testdata/dbg.err")
	got := Fold(string(b))
	for _, want := range []string{"Error: attempt to subscript", "⋯ 6 lines of library function signature", "⋯ 5 library frames",
		"in dfs(int, std::__debug::vector<int, std::allocator<int> >&) /w/dbg.cpp:5", "⋯ 3 repeats of the frame above (recursion)", "in main /w/dbg.cpp:8"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	for _, gone := range []string{"DEADLYSIGNAL", "ABRT", "rax =", "libc.so"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q not folded:\n%s", gone, got)
		}
	}
	if plain := "line 1\nline 2"; Fold(plain) != plain {
		t.Errorf("plain stderr changed: %q", Fold(plain))
	}
}
