package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Spec is a user theme from config.toml:
//
//	[themes.mine]
//	base = "nord"                # start from this theme (default: terminal)
//	glamour_dark = "dracula"     # statement style on dark / light backgrounds
//	[themes.mine.dark]
//	accent = "#7aa2f7"           # accent accent2 dim good bad warn surface bar on_accent
//	[themes.mine.light]
//	accent = "ansi4"             # "#rrggbb", "#rgb" or "ansiN" (terminal palette index 0-255)
type Spec struct {
	Base         string            `toml:"base"`
	GlamourDark  string            `toml:"glamour_dark"`
	GlamourLight string            `toml:"glamour_light"`
	Dark         map[string]string `toml:"dark"`
	Light        map[string]string `toml:"light"`
}

// Register adds (or replaces) a theme built from s.
func Register(name string, s Spec) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("theme name is empty")
	}
	base := s.Base
	if base == "" {
		base = "terminal"
	}
	b, ok := themes[base]
	if !ok {
		return fmt.Errorf("base %q is not a theme (have: %s)", base, strings.Join(Names(), ", "))
	}
	t := Theme{Name: name, Dark: b.Dark, Light: b.Light, GlamourDark: b.GlamourDark, GlamourLight: b.GlamourLight}
	if s.GlamourDark != "" {
		t.GlamourDark = s.GlamourDark
	}
	if s.GlamourLight != "" {
		t.GlamourLight = s.GlamourLight
	}
	var err error
	if t.Dark, err = override(t.Dark, s.Dark); err != nil {
		return fmt.Errorf("dark: %w", err)
	}
	if t.Light, err = override(t.Light, s.Light); err != nil {
		return fmt.Errorf("light: %w", err)
	}
	themes[name] = t
	return nil
}

// PaletteKeys are the color names a theme may set.
var PaletteKeys = []string{"accent", "accent2", "dim", "good", "bad", "warn", "surface", "bar", "on_accent"}

func override(p Palette, set map[string]string) (Palette, error) {
	fields := map[string]*color.Color{
		"accent": &p.Accent, "accent2": &p.Accent2, "dim": &p.Dim, "good": &p.Good, "bad": &p.Bad,
		"warn": &p.Warn, "surface": &p.Surface, "bar": &p.Bar, "on_accent": &p.OnAccent,
	}
	for k, v := range set {
		dst, ok := fields[k]
		if !ok {
			return p, fmt.Errorf("unknown color %q (have: %s)", k, strings.Join(PaletteKeys, ", "))
		}
		c, err := ParseColor(v)
		if err != nil {
			return p, fmt.Errorf("%s: %w", k, err)
		}
		*dst = c
	}
	return p, nil
}

// ParseColor reads "#rrggbb", "#rgb" or "ansiN".
func ParseColor(s string) (color.Color, error) {
	s = strings.TrimSpace(s)
	if n, ok := strings.CutPrefix(s, "ansi"); ok {
		i, err := strconv.Atoi(n)
		if err != nil || i < 0 || i > 255 {
			return nil, fmt.Errorf("%q: ansiN wants 0-255", s)
		}
		return ansi(i), nil
	}
	if h, ok := strings.CutPrefix(s, "#"); ok && (len(h) == 3 || len(h) == 6) {
		if _, err := strconv.ParseUint(h, 16, 32); err == nil {
			if len(h) == 3 {
				h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
			}
			return hex("#" + h), nil
		}
	}
	return nil, fmt.Errorf("%q: want #rrggbb, #rgb or ansiN", s)
}

// Describe is the palette as config.toml color strings ("#rrggbb" or "ansiN"), keyed like Spec.
func (p Palette) Describe() map[string]string {
	str := func(c color.Color) string {
		if a, ok := c.(lipgloss.ANSIColor); ok {
			return fmt.Sprintf("ansi%d", uint8(a))
		}
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	}
	return map[string]string{
		"accent": str(p.Accent), "accent2": str(p.Accent2), "dim": str(p.Dim), "good": str(p.Good), "bad": str(p.Bad),
		"warn": str(p.Warn), "surface": str(p.Surface), "bar": str(p.Bar), "on_accent": str(p.OnAccent),
	}
}
