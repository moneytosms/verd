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

func TestSubmitModeDefaultAndValidation(t *testing.T) {
	if Default().SubmitMode != "browser" {
		t.Fatal("browser must be the default")
	}
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte(`submit_mode = "direct"`), 0o644)
	if c, err := Load(p); err != nil || c.SubmitMode != "direct" {
		t.Fatalf("%+v %v", c, err)
	}
	os.WriteFile(p, []byte(`submit_mode = "magic"`), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("unknown submit_mode must error")
	}
}

func TestMouseOptionsDefaults(t *testing.T) {
	d := Default()
	if !d.Mouse || d.WheelLines != 3 || !d.MouseSelect {
		t.Fatalf("mouse defaults: mouse=%v wheel_lines=%v mouse_select=%v", d.Mouse, d.WheelLines, d.MouseSelect)
	}
}

func TestMouseOptionsValidation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	// Valid wheel_lines
	os.WriteFile(p, []byte(`wheel_lines = 5`), 0o644)
	if c, err := Load(p); err != nil || c.WheelLines != 5 {
		t.Fatalf("valid wheel_lines: %+v %v", c, err)
	}
	// Invalid wheel_lines (too low)
	os.WriteFile(p, []byte(`wheel_lines = 0`), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("wheel_lines=0 must error")
	}
	// Invalid wheel_lines (too high)
	os.WriteFile(p, []byte(`wheel_lines = 21`), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("wheel_lines=21 must error")
	}
	// Valid mouse flags
	os.WriteFile(p, []byte(`mouse = false`+"\n"+`mouse_select = false`), 0o644)
	if c, err := Load(p); err != nil || c.Mouse || c.MouseSelect {
		t.Fatalf("mouse false: %+v %v", c, err)
	}
}

func TestPerLanguageOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte("[lang.python]\ntime_multiplier = 2.5\nfloat_eps = 1e-7\nmemory_multiplier = 1.5\n\n[lang.rust]\ntemplate = \"/path/to/template.rs\"\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	py := c.Lang["python"]
	if py.TimeMultiplier != 2.5 || py.FloatEps != 1e-7 || py.MemoryMultiplier != 1.5 {
		t.Fatalf("python overrides: %+v", py)
	}
	rs := c.Lang["rust"]
	if rs.Template != "/path/to/template.rs" {
		t.Fatalf("rust template: %+v", rs)
	}
	// Verify defaults are preserved for other fields
	if py.Ext != "py" || py.Run[0] != "python3" {
		t.Fatalf("python should inherit defaults: %+v", py)
	}
}

func TestPerLanguageInheritance(t *testing.T) {
	// 0 values should inherit from global config
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte("[lang.cpp]\ntime_multiplier = 0\nfloat_eps = 0\nmemory_multiplier = 0\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	cpp := c.Lang["cpp"]
	// When 0, they should inherit from defaults (even though Load() doesn't explicitly set them,
	// they were already set to defaults when Lang struct was created from default values)
	// After Load(), time_multiplier=0 should be set to the default
	if cpp.TimeMultiplier == 0 {
		t.Fatalf("cpp should inherit global time_multiplier, got: %+v", cpp)
	}
}

func TestEffectiveConfigIncludesMouseAndLanguageOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte(`handle = "test"`+"\n"+`mouse = false`+"\n"+`wheel_lines = 5`+"\n"+`[lang.python]`+"\n"+`time_multiplier = 2.0`), 0o644)
	c, _ := Load(p)
	b, _ := c.Effective()
	out := string(b)
	if !strings.Contains(out, `mouse = false`) || !strings.Contains(out, `wheel_lines = 5`) {
		t.Errorf("effective config missing mouse options:\n%s", out)
	}
	if !strings.Contains(out, `time_multiplier = 2`) {
		t.Errorf("effective config missing per-language override:\n%s", out)
	}
}
