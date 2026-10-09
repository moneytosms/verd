// Package theme holds verd's color themes. The default inherits the terminal's ANSI palette.
package theme

import (
	"image/color"
	"sort"

	"charm.land/lipgloss/v2"
)

// Palette is one theme on one background. Surface is the selected-row background, Bar the
// header and footer background, OnAccent the text color drawn on an Accent fill.
// Ac, Wa, Unk are optional verdict-specific colors, defaulting to Good, Bad, Dim respectively.
type Palette struct {
	Accent, Accent2, Dim, Good, Bad, Warn, Surface, Bar, OnAccent color.Color
	Ac, Wa, Unk                                                   color.Color
}

type Theme struct {
	Name         string
	Dark, Light  Palette
	GlamourDark  string // glamour style for statements on dark / light backgrounds
	GlamourLight string
}

// Styles are the resolved lipgloss styles for one background.
type Styles struct {
	Accent, Accent2, Dim, Good, Bad, Warn lipgloss.Style
	Ac, Wa, Unk                           lipgloss.Style // verdict colors: accepted, wrong answer, unjudged
	Selected                              lipgloss.Style // the highlighted row
	Bar                                   lipgloss.Style // header and footer background
	PillOn, PillOff                       lipgloss.Style // tabs
	Key                                   lipgloss.Style // key caps in hints
	Chip                                  lipgloss.Style // filter chips
	Glamour                               string
	fg                                    lipgloss.Style
}

// Fg is the default foreground (terminal default unless the theme sets one).
func (s Styles) Fg() lipgloss.Style { return s.fg }

func ansi(n int) color.Color { return lipgloss.ANSIColor(n) }

func hex(s string) color.Color { return lipgloss.Color(s) }

// pal builds a Palette from hex strings: accent, accent2, dim, good, bad, warn, surface, bar, onAccent.
// Ac, Wa, Unk are optional and default to nil (use Good, Bad, Dim in Styles()).
func pal(a, a2, dim, good, bad, warn, surface, bar, on string) Palette {
	return Palette{Accent: hex(a), Accent2: hex(a2), Dim: hex(dim), Good: hex(good), Bad: hex(bad), Warn: hex(warn), Surface: hex(surface), Bar: hex(bar), OnAccent: hex(on)}
}

// ANSI 16 indices only: the terminal's own palette decides the actual colors.
var terminal = Palette{Accent: ansi(6), Accent2: ansi(5), Dim: ansi(8), Good: ansi(2), Bad: ansi(1), Warn: ansi(3), Surface: ansi(8), Bar: ansi(0), OnAccent: ansi(0)}

var terminalLight = Palette{Accent: ansi(4), Accent2: ansi(5), Dim: ansi(8), Good: ansi(2), Bad: ansi(1), Warn: ansi(3), Surface: ansi(7), Bar: ansi(7), OnAccent: ansi(15)}

