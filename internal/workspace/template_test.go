package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/config"
)

func vars() TemplateVars {
	return NewVars(1900, "A", "Cover in Water", "tourist", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))
}

func TestRenderTemplate(t *testing.T) {
	text, line, err := RenderTemplate("// {{.Problem.ID}} {{.Problem.Name}}\n// {{.Problem.URL}}\n// {{.Handle}} {{.Date}}\nint main() {\n    {{cursor}}\n}\n", vars())
	if err != nil {
		t.Fatal(err)
	}
	want := "// 1900A Cover in Water\n// https://codeforces.com/problemset/problem/1900/A\n// tourist 2026-10-05\nint main() {\n    \n}\n"
	if text != want || line != 5 {
		t.Fatalf("line %d\n%q", line, text)
	}
	if _, line, _ := RenderTemplate("no marker\n", vars()); line != 1 {
		t.Fatalf("no marker should start at line 1, got %d", line)
	}
	if _, line, _ := RenderTemplate("{{cursor}}x", vars()); line != 1 {
		t.Fatalf("marker on first line: %d", line)
	}
	if _, _, err := RenderTemplate("{{.Nope}}", vars()); err == nil {
		t.Fatal("bad variable must error")
	}
	if _, _, err := RenderTemplate("{{", vars()); err == nil {
		t.Fatal("bad syntax must error")
	}
}

func TestDefaultTemplatesRender(t *testing.T) {
	for k, l := range config.Default().Lang {
		tmpl, err := LoadTemplate(t.TempDir(), k, l)
		if err != nil || tmpl == "" {
			t.Fatalf("%s: no default template: %v", k, err)
		}
		text, line, err := RenderTemplate(tmpl, vars())
		if err != nil || line <= 1 || strings.Contains(text, "verd-cursor") || !strings.Contains(text, "Cover in Water") {
			t.Errorf("%s: line=%d err=%v\n%s", k, line, err, text)
		}
	}
}

func TestEnsureNeverOverwrites(t *testing.T) {
	w := Workspace{Root: t.TempDir()}
	l := config.Default().Lang["cpp"]
	path, line, created, err := w.Ensure(1900, "A", "cpp", l, "a\n{{cursor}}\nb\n", vars())
	if err != nil || !created || line != 2 || path != filepath.Join(w.Root, "1900", "A", "main.cpp") {
		t.Fatalf("%q %d %v %v", path, line, created, err)
	}
	os.WriteFile(path, []byte("my work"), 0o644)
	path2, line2, created2, err := w.Ensure(1900, "A", "cpp", l, "template\n", vars())
	if err != nil || created2 || path2 != path || line2 != 1 {
		t.Fatalf("existing Solution must be reused: %v %v %d", err, created2, line2)
	}
	if b, _ := os.ReadFile(path); string(b) != "my work" {
		t.Fatalf("Solution overwritten: %q", b)
	}
	// a broken template must not leave an empty file behind
	if _, _, _, err := w.Ensure(1, "B", "cpp", l, "{{", vars()); err == nil {
		t.Fatal("want template error")
	}
	if _, err := os.Stat(filepath.Join(w.Root, "1", "B", "main.cpp")); err == nil {
		t.Fatal("failed render left a file")
	}
}

func TestUserTemplateWinsAndInit(t *testing.T) {
	dir := t.TempDir()
	langs := config.Default().Lang
	written, err := InitTemplates(dir, langs, false)
	if err != nil || len(written) != 9 {
		t.Fatalf("%v %v", written, err)
	}
	os.WriteFile(filepath.Join(dir, "cpp.cpp"), []byte("mine {{cursor}}"), 0o644)
	if w, _ := InitTemplates(dir, langs, false); len(w) != 0 {
		t.Fatalf("init must not overwrite: %v", w)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "cpp.cpp")); string(b) != "mine {{cursor}}" {
		t.Fatal("user template clobbered")
	}
	if got, _ := LoadTemplate(dir, "cpp", langs["cpp"]); got != "mine {{cursor}}" {
		t.Fatalf("user template should win: %q", got)
	}
	if w, _ := InitTemplates(dir, langs, true); len(w) != 9 {
		t.Fatalf("--force rewrites all: %v", w)
	}
}

func TestWarning(t *testing.T) {
	if Warning("/mnt/c/Users/x/verd", true) == "" {
		t.Error("WSL + /mnt must warn")
	}
	if Warning("/mnt/c/verd", false) != "" || Warning("/home/x/verd", true) != "" {
		t.Error("only WSL under /mnt warns")
	}
}
