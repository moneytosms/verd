package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetKeepsCommentsAndTypes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Init(path, false); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	for key, val := range map[string]string{"theme": "nord", "autotest": "false", "embed_ratio": "0.5", "handle": "tourist", "split": "tmux"} {
		if err := Set(path, key, val); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "nord" || cfg.Autotest || cfg.EmbedRatio != 0.5 || cfg.Handle != "tourist" || cfg.Split != "tmux" {
		t.Fatalf("%+v", cfg)
	}
	after, _ := os.ReadFile(path)
	if strings.Count(string(after), "\n") != strings.Count(string(before), "\n") {
		t.Fatal("a commented default must be replaced in place, not appended")
	}
	if !strings.Contains(string(after), "# Run tests automatically whenever you save") {
		t.Fatal("comments must survive")
	}
	// second write replaces the live line
	if err := Set(path, "theme", "dracula"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Count(string(b), "theme = ") != 1 || !strings.Contains(string(b), `theme = 'dracula'`) && !strings.Contains(string(b), `theme = "dracula"`) {
		t.Fatalf("theme line:\n%s", b)
	}
}

func TestSetRejectsBadValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for key, val := range map[string]string{"split": "zellij", "autotest": "maybe", "embed_ratio": "2", "float_eps": "x", "lang": "x", "submit_mode": "fax"} {
		if err := Set(path, key, val); err == nil {
			t.Errorf("%s=%s should fail", key, val)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a rejected value must not create the file")
	}
}

func TestSetCreatesMissingFileAndInsertsBeforeTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("[lang.cpp]\ncf_compiler_id = 89\n"), 0o644)
	if err := Set(path, "handle", "me"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Handle != "me" || cfg.Lang["cpp"].CFCompilerID != 89 {
		t.Fatalf("%+v %v", cfg, err)
	}
}
