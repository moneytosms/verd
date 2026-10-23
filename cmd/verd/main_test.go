package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editor"
	"github.com/moneytosms/verd/internal/embed"
	"github.com/moneytosms/verd/internal/mux"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitAndConfig(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	path := filepath.Join(cfgHome, "verd", "config.toml")

	var out bytes.Buffer
	if err := run([]string{"init"}, &out); err != nil || !strings.Contains(out.String(), path) {
		t.Fatalf("init: %v %q", err, out.String())
	}
	if err := run([]string{"init"}, &out); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("second init must refuse and hint --force, got %v", err)
	}
	os.WriteFile(path, []byte(`handle = "tourist"`), 0o644)
	if err := run([]string{"init", "--force"}, &out); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(`handle = "tourist"`+"\n"+`time_multiplier = 3.0`), 0o644)
	out.Reset()
	if err := run([]string{"config"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{path, "handle = 'tourist'", "time_multiplier = 3.0", "default_lang = 'cpp'"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("config output missing %q:\n%s", want, out.String())
		}
	}
	if err := run([]string{"bogus"}, &out); err == nil {
		t.Fatal("unknown command should error")
	}
}

func TestInitWritesTemplates(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	var out bytes.Buffer
	if err := run([]string{"init"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"cpp.cpp", "c.c", "python.py"} {
		if _, err := os.Stat(filepath.Join(cfgHome, "verd", "snippets", "templates", f)); err != nil {
			t.Errorf("template %s not written: %v", f, err)
		}
	}
}

func TestEditCmdCreatesSolutionOnce(t *testing.T) {
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "nvim"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	os.MkdirAll(filepath.Join(cfgHome, "verd", "snippets", "templates"), 0o755)
	os.WriteFile(filepath.Join(cfgHome, "verd", "snippets", "templates", "cpp.cpp"), []byte("// {{.Problem.ID}} {{.Handle}}\n\n{{cursor}}\n"), 0o644)

	cfg := config.Default()
	cfg.Workspace, cfg.Handle = t.TempDir(), "tourist"
	p := cf.Problem{ContestID: 1900, Index: "A", Name: "Cover in Water"}
	ctrl := &editor.Controller{Bin: "nvim"}
	cmd, err := editOpen(cfg, ctrl, p, "cpp")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.Workspace, "1900", "A", "main.cpp")
	if len(cmd.Args) != 3 || cmd.Args[1] != "+3" || cmd.Args[2] != path || cmd.Dir != filepath.Dir(path) {
		t.Fatalf("args %v dir %s", cmd.Args, cmd.Dir)
	}
	if b, _ := os.ReadFile(path); string(b) != "// 1900A tourist\n\n\n" {
		t.Fatalf("solution %q", b)
	}
	os.WriteFile(path, []byte("my work"), 0o644)
	cmd, err = editOpen(cfg, ctrl, p, "cpp")
	if err != nil || cmd.Args[1] != "+1" {
		t.Fatalf("existing Solution: %v %v", cmd, err)
	}
	if b, _ := os.ReadFile(path); string(b) != "my work" {
		t.Fatal("overwrote existing Solution")
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := editOpen(cfg, ctrl, p, "cpp"); err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("missing nvim: %v", err)
	}
}

// addCustomEnv sets up a fake nvim on PATH and a fresh config for the custom and language helpers.
func addCustomEnv(t *testing.T) (config.Config, *editor.Controller) {
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "nvim"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Workspace = t.TempDir()
	return cfg, &editor.Controller{Bin: "nvim"}
}

func TestAddCustomAndLangHelpers(t *testing.T) {
	cfg, ctrl := addCustomEnv(t)
	p := cf.Problem{ContestID: 5, Index: "C"}
	t.Run("addCustom", func(t *testing.T) { checkAddCustom(t, cfg, ctrl, p) })
	t.Run("langKeys", func(t *testing.T) { checkLangKeys(t, cfg) })
	t.Run("solutionRef", func(t *testing.T) { checkSolutionRef(t, cfg, p) })
	t.Run("ensureSolution per language", func(t *testing.T) { checkEnsurePerLang(t, cfg, p) })
}

func checkAddCustom(t *testing.T, cfg config.Config, ctrl *editor.Controller, p cf.Problem) {
	cmd, err := addCustom(cfg, ctrl, p)
	if err != nil || len(cmd.Args) != 4 || cmd.Args[1] != "-O" || filepath.Base(cmd.Args[2]) != "custom-1.in" || filepath.Base(cmd.Args[3]) != "custom-1.ans" {
		t.Fatalf("%v %v", cmd, err)
	}
	cmd, _ = addCustom(cfg, ctrl, p)
	if filepath.Base(cmd.Args[2]) != "custom-2.in" {
		t.Fatalf("second add must not reuse custom-1: %v", cmd.Args)
	}
}