var themes = map[string]Theme{
	"terminal": {Name: "terminal", Dark: terminal, Light: terminalLight, GlamourDark: "dark", GlamourLight: "light"},
	"tokyo-night": {Name: "tokyo-night",
		Dark:        pal("#7aa2f7", "#bb9af7", "#565f89", "#9ece6a", "#f7768e", "#e0af68", "#2f3549", "#1f2335", "#1a1b26"),
		Light:       pal("#34548a", "#5a4a78", "#8990b3", "#485e30", "#8c4351", "#8f5e15", "#d5d6db", "#e1e2e7", "#e1e2e7"),
		GlamourDark: "tokyo-night", GlamourLight: "light"},
	"dracula": {Name: "dracula",
		Dark:        pal("#bd93f9", "#ff79c6", "#6272a4", "#50fa7b", "#ff5555", "#f1fa8c", "#44475a", "#21222c", "#282a36"),
		Light:       pal("#7c4dcc", "#c2307f", "#6272a4", "#1a8a3a", "#c92a2a", "#9a6b00", "#e3e0ef", "#efecf7", "#ffffff"),
		GlamourDark: "dracula", GlamourLight: "light"},
	"catppuccin": {Name: "catppuccin",
		Dark:        pal("#89b4fa", "#cba6f7", "#6c7086", "#a6e3a1", "#f38ba8", "#f9e2af", "#313244", "#181825", "#1e1e2e"),
		Light:       pal("#1e66f5", "#8839ef", "#8c8fa1", "#40a02b", "#d20f39", "#df8e1d", "#ccd0da", "#e6e9ef", "#eff1f5"),
		GlamourDark: "dark", GlamourLight: "light"},
	"gruvbox": {Name: "gruvbox",
		Dark:        pal("#83a598", "#d3869b", "#928374", "#b8bb26", "#fb4934", "#fabd2f", "#3c3836", "#1d2021", "#282828"),
		Light:       pal("#076678", "#8f3f71", "#928374", "#79740e", "#9d0006", "#b57614", "#ebdbb2", "#f2e5bc", "#fbf1c7"),
		GlamourDark: "dark", GlamourLight: "light"},
	"nord": {Name: "nord",
		Dark:        pal("#88c0d0", "#b48ead", "#616e88", "#a3be8c", "#bf616a", "#ebcb8b", "#3b4252", "#2e3440", "#2e3440"),
		Light:       pal("#5e81ac", "#8f6a8a", "#7b88a1", "#6a8a4f", "#bf616a", "#b5893a", "#d8dee9", "#e5e9f0", "#eceff4"),
		GlamourDark: "dark", GlamourLight: "light"},
	"one-dark": {Name: "one-dark",
		Dark:        pal("#61afef", "#c678dd", "#5c6370", "#98c379", "#e06c75", "#e5c07b", "#2c313a", "#21252b", "#282c34"),
		Light:       pal("#4078f2", "#a626a4", "#a0a1a7", "#50a14f", "#e45649", "#c18401", "#e5e5e6", "#eaeaeb", "#fafafa"),
		GlamourDark: "dark", GlamourLight: "light"},
	"solarized": {Name: "solarized",
		Dark:        pal("#268bd2", "#6c71c4", "#586e75", "#859900", "#dc322f", "#b58900", "#073642", "#04303b", "#002b36"),
		Light:       pal("#1f6fa8", "#6c71c4", "#93a1a1", "#859900", "#dc322f", "#b58900", "#eee8d5", "#e9e2cb", "#fdf6e3"),
		GlamourDark: "dark", GlamourLight: "light"},
}

// Names lists the available theme names.
func Names() []string {
	var out []string
	for n := range themes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Get returns the named theme; unknown names give the terminal theme and ok=false.
func Get(name string) (Theme, bool) {
	t, ok := themes[name]
	if !ok {
		return themes["terminal"], name == ""
	}
	return t, true
}

// Swatch is the palette of a theme's background, for previews.
func (t Theme) Swatch(dark bool) Palette {
	if dark {
		return t.Dark
	}
	return t.Light
}

// Styles resolves the theme for a dark or light background.
func (t Theme) Styles(dark bool) Styles {
	p, g := t.Dark, t.GlamourDark
	if !dark {
		p, g = t.Light, t.GlamourLight
	}
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

	// Verdict colors default to Good/Bad/Dim if not explicitly set
	acColor, waColor, unkColor := p.Good, p.Bad, p.Dim
	if p.Ac != nil {
		acColor = p.Ac
	}
	if p.Wa != nil {
		waColor = p.Wa
	}
	if p.Unk != nil {
		unkColor = p.Unk
	}

	return Styles{
		Accent: fg(p.Accent).Bold(true), Accent2: fg(p.Accent2).Bold(true), Dim: fg(p.Dim),
		Good: fg(p.Good), Bad: fg(p.Bad), Warn: fg(p.Warn),
		Ac: fg(acColor), Wa: fg(waColor), Unk: fg(unkColor),
		Selected: lipgloss.NewStyle().Background(p.Surface).Bold(true),
		Bar:      lipgloss.NewStyle().Background(p.Bar),
		PillOn:   lipgloss.NewStyle().Background(p.Accent).Foreground(p.OnAccent).Bold(true),
		PillOff:  fg(p.Dim),
		Key:      fg(p.Accent2).Bold(true),
		Chip:     lipgloss.NewStyle().Background(p.Surface).Foreground(p.Accent2),
		Glamour:  g,
		fg:       lipgloss.NewStyle().Bold(true),
	}
}
