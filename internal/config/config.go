// Package config loads verd's TOML config with embedded defaults.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/moneytosms/verd/internal/theme"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Handle           string   `toml:"handle"`
	Workspace        string   `toml:"workspace"`
	SnippetsDir      string   `toml:"snippets_dir"`
	DefaultLang      string   `toml:"default_lang"`
	TimeMultiplier   float64  `toml:"time_multiplier"`
	FloatEps         float64  `toml:"float_eps"`
	Autotest         bool     `toml:"autotest"`
	Resume           bool     `toml:"resume"`
	Theme            string   `toml:"theme"`
	Background       string   `toml:"background"`
	SourceCF         bool     `toml:"source_cf"`
	SourceCSES       bool     `toml:"source_cses"`
	Editor           string   `toml:"editor"`
	Split            string   `toml:"split"`
	EmbedRatio       float64  `toml:"embed_ratio"`
	EmbedFocusKey    string   `toml:"embed_focus_key"`
	EmbedSide        string   `toml:"embed_side"`
	EmbedZoom        bool     `toml:"embed_zoom"`
	SubmitMode       string   `toml:"submit_mode"`
	Browser          string   `toml:"browser"`
	Images           string   `toml:"images"`
	ReadingWidth     int      `toml:"reading_width"`
	ReadingMargin    int      `toml:"reading_margin"`
	ReadingSpacing   string   `toml:"reading_spacing"`
	ReadingHeadings  string   `toml:"reading_headings"`
	ReadingMath      string   `toml:"reading_math"`
	ReadingEmphasis  bool     `toml:"reading_emphasis"`
	ReadingSplit     string   `toml:"reading_split"`
	ReadingPalette   string   `toml:"reading_palette"`
	ReadingNightFrom int      `toml:"reading_night_from"`
	Border           string   `toml:"border"`
	Mouse            bool     `toml:"mouse"`
	WheelLines       int      `toml:"wheel_lines"`
	MouseSelect      bool     `toml:"mouse_select"`
	Header           bool     `toml:"header"`
	Footer           bool     `toml:"footer"`
	Tabs             []string `toml:"tabs"`

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
		Workspace: "~/verd", DefaultLang: "cpp", TimeMultiplier: 1.0, FloatEps: 1e-6, Autotest: true, Resume: true, Theme: "terminal", Background: "auto", SourceCF: true, Editor: "nvim", Split: "auto", EmbedRatio: 0.4, EmbedFocusKey: "ctrl+\\", EmbedSide: "right", SubmitMode: "browser", Browser: "default",
		Images: "auto", ReadingMargin: 1, ReadingSpacing: "normal", ReadingHeadings: "bar", ReadingMath: "unicode", ReadingEmphasis: true, ReadingSplit: "full", ReadingPalette: "app", ReadingNightFrom: 20, Border: "rounded",
		Mouse: true, WheelLines: 3, MouseSelect: true,
		Header: true, Footer: true, Tabs: []string{"problems", "contests", "stats", "picker", "settings"},
		Lang: map[string]Lang{
			"c":      {Ext: "c", Compile: []string{gnu(runtime.GOOS, "gcc", exec.LookPath), "-std=c11", "-O2", "-Wall", "-DLOCAL", "-I{include}", "-o", "{bin}", "{src}", "-lm"}, Run: []string{"{bin}"}, CFCompilerID: 43},
			"cpp":    {Ext: "cpp", Compile: []string{gnu(runtime.GOOS, "g++", exec.LookPath), "-std=c++20", "-O2", "-Wall", "-DLOCAL", "-I{include}", "-o", "{bin}", "{src}"}, Run: []string{"{bin}"}, CFCompilerID: 89},
			"python": {Ext: "py", Run: []string{"python3", "{src}"}, CFCompilerID: 31},
		},
	}
}

// CLIDefaultLang is default_lang for commands run outside the TUI, where "ask" cannot prompt: cpp.
func (c Config) CLIDefaultLang() string {
	if c.DefaultLang == "ask" {
		return "cpp"
	}
	return c.DefaultLang
}

