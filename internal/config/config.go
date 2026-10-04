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

func Default() Config {
	return Config{Workspace: "~/verd", DefaultLang: "cpp", TimeMultiplier: 1.0, Theme: "terminal", Split: "auto"}
}

// Dir returns $XDG_CONFIG_HOME/verd, falling back to ~/.config/verd on every OS.
func Dir() string { return xdg("XDG_CONFIG_HOME", ".config") }

// DataDir returns $XDG_DATA_HOME/verd, falling back to ~/.local/share/verd.
func DataDir() string { return xdg("XDG_DATA_HOME", filepath.Join(".local", "share")) }

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
	return c, toml.Unmarshal(b, &c)
}
