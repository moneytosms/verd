// Package config loads verd's TOML config with embedded defaults.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/moneytosms/verd/internal/theme"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Handle          string  `toml:"handle"`
	Workspace       string  `toml:"workspace"`
	DefaultLang     string  `toml:"default_lang"`
	TimeMultiplier  float64 `toml:"time_multiplier"`
	FloatEps        float64 `toml:"float_eps"`
	Autotest        bool    `toml:"autotest"`
	Theme           string  `toml:"theme"`
	Background      string  `toml:"background"`
	SourceCF        bool    `toml:"source_cf"`
	SourceCSES      bool    `toml:"source_cses"`
	Editor          string  `toml:"editor"`
	Split           string  `toml:"split"`
	EmbedRatio      float64 `toml:"embed_ratio"`
	EmbedFocusKey   string  `toml:"embed_focus_key"`
	EmbedSide       string  `toml:"embed_side"`
	SubmitMode      string  `toml:"submit_mode"`
	ReadingWidth    int     `toml:"reading_width"`
	ReadingMargin   int     `toml:"reading_margin"`
	ReadingSpacing  string  `toml:"reading_spacing"`
	ReadingHeadings string  `toml:"reading_headings"`
	ReadingMath     string  `toml:"reading_math"`
	ReadingEmphasis bool    `toml:"reading_emphasis"`
	Border          string  `toml:"border"`
	Mouse           bool    `toml:"mouse"`
	WheelLines      int     `toml:"wheel_lines"`
	MouseSelect     bool    `toml:"mouse_select"`

	// Keys rebinds shortcuts: [keys.<context>] action = "key" or ["key", ...]. See KeyBindings.
	Keys map[string]map[string]any `toml:"keys"`

	// Themes defines named color themes: [themes.<name>] base/glamour_*/[dark]/[light]. See theme.Spec.
	Themes map[string]theme.Spec `toml:"themes"`

	Lang map[string]Lang `toml:"lang"`
}

