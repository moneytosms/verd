package main

import (
	"bytes"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editor"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if _, err := editOpen(cfg, ctrl, p, "cpp"); err == nil || !strings.Contains(err.Error(), "nvim not found") {
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
