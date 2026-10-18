package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/config"
)

func TestListSnippetsUserOverridesAndAdds(t *testing.T) {
	dir := t.TempDir()
	l := config.Default().Lang["python"]
	base := ListSnippets(dir, "python", l)
	if len(base) < 20 || base[0].Desc == "" || strings.HasPrefix(base[0].Desc, "#") {
		t.Fatalf("embedded library: %d snippets, first %+v", len(base), base[0])
	}
	lib := filepath.Join(dir, "library", "python")
	os.MkdirAll(lib, 0o755)
	os.WriteFile(filepath.Join(lib, "mine.py"), []byte("# my helper\nx = 1\n"), 0o644)
	os.WriteFile(filepath.Join(lib, "modpow.py"), []byte("# custom\n"), 0o644)
	os.WriteFile(filepath.Join(lib, "notes.txt"), []byte("ignored"), 0o644)
	got := ListSnippets(dir, "python", l)
	if len(got) != len(base)+1 {
		t.Fatalf("want one added snippet, got %d vs %d", len(got), len(base))
	}
	s, err := FindSnippet(dir, "python", l, "modpow")
	if err != nil || s.Desc != "custom" {
		t.Fatalf("user file must win: %+v %v", s, err)
	}
	if s, err := FindSnippet(dir, "python", l, "min"); err != nil || s.Name != "mine" {
		t.Fatalf("unique prefix: %+v %v", s, err)
	}
	if _, err := FindSnippet(dir, "python", l, "prefix_"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("want ambiguous, got %v", err)
	}
	if _, err := FindSnippet(dir, "python", l, "nope"); err == nil {
		t.Fatal("want not found")
	}
}

func TestInitSnippets(t *testing.T) {
	dir := t.TempDir()
	langs := config.Default().Lang
	n, err := InitSnippets(dir, langs, false)
	if err != nil || n < 90 {
		t.Fatalf("%d %v", n, err)
	}
	path := filepath.Join(dir, "library", "cpp", "dsu.cpp")
	os.WriteFile(path, []byte("// mine\n"), 0o644)
	if n, _ := InitSnippets(dir, langs, false); n != 0 {
		t.Fatalf("init must not overwrite: wrote %d", n)
	}
	if b, _ := os.ReadFile(path); string(b) != "// mine\n" {
		t.Fatal("user snippet clobbered")
	}
	if n, _ := InitSnippets(dir, langs, true); n < 90 {
		t.Fatalf("--force rewrites all: %d", n)
	}
}

// Every shipped snippet must compile or parse: top-level ones above main, statements inside it.
func TestShippedSnippetsBuild(t *testing.T) {
	const cppPre = "#include <bits/stdc++.h>\nusing namespace std;\n"
	const cPre = "#include <stdio.h>\n#include <stdlib.h>\n#include <string.h>\n#include <math.h>\n#include <limits.h>\n"
	graph, _ := defaultSnippets.ReadFile("snippets/c/graphinput.c")
	needsGraph := map[string]bool{"bfs": true, "dfs": true, "dijkstra": true, "topo_sort": true}
	vars := map[string]string{
		"cpp": "int n = 3, m = 3; vector<long long> a(3); vector<vector<long long>> g(3, vector<long long>(3));",
		"c":   "int n = 3, m = 3; static long long a[8]; static long long g[8][8];",
	}
	for key, l := range config.Default().Lang {
		for _, s := range ListSnippets(t.TempDir(), key, l) {
			t.Run(key+"/"+s.Name, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				write := func(name, src string) string {
					p := filepath.Join(dir, name)
					os.WriteFile(p, []byte(src), 0o644)
					return p
				}
				try := func(name string, argv ...string) error {
					if _, err := exec.LookPath(argv[0]); err != nil {
						t.Skip(argv[0] + " missing")
					}
					out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
					if err != nil {
						t.Logf("%s: %s", name, out)
					}
					return err
				}
				switch key {
				case "python":
					p := write("s.py", s.Code)
					if err := try("parse", "python3", "-I", "-c", "import sys; compile(open(sys.argv[1]).read(), 'snippet', 'exec')", p); err != nil {
						t.Fatal(err)
					}
				case "cpp":
					// the configured compiler: on macOS that is a Homebrew GNU g++ when there is one, since Apple Clang has no <bits/stdc++.h>
					cxx := config.Default().Lang["cpp"].Compile[0]
					probe := write("probe.cpp", cppPre+"int main() {}\n")
					if exec.Command(cxx, "-std=c++20", "-fsyntax-only", probe).Run() != nil {
						t.Skip(cxx + " cannot include <bits/stdc++.h>")
					}
					top := write("top.cpp", cppPre+s.Code+"\nint main() {}\n")
					if try("top-level", cxx, "-std=c++20", "-fsyntax-only", top) == nil {
						return
					}
					in := write("in.cpp", cppPre+"int main() {\n"+vars[key]+"\n{\n"+s.Code+"\n}\n}\n")
					if err := try("in main", cxx, "-std=c++20", "-fsyntax-only", in); err != nil {
						t.Fatal(err)
					}
				case "c":
					pre := cPre
					if needsGraph[s.Name] {
						pre += string(graph) + "\n"
					}
					top := write("top.c", pre+s.Code+"\nint main(void) { return 0; }\n")
					if try("top-level", "gcc", "-std=c11", "-fsyntax-only", top) == nil {
						return
					}
					in := write("in.c", cPre+"int main(void) {\n"+vars[key]+"\n{\n"+s.Code+"\n}\nreturn 0;\n}\n")
					if err := try("in main", "gcc", "-std=c11", "-fsyntax-only", in); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestSnippetDescriptionByLanguage(t *testing.T) {
	c, py := config.Default().Lang["c"], config.Default().Lang["python"]
	if s := newSnippet("x", c, []byte("#include <stdio.h>\nint x;\n")); s.Desc != "" {
		t.Fatalf("a C preprocessor line is not a description: %q", s.Desc)
	}
	if s := newSnippet("x", py, []byte("# hi there\nx = 1\n")); s.Desc != "hi there" {
		t.Fatalf("%q", s.Desc)
	}
	if got := WithDescription("int x;\n", "x", "", c); got != "// x\nint x;\n" {
		t.Fatalf("%q", got)
	}
	if got := WithDescription("// have\nint x;\n", "x", "", c); got != "// have\nint x;\n" {
		t.Fatalf("%q", got)
	}
	if got := WithDescription("#include <a.h>\n", "x", "", c); got != "// x\n#include <a.h>\n" {
		t.Fatalf("%q", got)
	}
}