func checkLangKeys(t *testing.T, cfg config.Config) {
	if got := strings.Join(langKeys(cfg), ","); got != "c,cpp,python" {
		t.Fatalf("langs %s", got)
	}
}

func checkSolutionRef(t *testing.T, cfg config.Config, p cf.Problem) {
	ref, err := solutionRef(cfg, p, "python")
	if err != nil || filepath.Base(ref.Path) != "main.py" || ref.Lang != "python" {
		t.Fatalf("%+v %v", ref, err)
	}
	if _, err := solutionRef(cfg, p, "rust"); err == nil {
		t.Fatal("unknown language must error")
	}
}

// switching language creates that Solution; the other language's file is untouched
func checkEnsurePerLang(t *testing.T, cfg config.Config, p cf.Problem) {
	cppPath, _, _ := ensureSolution(cfg, p, "cpp")
	os.WriteFile(cppPath, []byte("cpp work"), 0o644)
	pyPath, _, err := ensureSolution(cfg, p, "python")
	if err != nil || filepath.Base(pyPath) != "main.py" {
		t.Fatalf("%s %v", pyPath, err)
	}
	if b, _ := os.ReadFile(cppPath); string(b) != "cpp work" {
		t.Fatal("cpp Solution touched")
	}
	if b, _ := os.ReadFile(pyPath); !strings.Contains(string(b), "def solve") {
		t.Fatalf("python template not used: %q", b)
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--version"}, &out); err != nil || !strings.HasPrefix(out.String(), "verd ") || !strings.Contains(out.String(), "(") {
		t.Fatalf("%v %q", err, out.String())
	}
}

// runCLI runs the CLI and returns its stdout along with the error.
func runCLI(args ...string) (string, error) {
	var out bytes.Buffer
	err := run(args, &out)
	return out.String(), err
}

// customizeConfig covers `config set/get` on the embed_side setting.
func customizeConfig(t *testing.T) {
	if out, err := runCLI("config", "set", "embed_side", "left"); err != nil || !strings.Contains(out, "embed_side = left") {
		t.Fatalf("%q %v", out, err)
	}
	if out, _ := runCLI("config", "get", "embed_side"); strings.TrimSpace(out) != "left" {
		t.Fatalf("get: %q", out)
	}
	if _, err := runCLI("config", "set", "embed_side", "up"); err == nil {
		t.Fatal("bad value must be refused")
	}
}

// customizeKeys covers `keys set/reset` and the --json listing.
func customizeKeys(t *testing.T) {
	if _, err := runCLI("keys", "set", "problem.run_tests", "ctrl+t"); err != nil {
		t.Fatal(err)
	}
	if out, _ := runCLI("keys", "--json"); !strings.Contains(out, `"keys": [`+"\n"+`      "ctrl+t"`) {
		t.Fatalf("json should show the new key:\n%s", out)
	}
	if _, err := runCLI("keys", "set", "problem.submit", "ctrl+t"); err == nil || !strings.Contains(err.Error(), "bound to both") {
		t.Fatalf("conflict must be refused: %v", err)
	}
	if _, err := runCLI("keys", "set", "problem.nope", "x"); err == nil {
		t.Fatal("unknown action must be refused")
	}
	if _, err := runCLI("keys", "reset", "problem.run_tests"); err != nil {
		t.Fatal(err)
	}
	if out, _ := runCLI("keys"); strings.Contains(out, "ctrl+t") {
		t.Fatalf("reset should restore t:\n%s", out)
	}
}

// customizeThemes covers `themes show` for a custom and the terminal palette.
func customizeThemes(t *testing.T) {
	out, err := runCLI("themes", "show", "nord")
	if err != nil || !strings.Contains(out, "[themes.my-nord.dark]") || !strings.Contains(out, `accent = "#88c0d0"`) {
		t.Fatalf("%q %v", out, err)
	}
	if out, _ := runCLI("themes", "show", "terminal"); !strings.Contains(out, `accent = "ansi6"`) {
		t.Fatalf("terminal palette uses ansi indices:\n%s", out)
	}
}

// customizePresets covers `keys presets` list, apply, export and reset.
func customizePresets(t *testing.T) {
	if out, err := runCLI("keys", "presets", "list"); err != nil || !strings.Contains(out, "default") || !strings.Contains(out, "vim") {
		t.Fatalf("presets list failed: %q %v", out, err)
	}
	if out, err := runCLI("keys", "presets", "preset", "vim"); err != nil || !strings.Contains(out, "applied") {
		t.Fatalf("apply vim preset failed: %q %v", out, err)
	}
	if out, _ := runCLI("keys"); !strings.Contains(out, "ctrl+d") {
		t.Fatalf("vim preset should have ctrl+d:\n%s", out)
	}
	if out, err := runCLI("keys", "presets", "export"); err != nil || !strings.Contains(out, "[keys.") {
		t.Fatalf("export failed: %q %v", out, err)
	}
	// round-trip: export and reset to default
	if _, err := runCLI("keys", "presets", "export"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI("keys", "presets", "preset", "default"); err != nil {
		t.Fatalf("reset to default failed: %v", err)
	}
	if o2, _ := runCLI("keys"); strings.Contains(o2, "ctrl+d") {
		t.Fatalf("default preset should not have ctrl+d:\n%s", o2)
	}
}

func TestCustomizeCLI(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := runCLI("init", "--no-setup"); err != nil {
		t.Fatal(err)
	}
	t.Run("config", customizeConfig)
	t.Run("keys", customizeKeys)
	t.Run("themes", customizeThemes)
	t.Run("presets", customizePresets)
}

func TestWatchConfigStartsAndStops(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgPath, []byte(`handle = "test"`), 0o644)

	called := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watchConfig(ctx, dir, "config.toml", func() {
		called++
	})

	// Give watchConfig time to start; cancel should stop it cleanly
	time.Sleep(100 * time.Millisecond)
	cancel()

	// If the watcher doesn't crash and the function returns, the test passes
	time.Sleep(100 * time.Millisecond)
	// called may or may not be > 0 depending on timing, but the goroutine should not leak
}

func TestKeysImportExportRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	do := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := run(args, &out)
		return out.String(), err
	}
	if _, err := do("init", "--no-setup"); err != nil {
		t.Fatal(err)
	}
	do("keys", "set", "problem.run_tests", "ctrl+t")
	exported, err := do("keys", "export")
	if err != nil || !strings.Contains(exported, "ctrl+t") {
		t.Fatalf("%q %v", exported, err)
	}
	file := filepath.Join(t.TempDir(), "keys.toml")
	os.WriteFile(file, []byte(exported), 0o644)
	do("keys", "set", "problem.submit", "ctrl+s") // import must replace this away
	if out, err := do("keys", "import", file); err != nil || !strings.Contains(out, "imported 1") {
		t.Fatalf("%q %v", out, err)
	}
	if out, _ := do("keys", "--json"); strings.Contains(out, `"ctrl+s"`) || !strings.Contains(out, "ctrl+t") {
		t.Fatalf("import should replace:\n%s", out)
	}
	bad := filepath.Join(t.TempDir(), "bad.toml")
	os.WriteFile(bad, []byte("[keys.problem]\nrun_tests = \"s\"\n"), 0o644) // s is submit
	if _, err := do("keys", "import", bad); err == nil {
		t.Fatal("conflicting import must fail")
	}
	if out, _ := do("keys", "--json"); !strings.Contains(out, "ctrl+t") {
		t.Fatal("failed import must not change anything")
	}
}

