// Package config loads verd's TOML config with embedded defaults.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Handle         string  `toml:"handle"`
	Workspace      string  `toml:"workspace"`
	DefaultLang    string  `toml:"default_lang"`
	TimeMultiplier float64 `toml:"time_multiplier"`
	Theme          string  `toml:"theme"`
	Split          string  `toml:"split"`

	Lang map[string]Lang `toml:"lang"`
}

// Lang describes how to build and run one language. Placeholders: {src}, {bin}, {dir}.
// An empty Compile means interpreted: Run is used directly on {src}.
type Lang struct {
	Ext          string   `toml:"ext"`
	Compile      []string `toml:"compile"`
	Run          []string `toml:"run"`
	CFCompilerID int      `toml:"cf_compiler_id"`
}

//go:embed default.toml
var defaultTOML []byte

// ErrExists is returned by Init when the config file is already there.
var ErrExists = errors.New("config already exists")

// Init writes the commented default config to path, creating parent dirs.
// It never overwrites an existing file unless force is set.
func Init(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%w: %s (use --force to overwrite)", ErrExists, path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultTOML, 0o644)
}

// Effective renders the merged config as TOML.
func (c Config) Effective() ([]byte, error) { return toml.Marshal(c) }

// Defaults mirror the Codeforces compilers.
func Default() Config {
	return Config{
		Workspace: "~/verd", DefaultLang: "cpp", TimeMultiplier: 1.0, Theme: "terminal", Split: "auto",
		Lang: map[string]Lang{
			"c":      {Ext: "c", Compile: []string{"gcc", "-std=c11", "-O2", "-Wall", "-o", "{bin}", "{src}", "-lm"}, Run: []string{"{bin}"}, CFCompilerID: 43},
			"cpp":    {Ext: "cpp", Compile: []string{"g++", "-std=c++20", "-O2", "-Wall", "-o", "{bin}", "{src}"}, Run: []string{"{bin}"}, CFCompilerID: 89},
			"python": {Ext: "py", Run: []string{"python3", "{src}"}, CFCompilerID: 31},
		},
	}
}

// Dir returns $XDG_CONFIG_HOME/verd, falling back to ~/.config/verd on every OS.
func Dir() string { return xdg("XDG_CONFIG_HOME", ".config") }

// DataDir returns $XDG_DATA_HOME/verd, falling back to ~/.local/share/verd.
func DataDir() string { return xdg("XDG_DATA_HOME", filepath.Join(".local", "share")) }

// CacheDir returns $XDG_CACHE_HOME/verd, falling back to ~/.cache/verd.
func CacheDir() string { return xdg("XDG_CACHE_HOME", ".cache") }

func xdg(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return filepath.Join(d, "verd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback, "verd")
}

// Load reads path over defaults. A missing file yields defaults.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	// A user's [lang.x] table replaces the default wholesale: re-inherit what they left out.
	def := Default().Lang
	for k, l := range c.Lang {
		d, ok := def[k]
		if !ok {
			continue
		}
		if l.Ext == "" {
			l.Ext = d.Ext
		}
		if l.CFCompilerID == 0 {
			l.CFCompilerID = d.CFCompilerID
		}
		if len(l.Compile) == 0 && len(l.Run) == 0 {
			l.Compile, l.Run = d.Compile, d.Run
		}
		c.Lang[k] = l
	}
	return c, nil
}
