package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func uninstallFixture(t *testing.T) uninstallPaths {
	t.Helper()
	root := t.TempDir()
	p := uninstallPaths{exe: filepath.Join(root, "bin", "verd"), config: filepath.Join(root, "c", "verd"), data: filepath.Join(root, "d", "verd"), cache: filepath.Join(root, "x", "verd"), workspace: filepath.Join(root, "ws")}
	for _, f := range []string{p.exe, filepath.Join(p.config, "config.toml"), filepath.Join(p.data, "verd.db"), filepath.Join(p.cache, "build", "a"), filepath.Join(p.workspace, "1", "A", "main.cpp")} {
		os.MkdirAll(filepath.Dir(f), 0o755)
		os.WriteFile(f, []byte("x"), 0o644)
	}
	return p
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestUninstallAll(t *testing.T) {
	p := uninstallFixture(t)
	var out bytes.Buffer
	if err := uninstallCmd(&out, strings.NewReader(""), []string{"--all", "--yes"}, p); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{p.exe, p.config, p.data, p.cache} {
		if exists(f) {
			t.Errorf("%s still there", f)
		}
	}
	if !exists(filepath.Join(p.workspace, "1", "A", "main.cpp")) {
		t.Error("workspace touched")
	}
}

func TestUninstallKeepAndPrompt(t *testing.T) {
	p := uninstallFixture(t)
	var out bytes.Buffer
	if err := uninstallCmd(&out, strings.NewReader("3\ny\n"), nil, p); err != nil {
		t.Fatal(err)
	}
	if exists(p.exe) || !exists(p.config) || !exists(p.data) {
		t.Error("keep mode should remove only the binary")
	}
}

func TestUninstallDeclineAndBackup(t *testing.T) {
	p := uninstallFixture(t)
	var out bytes.Buffer
	if err := uninstallCmd(&out, strings.NewReader("n\n"), []string{"--all"}, p); exitCode(err) != 1 || !exists(p.exe) {
		t.Fatalf("decline should cancel: %v", err)
	}
	dir := t.TempDir()
	if err := uninstallCmd(&out, strings.NewReader(""), []string{"--backup", dir, "--yes"}, p); err != nil {
		t.Fatal(err)
	}
	m, _ := filepath.Glob(filepath.Join(dir, "verd-backup-*.tar.gz"))
	if len(m) != 1 || exists(p.config) {
		t.Fatalf("backup %v, config gone=%v", m, !exists(p.config))
	}
}

func TestUninstallRefusesForeignDir(t *testing.T) {
	p := uninstallFixture(t)
	p.config = t.TempDir()
	if err := uninstallCmd(&bytes.Buffer{}, strings.NewReader(""), []string{"--all", "--yes"}, p); exitCode(err) != 2 {
		t.Fatalf("got %v", err)
	}
}