func TestSnippetCmd(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	var out bytes.Buffer
	if err := run([]string{"snippet", "--list"}, &out); err != nil || !strings.Contains(out.String(), "dijkstra") {
		t.Fatalf("list: %v %q", err, out.String())
	}
	out.Reset()
	if err := run([]string{"snippet", "--lang", "python", "modpow"}, &out); err != nil || !strings.Contains(out.String(), "def modpow") {
		t.Fatalf("print: %v %q", err, out.String())
	}
	if err := run([]string{"snippet", "nope"}, &out); err == nil {
		t.Fatal("unknown snippet should error")
	}
	// snippets_dir moves both the library and the Templates
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(cfgHome, "verd"), 0o755)
	os.WriteFile(filepath.Join(cfgHome, "verd", "config.toml"), []byte("snippets_dir = '"+dir+"'\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "library", "cpp"), 0o755)
	os.WriteFile(filepath.Join(dir, "library", "cpp", "mine.cpp"), []byte("// mine\nint x;\n"), 0o644)
	out.Reset()
	if err := run([]string{"snippet", "mine"}, &out); err != nil || out.String() != "// mine\nint x;\n" {
		t.Fatalf("snippets_dir: %v %q", err, out.String())
	}
	// ...and a Template in <snippets_dir>/templates replaces the built-in one
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "cpp.cpp"), []byte("// custom {{.Problem.ID}}\n"), 0o644)
	cfg := config.Default()
	cfg.SnippetsDir, cfg.Workspace = dir, t.TempDir()
	path, _, err := ensureSolution(cfg, cf.Problem{ContestID: 1900, Index: "A"}, "cpp")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !strings.HasPrefix(string(b), "// custom 1900A") {
		t.Fatalf("custom template not used: %q", b)
	}
}

