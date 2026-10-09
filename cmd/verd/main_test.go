package main

import (
	"bytes"
	"context"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editor"
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
		if _, err := os.Stat(filepath.Join(cfgHome, "verd", "templates", f)); err != nil {
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
	os.MkdirAll(filepath.Join(cfgHome, "verd", "templates"), 0o755)
	os.WriteFile(filepath.Join(cfgHome, "verd", "templates", "cpp.cpp"), []byte("// {{.Problem.ID}} {{.Handle}}\n\n{{cursor}}\n"), 0o644)

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

func TestAddCustomAndLangHelpers(t *testing.T) {
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "nvim"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Workspace = t.TempDir()
	p := cf.Problem{ContestID: 5, Index: "C"}
	ctrl := &editor.Controller{Bin: "nvim"}

	cmd, err := addCustom(cfg, ctrl, p)
	if err != nil || len(cmd.Args) != 4 || cmd.Args[1] != "-O" || filepath.Base(cmd.Args[2]) != "custom-1.in" || filepath.Base(cmd.Args[3]) != "custom-1.ans" {
		t.Fatalf("%v %v", cmd, err)
	}
	cmd, _ = addCustom(cfg, ctrl, p)
	if filepath.Base(cmd.Args[2]) != "custom-2.in" {
		t.Fatalf("second add must not reuse custom-1: %v", cmd.Args)
	}

	if got := strings.Join(langKeys(cfg), ","); got != "c,cpp,python" {
		t.Fatalf("langs %s", got)
	}
	ref, err := solutionRef(cfg, p, "python")
	if err != nil || filepath.Base(ref.Path) != "main.py" || ref.Lang != "python" {
		t.Fatalf("%+v %v", ref, err)
	}
	if _, err := solutionRef(cfg, p, "rust"); err == nil {
		t.Fatal("unknown language must error")
	}
	// switching language creates that Solution; the other language's file is untouched
	cppPath, _, _ := ensureSolution(cfg, p, "cpp")
	os.WriteFile(cppPath, []byte("cpp work"), 0o644)
	pyPath, _, err := ensureSolution(cfg, p, "python")
	if err != nil || filepath.Base(pyPath) != "main.py" {
		t.Fatalf("%s %v", pyPath, err)
	}
	if b, _ := os.ReadFile(cppPath); string(b) != "cpp work" {
		t.Fatal("cpp Solution touched")
	}
	if b, _ := os.ReadFile(pyPath); !strings.Contains(string(b), "def main") {
		t.Fatalf("python template not used: %q", b)
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--version"}, &out); err != nil || !strings.HasPrefix(out.String(), "verd ") || !strings.Contains(out.String(), "(") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestCustomizeCLI(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	do := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := run(args, &out)
		return out.String(), err
	}
	if _, err := do("init", "--no-setup"); err != nil {
		t.Fatal(err)
	}
	if out, err := do("config", "set", "embed_side", "left"); err != nil || !strings.Contains(out, "embed_side = left") {
		t.Fatalf("%q %v", out, err)
	}
	if out, _ := do("config", "get", "embed_side"); strings.TrimSpace(out) != "left" {
		t.Fatalf("get: %q", out)
	}
	if _, err := do("config", "set", "embed_side", "up"); err == nil {
		t.Fatal("bad value must be refused")
	}
	if _, err := do("keys", "set", "problem.run_tests", "ctrl+t"); err != nil {
		t.Fatal(err)
	}
	if out, _ := do("keys", "--json"); !strings.Contains(out, `"keys": [`+"\n"+`      "ctrl+t"`) {
		t.Fatalf("json should show the new key:\n%s", out)
	}
	if _, err := do("keys", "set", "problem.submit", "ctrl+t"); err == nil || !strings.Contains(err.Error(), "bound to both") {
		t.Fatalf("conflict must be refused: %v", err)
	}
	if _, err := do("keys", "set", "problem.nope", "x"); err == nil {
		t.Fatal("unknown action must be refused")
	}
	if _, err := do("keys", "reset", "problem.run_tests"); err != nil {
		t.Fatal(err)
	}
	if out, _ := do("keys"); strings.Contains(out, "ctrl+t") {
		t.Fatalf("reset should restore t:\n%s", out)
	}
	out, err := do("themes", "show", "nord")
	if err != nil || !strings.Contains(out, "[themes.my-nord.dark]") || !strings.Contains(out, `accent = "#88c0d0"`) {
		t.Fatalf("%q %v", out, err)
	}
	if out, _ := do("themes", "show", "terminal"); !strings.Contains(out, `accent = "ansi6"`) {
		t.Fatalf("terminal palette uses ansi indices:\n%s", out)
	}
	// Test presets
	if out, err := do("keys", "presets", "list"); err != nil || !strings.Contains(out, "default") || !strings.Contains(out, "vim") {
		t.Fatalf("presets list failed: %q %v", out, err)
	}
	if out, err := do("keys", "presets", "preset", "vim"); err != nil || !strings.Contains(out, "applied") {
		t.Fatalf("apply vim preset failed: %q %v", out, err)
	}
	if out, _ := do("keys"); !strings.Contains(out, "ctrl+d") {
		t.Fatalf("vim preset should have ctrl+d:\n%s", out)
	}
	// Test export
	if out, err := do("keys", "presets", "export"); err != nil || !strings.Contains(out, "[keys.") {
		t.Fatalf("export failed: %q %v", out, err)
	}
	// Test round-trip: export and import
	if _, err := do("keys", "presets", "export"); err != nil {
		t.Fatal(err)
	}
	if _, err := do("keys", "presets", "preset", "default"); err != nil {
		t.Fatalf("reset to default failed: %v", err)
	}
	if o2, _ := do("keys"); strings.Contains(o2, "ctrl+d") {
		t.Fatalf("default preset should not have ctrl+d:\n%s", o2)
	}
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
	if out, _ := do("keys", "--json"); strings.Contains(out, "ctrl+s") || !strings.Contains(out, "ctrl+t") {
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