// Lang describes how to build and run one language. Placeholders: {src}, {bin}, {dir}.
// An empty Compile means interpreted: Run is used directly on {src}.
type Lang struct {
	Ext              string   `toml:"ext"`
	Compile          []string `toml:"compile"`
	Run              []string `toml:"run"`
	CFCompilerID     int      `toml:"cf_compiler_id"`
	TimeMultiplier   float64  `toml:"time_multiplier"`
	FloatEps         float64  `toml:"float_eps"`
	MemoryMultiplier float64  `toml:"memory_multiplier"`
	Template         string   `toml:"template"`
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
		Workspace: "~/verd", DefaultLang: "cpp", TimeMultiplier: 1.0, FloatEps: 1e-6, Autotest: true, Theme: "terminal", Background: "auto", SourceCF: true, Editor: "nvim", Split: "auto", EmbedRatio: 0.4, EmbedFocusKey: "ctrl+\\", EmbedSide: "right", SubmitMode: "browser",
		ReadingMargin: 1, ReadingSpacing: "normal", ReadingHeadings: "bar", ReadingMath: "unicode", ReadingEmphasis: true, Border: "rounded",
		Mouse: true, WheelLines: 3, MouseSelect: true,
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

// RuntimeDir holds sockets: $XDG_RUNTIME_DIR/verd, else /tmp/verd-<uid>. Unix socket paths are
// limited to about 104 bytes (macOS), so a long $XDG_RUNTIME_DIR falls back to /tmp too.
func RuntimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" && len(d) < 60 {
		return filepath.Join(d, "verd")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("verd-%d", os.Getuid()))
}

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
	// Track which Lang entries were in the TOML before unmarshaling.
	var userLangs map[string]bool
	var raw map[string]interface{}
	if err := toml.Unmarshal(b, &raw); err == nil {
		if langMap, ok := raw["lang"].(map[string]interface{}); ok {
			userLangs = make(map[string]bool)
			for k := range langMap {
				userLangs[k] = true
			}
		}
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Background != "auto" && c.Background != "dark" && c.Background != "light" {
		return c, fmt.Errorf("background %q: want auto, dark or light", c.Background)
	}
	for name, spec := range c.Themes {
		if err := theme.Register(name, spec); err != nil {
			return c, fmt.Errorf("themes.%s: %w", name, err)
		}
	}
	if !slices.Contains([]string{"rounded", "square", "heavy", "double", "ascii", "none"}, c.Border) {
		return c, fmt.Errorf("border %q: want rounded, square, heavy, double, ascii or none", c.Border)
	}
	if c.EmbedSide != "left" && c.EmbedSide != "right" {
		return c, fmt.Errorf("embed_side %q: want left or right", c.EmbedSide)
	}
	if c.SubmitMode != "browser" && c.SubmitMode != "direct" {
		return c, fmt.Errorf("submit_mode %q: want browser or direct", c.SubmitMode)
	}
	if c.EmbedSide != "" && c.EmbedSide != "left" && c.EmbedSide != "right" {
		return c, fmt.Errorf("embed_side %q: want left or right", c.EmbedSide)
	}
	if c.ReadingWidth != 0 && (c.ReadingWidth < 40 || c.ReadingWidth > 200) {
		return c, fmt.Errorf("reading_width: %d is outside 40 to 200", c.ReadingWidth)
	}
	if c.ReadingMargin < 0 || c.ReadingMargin > 8 {
		return c, fmt.Errorf("reading_margin: %d is outside 0 to 8", c.ReadingMargin)
	}
	if c.ReadingSpacing != "" && c.ReadingSpacing != "compact" && c.ReadingSpacing != "normal" && c.ReadingSpacing != "relaxed" {
		return c, fmt.Errorf("reading_spacing %q: want compact, normal or relaxed", c.ReadingSpacing)
	}
	if c.ReadingHeadings != "" && c.ReadingHeadings != "plain" && c.ReadingHeadings != "bold" && c.ReadingHeadings != "bar" && c.ReadingHeadings != "underline" {
		return c, fmt.Errorf("reading_headings %q: want plain, bold, bar or underline", c.ReadingHeadings)
	}
	if c.ReadingMath != "" && c.ReadingMath != "unicode" && c.ReadingMath != "raw" {
		return c, fmt.Errorf("reading_math %q: want unicode or raw", c.ReadingMath)
	}
	if c.WheelLines < 1 || c.WheelLines > 20 {
		return c, fmt.Errorf("wheel_lines: %d is outside 1 to 20", c.WheelLines)
	}
	// A user's [lang.x] table replaces the default wholesale: re-inherit what they left out.
	// Only apply this logic to Lang entries explicitly set by the user in the TOML.
	def := Default().Lang
	for k, l := range c.Lang {
		// Skip inheritance if userLangs wasn't detected (parse error) or if this key isn't in it.
		if userLangs == nil || !userLangs[k] {
			continue
		}
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
		// Inherit testing multipliers (0 means inherit global config)
		if l.TimeMultiplier == 0 {
			l.TimeMultiplier = c.TimeMultiplier
		}
		if l.FloatEps == 0 {
			l.FloatEps = c.FloatEps
		}
		if l.MemoryMultiplier == 0 {
			l.MemoryMultiplier = 1 // default: no scaling
		}
		c.Lang[k] = l
	}
	return c, nil
}

// Get returns a top-level settable key's value as text.
func (c Config) Get(key string) (string, bool) {
	vals := map[string]any{
		"handle": c.Handle, "workspace": c.Workspace, "default_lang": c.DefaultLang, "theme": c.Theme,
		"background": c.Background, "editor": c.Editor, "split": c.Split, "embed_focus_key": c.EmbedFocusKey,
		"embed_side": c.EmbedSide, "submit_mode": c.SubmitMode, "time_multiplier": c.TimeMultiplier,
		"float_eps": c.FloatEps, "embed_ratio": c.EmbedRatio, "autotest": c.Autotest,
		"source_cf": c.SourceCF, "source_cses": c.SourceCSES,
		"reading_width": c.ReadingWidth, "reading_margin": c.ReadingMargin, "reading_spacing": c.ReadingSpacing,
		"reading_headings": c.ReadingHeadings, "reading_math": c.ReadingMath, "reading_emphasis": c.ReadingEmphasis,
		"mouse": c.Mouse, "wheel_lines": c.WheelLines, "mouse_select": c.MouseSelect,
	}
	v, ok := vals[key]
	return fmt.Sprint(v), ok
}