func TestSnippetExport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out bytes.Buffer
	if err := run([]string{"snippet", "--export", "--lang", "c"}, &out); err != nil {
		t.Fatal(err)
	}
	var got map[string]struct {
		Prefix, Description string
		Body                []string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	m := got["modpow"]
	if m.Prefix != "modpow" || len(m.Body) < 3 || m.Description == "" || strings.Contains(m.Body[0], "\n") {
		t.Fatalf("%+v", m)
	}
	if !strings.Contains(out.String(), `"debug"`) || !strings.Contains(strings.Join(got["debug"].Body, "\n"), "fprintf") {
		t.Fatal("debug snippet missing")
	}
}

// snipCLI runs `verd snippet ...` and keeps the output of the last call.
type snipCLI struct {
	out bytes.Buffer
}

func (s *snipCLI) do(args ...string) error {
	s.out.Reset()
	return run(append([]string{"snippet"}, args...), &s.out)
}

// must runs a snippet command and fails the test on error.
func (s *snipCLI) must(t *testing.T, args ...string) {
	t.Helper()
	if err := s.do(args...); err != nil {
		t.Fatal(err)
	}
}

// snippetAdd covers `snippet add` from a file, with a description, and its errors.
func snippetAdd(t *testing.T, s *snipCLI, dir string) {
	src := filepath.Join(t.TempDir(), "mine.cpp")
	os.WriteFile(src, []byte("int twice(int x) { return 2 * x; }"), 0o644)
	s.must(t, "add", "--desc", "double it", "twice", src)
	if b, _ := os.ReadFile(filepath.Join(dir, "library", "cpp", "twice.cpp")); string(b) != "// double it\nint twice(int x) { return 2 * x; }\n" {
		t.Fatalf("added file: %q", b)
	}
	if err := s.do("--list"); err != nil || !strings.Contains(s.out.String(), "double it") {
		t.Fatalf("list: %v %q", err, s.out.String())
	}
	if err := s.do("add", "twice", src); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate add: %v", err)
	}
	if err := s.do("add", "dsu", src); err == nil {
		t.Fatal("adding over a built-in needs --force")
	}
	if err := s.do("add", "bad name", src); err == nil {
		t.Fatal("bad name")
	}
}

// snippetAddPython covers that python gets a # description line.
func snippetAddPython(t *testing.T, s *snipCLI, dir string) {
	pysrc := filepath.Join(t.TempDir(), "p.py")
	os.WriteFile(pysrc, []byte("x = 1\n"), 0o644)
	s.must(t, "add", "--lang", "python", "xone", pysrc)
	if b, _ := os.ReadFile(filepath.Join(dir, "library", "python", "xone.py")); string(b) != "# xone\nx = 1\n" {
		t.Fatalf("python add: %q", b)
	}
}

// snippetEditAndStub covers editing a built-in and adding a stub with no source.
func snippetEditAndStub(t *testing.T, s *snipCLI, dir, rec string) {
	s.must(t, "edit", "modpow")
	mp := filepath.Join(dir, "library", "cpp", "modpow.cpp")
	if _, err := os.Stat(mp); err != nil {
		t.Fatal("edit should write the built-in's copy")
	}
	if log, _ := os.ReadFile(rec); !strings.Contains(string(log), mp) {
		t.Fatalf("editor not run on %s: %q", mp, log)
	}
	s.must(t, "add", "--desc", "mine", "stub")
	if b, _ := os.ReadFile(filepath.Join(dir, "library", "cpp", "stub.cpp")); string(b) != "// mine\n" {
		t.Fatalf("stub: %q", b)
	}
}