// SnippetsPath is where Templates and the snippet library live: snippets_dir with a leading ~
// expanded, else <config dir>/snippets.
func (c Config) SnippetsPath() string {
	d := c.SnippetsDir
	if d == "" {
		return filepath.Join(Dir(), "snippets")
	}
	if home, err := os.UserHomeDir(); err == nil && (d == "~" || strings.HasPrefix(d, "~/")) {
		d = filepath.Join(home, strings.TrimPrefix(d, "~"))
	}
	return d
}

// TemplatesPath is where the Solution, gen and brute Templates live.
func (c Config) TemplatesPath() string { return filepath.Join(c.SnippetsPath(), "templates") }

// gnu returns the compiler for base. On macOS plain g++/gcc is Apple Clang (no <bits/stdc++.h>),
// so prefer a Homebrew GNU build (g++-15 .. g++-9) when one is on PATH.
func gnu(goos, base string, look func(string) (string, error)) string {
	if goos != "darwin" {
		return base
	}
	for v := 15; v >= 9; v-- {
		n := fmt.Sprintf("%s-%d", base, v)
		if _, err := look(n); err == nil {
			return n
		}
	}
	return base
}

// Dir returns $XDG_CONFIG_HOME/verd, falling back to ~/.config/verd on every OS.
func Dir() string { return xdg("XDG_CONFIG_HOME", ".config") }

// DataDir returns $XDG_DATA_HOME/verd, falling back to ~/.local/share/verd.
func DataDir() string { return xdg("XDG_DATA_HOME", filepath.Join(".local", "share")) }

// IncludeDir holds the headers verd ships (debug.h); local builds get -I{include} pointing here.
func IncludeDir() string { return filepath.Join(DataDir(), "include") }

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
	userLangs := userLangTables(b)
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if err := c.validate(); err != nil {
		return c, err
	}
	c.inheritLangs(userLangs)
	return c, nil
}

// userLangTables names the [lang.x] tables in b. It returns nil if b does not parse.
func userLangTables(b []byte) map[string]bool {
	var raw map[string]any
	if err := toml.Unmarshal(b, &raw); err != nil {
		return nil
	}
	langs, ok := raw["lang"].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]bool, len(langs))
	for k := range langs {
		out[k] = true
	}
	return out
}

// validate checks the decoded config and returns the first problem, in a fixed order.
func (c *Config) validate() error {
	if c.Background != "auto" && c.Background != "dark" && c.Background != "light" {
		return fmt.Errorf("background %q: want auto, dark or light", c.Background)
	}
	for name, spec := range c.Themes {
		if err := theme.Register(name, spec); err != nil {
			return fmt.Errorf("themes.%s: %w", name, err)
		}
	}
	if err := c.checkTabs(); err != nil {
		return err
	}
	if !slices.Contains([]string{"rounded", "square", "heavy", "double", "ascii", "none"}, c.Border) {
		return fmt.Errorf("border %q: want rounded, square, heavy, double, ascii or none", c.Border)
	}
	if err := c.checkEmbed(); err != nil {
		return err
	}
	if err := c.checkReading(); err != nil {
		return err
	}
	return rangeErr("wheel_lines", int64(c.WheelLines), 1, 20)
}

// checkTabs lowercases the tab names, rejects unknown ones and requires the settings tab.
func (c *Config) checkTabs() error {
	hasSettings := false
	for i, t := range c.Tabs {
		t = strings.ToLower(strings.TrimSpace(t))
		c.Tabs[i] = t
		if !slices.Contains([]string{"problems", "contests", "stats", "picker", "settings"}, t) {
			return fmt.Errorf("tabs: %q is not a tab (problems, contests, stats, picker, settings)", t)
		}
		hasSettings = hasSettings || t == "settings"
	}
	if !hasSettings {
		return fmt.Errorf("tabs: the settings tab cannot be hidden")
	}
	return nil
}

// checkEmbed checks embed_side and submit_mode.
func (c *Config) checkEmbed() error {
	if c.EmbedSide != "left" && c.EmbedSide != "right" {
		return fmt.Errorf("embed_side %q: want left or right", c.EmbedSide)
	}
	if c.SubmitMode != "browser" && c.SubmitMode != "direct" {
		return fmt.Errorf("submit_mode %q: want browser or direct", c.SubmitMode)
	}
	if c.EmbedSide != "" && c.EmbedSide != "left" && c.EmbedSide != "right" {
		return fmt.Errorf("embed_side %q: want left or right", c.EmbedSide)
	}
	return nil
}

