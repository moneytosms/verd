package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil || c.DefaultLang != "cpp" || c.Handle != "" {
		t.Fatalf("missing file should give defaults, got %+v %v", c, err)
	}
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte(`handle = "tourist"`+"\n"+`time_multiplier = 2.0`), 0o644)
	c, err = Load(p)
	if err != nil || c.Handle != "tourist" || c.TimeMultiplier != 2 || c.DefaultLang != "cpp" {
		t.Fatalf("got %+v %v", c, err)
	}
}

func TestInitNoOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "verd", "config.toml")
	if err := Init(p, false); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(p); err != nil || !reflect.DeepEqual(c, Default()) {
		t.Fatalf("fresh init must load as defaults: %+v %v", c, err)
	}
	os.WriteFile(p, []byte(`handle = "mine"`), 0o644)
	if err := Init(p, false); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != `handle = "mine"` {
		t.Fatal("existing config was modified")
	}
	if err := Init(p, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) == `handle = "mine"` {
		t.Fatal("--force should overwrite")
	}
}

func TestEffectiveReflectsOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte(`handle = "tourist"`+"\n"+`theme = "dracula"`), 0o644)
	c, _ := Load(p)
	b, err := c.Effective()
	out := string(b)
	for _, want := range []string{`handle = 'tourist'`, `theme = 'dracula'`, `default_lang = 'cpp'`, `time_multiplier = 1.0`} {
		if err != nil || !strings.Contains(out, want) {
			t.Errorf("effective config missing %q:\n%s", want, out)
		}
	}
}

func TestXDGDirs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/cfg")
	t.Setenv("XDG_DATA_HOME", "/x/data")
	if Dir() != "/x/cfg/verd" || DataDir() != "/x/data/verd" {
		t.Fatalf("%s %s", Dir(), DataDir())
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/u")
	if Dir() != "/home/u/.config/verd" || DataDir() != "/home/u/.local/share/verd" {
		t.Fatalf("fallbacks: %s %s", Dir(), DataDir())
	}
}

func TestLangMergeInheritsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte("[lang.cpp]\ncompile = [\"clang++\", \"-o\", \"{bin}\", \"{src}\"]\n\n[lang.rust]\next = \"rs\"\nrun = [\"x\"]\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	cpp := c.Lang["cpp"]
	if cpp.Compile[0] != "clang++" || cpp.Ext != "cpp" || cpp.CFCompilerID != 89 || len(cpp.Run) != 1 {
		t.Fatalf("partial override must keep ext/id and keep default run: %+v", cpp)
	}
	if c.Lang["python"].Run[0] != "python3" || c.Lang["rust"].Ext != "rs" {
		t.Fatalf("defaults and new langs: %+v", c.Lang)
	}
}