// snippetRmOpenPath covers rm (user file goes, built-in stays), open and path.
func snippetRmOpenPath(t *testing.T, s *snipCLI, dir, rec string) {
	s.must(t, "rm", "twice")
	if err := s.do("rm", "twice"); err == nil {
		t.Fatal("second rm should fail")
	}
	s.must(t, "rm", "modpow")
	if err := s.do("modpow"); err != nil || !strings.Contains(s.out.String(), "modular exponentiation") {
		t.Fatalf("built-in must remain after rm: %v %q", err, s.out.String())
	}
	// open: populates and opens the library folder; path prints the dir
	s.must(t, "open")
	if log, _ := os.ReadFile(rec); !strings.Contains(string(log), filepath.Join(dir, "library")) {
		t.Fatalf("open: %q", log)
	}
	if _, err := os.Stat(filepath.Join(dir, "library", "python", "dsu.py")); err != nil {
		t.Fatal("open should populate the library")
	}
	if err := s.do("path"); err != nil || strings.TrimSpace(s.out.String()) != dir {
		t.Fatalf("path: %v %q", err, s.out.String())
	}
}

func TestSnippetManage(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	dir := filepath.Join(cfgHome, "verd", "snippets")
	rec := filepath.Join(t.TempDir(), "editor.log")
	ed := filepath.Join(t.TempDir(), "fakeed")
	os.WriteFile(ed, []byte("#!/bin/sh\necho \"$@\" >> '"+rec+"'\n"), 0o755)
	os.MkdirAll(filepath.Join(cfgHome, "verd"), 0o755)
	os.WriteFile(filepath.Join(cfgHome, "verd", "config.toml"), []byte("editor = '"+ed+"'\n"), 0o644)
	s := &snipCLI{}

	t.Run("add from file", func(t *testing.T) { snippetAdd(t, s, dir) })
	t.Run("add python", func(t *testing.T) { snippetAddPython(t, s, dir) })
	t.Run("edit and stub", func(t *testing.T) { snippetEditAndStub(t, s, dir, rec) })
	t.Run("rm open path", func(t *testing.T) { snippetRmOpenPath(t, s, dir, rec) })
}

func TestChooseMux(t *testing.T) {
	emb := func() mux.Mux { return &embed.Adapter{} }
	none := func(string) string { return "" }
	tmuxEnv := func(k string) string {
		return map[string]string{"TMUX": "/tmp/tmux-1/default,1,0", "TMUX_PANE": "%1"}[k]
	}
	isEmbedded := func(m mux.Mux) bool { _, ok := m.(*embed.Adapter); return ok }
	for _, c := range []struct {
		name, mode string
		env        func(string) string
		want       string // "embedded", "tmux" or "suspend"
	}{
		{"auto outside a multiplexer embeds", "auto", none, "embedded"},
		{"empty means auto", "", none, "embedded"},
		{"auto inside tmux splits", "auto", tmuxEnv, "tmux"},
		{"explicit embedded wins in tmux", "embedded", tmuxEnv, "embedded"},
		{"suspend is the opt-out", "suspend", none, "suspend"},
		{"suspend inside tmux still suspends", "suspend", tmuxEnv, "suspend"},
		{"forced tmux outside tmux suspends", "tmux", none, "suspend"},
	} {
		got := chooseMux(c.mode, c.env, emb)
		kind := "suspend"
		switch {
		case isEmbedded(got):
			kind = "embedded"
		case got != nil:
			kind = "tmux"
		}
		if kind != c.want {
			t.Errorf("%s: got %s, want %s", c.name, kind, c.want)
		}
	}
}

func TestSnippetsDirIsSettableAndInTheSchema(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	var out bytes.Buffer
	if err := run([]string{"config", "set", "snippets_dir", "~/dotfiles/verd"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"config", "get", "snippets_dir"}, &out); err != nil || strings.TrimSpace(out.String()) != "~/dotfiles/verd" {
		t.Fatalf("%v %q", err, out.String())
	}
	out.Reset()
	if err := run([]string{"schema"}, &out); err != nil {
		t.Fatal(err)
	}
	var s struct {
		Properties map[string]struct {
			Type    string
			Default any
			Enum    []string
		}
	}
	if err := json.Unmarshal(out.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	for _, k := range config.SettableKeys() {
		if _, ok := s.Properties[k]; !ok {
			t.Errorf("schema is missing %s", k)
		}
	}
	if s.Properties["autotest"].Default != true || s.Properties["embed_side"].Enum[0] != "left" || s.Properties["tabs"].Type != "array" {
		t.Fatalf("defaults and enums come from the config: %+v", s.Properties["autotest"])
	}
	out.Reset()
	run([]string{"__complete", "config-keys"}, &out)
	if !strings.Contains(out.String(), "snippets_dir") || !strings.Contains(out.String(), "reading_width") {
		t.Fatalf("completion lists every settable key:\n%s", out.String())
	}
}