// checkReading checks the reading_* and images keys. Empty enum values keep the default.
func (c *Config) checkReading() error {
	if c.ReadingWidth != 0 {
		if err := rangeErr("reading_width", int64(c.ReadingWidth), 40, 200); err != nil {
			return err
		}
	}
	if !allowed(c.Images, "auto", "kitty", "blocks", "off") {
		return fmt.Errorf("images %q: want auto, kitty, blocks or off", c.Images)
	}
	if err := rangeErr("reading_margin", int64(c.ReadingMargin), 0, 8); err != nil {
		return err
	}
	if !allowed(c.ReadingSpacing, "compact", "normal", "relaxed") {
		return fmt.Errorf("reading_spacing %q: want compact, normal or relaxed", c.ReadingSpacing)
	}
	if !allowed(c.ReadingHeadings, "plain", "bold", "bar", "underline") {
		return fmt.Errorf("reading_headings %q: want plain, bold, bar or underline", c.ReadingHeadings)
	}
	if !allowed(c.ReadingMath, "unicode", "latex", "raw") {
		return fmt.Errorf("reading_math %q: want unicode, latex or raw", c.ReadingMath)
	}
	if !allowed(c.ReadingSplit, "full", "cap") {
		return fmt.Errorf("reading_split %q: want full or cap", c.ReadingSplit)
	}
	if !allowed(c.ReadingPalette, ReadingPalettes...) {
		return fmt.Errorf("reading_palette %q: want %s", c.ReadingPalette, strings.Join(ReadingPalettes, ", "))
	}
	return rangeErr("reading_night_from", int64(c.ReadingNightFrom), 0, 23)
}

// inheritLangs fills what a user's [lang.x] table left out from the default of the same name.
// Only tables the user wrote in the TOML (user) are touched.
func (c *Config) inheritLangs(user map[string]bool) {
	def := Default().Lang
	for k, l := range c.Lang {
		d, ok := def[k]
		if !user[k] || !ok {
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
}

// allowed reports whether v is unset (keeps the default) or one of opts.
func allowed(v string, opts ...string) bool { return v == "" || slices.Contains(opts, v) }

// rangeErr reports key's value v when it is outside lo to hi.
func rangeErr(key string, v, lo, hi int64) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s: %d is outside %d to %d", key, v, lo, hi)
	}
	return nil
}

// Get returns a top-level settable key's value as text.
func (c Config) Get(key string) (string, bool) {
	vals := map[string]any{
		"handle": c.Handle, "workspace": c.Workspace, "snippets_dir": c.SnippetsDir, "default_lang": c.DefaultLang, "theme": c.Theme,
		"background": c.Background, "editor": c.Editor, "split": c.Split, "embed_focus_key": c.EmbedFocusKey,
		"embed_side": c.EmbedSide, "embed_zoom": c.EmbedZoom, "submit_mode": c.SubmitMode, "browser": c.Browser, "time_multiplier": c.TimeMultiplier,
		"float_eps": c.FloatEps, "embed_ratio": c.EmbedRatio, "autotest": c.Autotest, "resume": c.Resume,
		"source_cf": c.SourceCF, "source_cses": c.SourceCSES,
		"images": c.Images, "reading_width": c.ReadingWidth, "reading_margin": c.ReadingMargin, "reading_spacing": c.ReadingSpacing,
		"reading_headings": c.ReadingHeadings, "reading_math": c.ReadingMath, "reading_emphasis": c.ReadingEmphasis,
		"reading_split": c.ReadingSplit, "reading_palette": c.ReadingPalette, "reading_night_from": c.ReadingNightFrom,
		"header": c.Header, "footer": c.Footer, "tabs": strings.Join(c.Tabs, ","), "mouse": c.Mouse, "wheel_lines": c.WheelLines, "mouse_select": c.MouseSelect,
	}
	v, ok := vals[key]
	return fmt.Sprint(v), ok
}
