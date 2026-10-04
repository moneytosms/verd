// Package theme holds verd's color themes. The default inherits the terminal's ANSI palette.
package theme

import (
	"image/color"
	"sort"

	"charm.land/lipgloss/v2"
)

type Palette struct{ Accent, Dim, Good, Bad, Warn color.Color }

type Theme struct {
	Name         string
	Dark, Light  Palette
	GlamourDark  string // glamour style for statements on dark / light backgrounds
	GlamourLight string
}

// Styles are the resolved lipgloss styles for one background.
type Styles struct {
	Accent, Dim, Good, Bad, Warn lipgloss.Style
	Glamour                      string
}

func ansi(n int) color.Color { return lipgloss.ANSIColor(n) }

func hex(s string) color.Color { return lipgloss.Color(s) }

// ANSI 16 indices only: the terminal's own palette decides the actual colors.
var terminal = Palette{Accent: ansi(6), Dim: ansi(8), Good: ansi(2), Bad: ansi(1), Warn: ansi(3)}

var themes = map[string]Theme{
	"terminal": {
		Name: "terminal", Dark: terminal,
		Light:       Palette{Accent: ansi(4), Dim: ansi(8), Good: ansi(2), Bad: ansi(1), Warn: ansi(3)},
		GlamourDark: "dark", GlamourLight: "light",
	},
	"dracula": {
		Name:        "dracula",
		Dark:        Palette{Accent: hex("#bd93f9"), Dim: hex("#6272a4"), Good: hex("#50fa7b"), Bad: hex("#ff5555"), Warn: hex("#f1fa8c")},
		Light:       Palette{Accent: hex("#7c4dcc"), Dim: hex("#6272a4"), Good: hex("#1a8a3a"), Bad: hex("#c92a2a"), Warn: hex("#9a6b00")},
		GlamourDark: "dracula", GlamourLight: "light",
	},
	"tokyo-night": {
		Name:        "tokyo-night",
		Dark:        Palette{Accent: hex("#7aa2f7"), Dim: hex("#565f89"), Good: hex("#9ece6a"), Bad: hex("#f7768e"), Warn: hex("#e0af68")},
		Light:       Palette{Accent: hex("#34548a"), Dim: hex("#8990b3"), Good: hex("#485e30"), Bad: hex("#8c4351"), Warn: hex("#8f5e15")},
		GlamourDark: "tokyo-night", GlamourLight: "light",
	},
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

// Styles resolves the theme for a dark or light background.
func (t Theme) Styles(dark bool) Styles {
	p, g := t.Dark, t.GlamourDark
	if !dark {
		p, g = t.Light, t.GlamourLight
	}
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return Styles{Accent: fg(p.Accent).Bold(true), Dim: fg(p.Dim), Good: fg(p.Good), Bad: fg(p.Bad), Warn: fg(p.Warn), Glamour: g}
}
